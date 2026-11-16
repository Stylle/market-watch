package main

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"
)

const version = "0.1.2"

//go:embed all:web
var assets embed.FS

type bucket struct {
	start time.Time
	count int
}
type app struct {
	service *marketService
	mu      sync.Mutex
	limits  map[string]bucket
}

func newApp() *app { return &app{service: newService(), limits: map[string]bucket{}} }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *app) allowed(r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if len(a.limits) > 4096 {
		for k, b := range a.limits {
			if now.Sub(b.start) >= time.Minute {
				delete(a.limits, k)
			}
		}
		if len(a.limits) > 4096 {
			return false
		}
	}
	b := a.limits[host]
	if now.Sub(b.start) >= time.Minute {
		b = bucket{start: now}
	}
	b.count++
	a.limits[host] = b
	return b.count <= 180
}
func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	if r.URL.Path == "/healthz" && r.Method == "GET" {
		respond(w, 200, map[string]any{"ok": true, "version": version})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") && !a.allowed(r) {
		respond(w, 429, map[string]string{"error": "请求过于频繁，请稍后再试"})
		return
	}
	switch {
	case r.URL.Path == "/api/catalog" && r.Method == "GET":
		respond(w, 200, builtins)
	case r.URL.Path == "/api/quotes" && r.Method == "POST":
		var in struct {
			IDs []string `json:"ids"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil || d.Decode(&struct{}{}) != io.EOF || len(in.IDs) == 0 || len(in.IDs) > 50 {
			respond(w, 400, map[string]string{"error": "标的列表无效（最多50个）"})
			return
		}
		seen := map[string]bool{}
		for _, id := range in.IDs {
			if !validID(id) || seen[id] {
				respond(w, 400, map[string]string{"error": "标的标识无效或重复"})
				return
			}
			seen[id] = true
		}
		out := make([]Quote, len(in.IDs))
		var wg sync.WaitGroup
		for k, id := range in.IDs {
			wg.Add(1)
			go func(k int, id string) { defer wg.Done(); q, _ := a.service.quote(r.Context(), id); out[k] = q }(k, id)
		}
		wg.Wait()
		respond(w, 200, map[string]any{"quotes": out, "fetched": time.Now().Unix()})
	case r.URL.Path == "/api/chart" && r.Method == "GET":
		id, p := r.URL.Query().Get("id"), r.URL.Query().Get("range")
		if !validID(id) || ranges[p] == "" {
			respond(w, 400, map[string]string{"error": "标的或时间区间无效"})
			return
		}
		c, e := a.service.chart(r.Context(), id, p)
		if e != nil {
			respond(w, 503, c)
			return
		}
		respond(w, 200, c)
	case r.URL.Path == "/api/search" && r.Method == "GET":
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len([]rune(q)) < 1 || len(q) > 100 {
			respond(w, 400, map[string]string{"error": "请输入1～100字节的名称或代码"})
			return
		}
		respond(w, 200, map[string]any{"items": a.service.search(r.Context(), q)})
	default:
		if r.Method != "GET" && r.Method != "HEAD" {
			respond(w, 405, map[string]string{"error": "请求方式不支持"})
			return
		}
		clean := path.Clean(r.URL.Path)
		if strings.HasPrefix(clean, "/api") || strings.Contains(clean, "..") || strings.Contains(clean, "/.") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(clean, "/")
		if name == "" {
			name = "index.html"
		}
		sub, _ := fs.Sub(assets, "web")
		stat, e := fs.Stat(sub, name)
		if e != nil || stat.IsDir() {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
		}
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
	}
}
func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: newApp(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 8192}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Print("Market Watch started")
	if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal("server startup failed")
	}
}
