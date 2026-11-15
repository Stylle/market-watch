package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIdentifiers(t *testing.T) {
	for _, s := range []string{"y:AAPL", "y:0700.HK", "y:^HSI", "y:GC=F", "c:BTC-USD", "g:XAU"} {
		if !validID(s) {
			t.Fatal(s)
		}
	}
	for _, s := range []string{"y:../etc", "y:https://localhost", "c:BTC-USDT", "g:BTC", "y:", "c:BTC-USD?x=1"} {
		if validID(s) {
			t.Fatal(s)
		}
	}
}
func TestPreviousTradingClose(t *testing.T) {
	var env yahooEnvelope
	body := `{"chart":{"result":[{"meta":{"regularMarketTime":1794603600,"exchangeTimezoneName":"America/New_York","regularMarketPrice":248.5},"timestamp":[1794407400,1794493800,1794580200],"indicators":{"quote":[{"close":[240,246.32,248.5],"volume":[20,30,40]}]}}]}}`
	if json.Unmarshal([]byte(body), &env) != nil {
		t.Fatal("fixture")
	}
	y := env.Chart.Result[0]
	p := previousClose(y)
	if p == nil || *p != 246.32 {
		t.Fatalf("previous trading day: %v", p)
	}
	q := Quote{Price: 248.5, Previous: p}
	changes(&q)
	if q.Change == nil || *q.Change < 2.179 || *q.Change > 2.181 {
		t.Fatal(q)
	}
	y.Meta.At = 1794407400
	if previousClose(y) != nil {
		t.Fatal("must not invent baseline")
	}
}
func TestMarketSession(t *testing.T) {
	var y yahooChart
	y.Meta.Periods.Regular.Start = 100
	y.Meta.Periods.Regular.End = 200
	y.Meta.Periods.Pre.Start = 50
	y.Meta.Periods.Pre.End = 100
	y.Meta.Periods.Post.Start = 200
	y.Meta.Periods.Post.End = 250
	for _, v := range []struct {
		n     int64
		state string
	}{{75, "盘前"}, {150, "交易中"}, {225, "盘后"}, {300, "休市"}} {
		if marketState(y, v.n) != v.state {
			t.Fatal(v)
		}
	}
}
func TestSingleFlightCancellationAndStale(t *testing.T) {
	s := newService()
	var calls atomic.Int32
	load := func(context.Context) (any, error) { calls.Add(1); time.Sleep(30 * time.Millisecond); return "ok", nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _ = s.cached(ctx, "key", time.Hour, load)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, stale, e := s.cached(context.Background(), "key", time.Hour, load)
			if e != nil || stale || v != "ok" {
				t.Error("shared refresh", e)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
	s.mu.Lock()
	s.entries["key"].expires = time.Now().Add(-time.Second)
	s.mu.Unlock()
	v, stale, e := s.cached(context.Background(), "key", time.Hour, func(context.Context) (any, error) { return nil, errors.New("down") })
	if e != nil || !stale || v != "ok" {
		t.Fatal("stale recovery", v, stale, e)
	}
}
func TestNoKeyGoldHistory(t *testing.T) {
	s := newService()
	s.goldKey = ""
	s.getJSON = func(context.Context, string, any) error {
		t.Fatal("must not call paid history without key")
		return nil
	}
	c, e := s.chart(context.Background(), "g:XAU", "1mo")
	if e == nil || !strings.Contains(c.Message, "GOLD_API_KEY") || len(c.Points) != 0 {
		t.Fatal(c, e)
	}
}
func TestCoinbaseSortedAndBounded(t *testing.T) {
	s := newService()
	s.getJSON = func(ctx context.Context, u string, out any) error {
		now := time.Now().Unix()
		data := [][]float64{{float64(now - 100), 1, 3, 2, 2.5, 4}, {float64(now - 200), 1, 2, 1, 1.5, 3}, {float64(now + 1000), 1, 3, 2, 3, 1}}
		b, _ := json.Marshal(data)
		return json.Unmarshal(b, out)
	}
	c, e := s.coinbaseChart(context.Background(), "c:BTC-USD", "1d")
	if e != nil || len(c.Points) != 2 || c.Points[0].Time >= c.Points[1].Time {
		t.Fatal(c, e)
	}
	_, e = s.coinbaseChart(context.Background(), "c:BTC-USD", "max")
	if e == nil {
		t.Fatal("cannot claim incomplete all-time data")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestFixedHostsAnd429Cooldown(t *testing.T) {
	s := newService()
	calls := 0
	s.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"60"}}, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})
	var out any
	if s.request(context.Background(), "http://127.0.0.1/", &out) == nil || calls != 0 {
		t.Fatal("SSRF boundary")
	}
	for i := 0; i < 2; i++ {
		if s.request(context.Background(), "https://api.gold-api.com/price/XAU", &out) == nil {
			t.Fatal("429 ignored")
		}
	}
	if calls != 1 {
		t.Fatal("host cooldown missing", calls)
	}
}
func TestAPIRejectsInvalidAndStaticLeaks(t *testing.T) {
	a := newApp()
	for _, body := range []string{`{"ids":["y:../x"]}`, `{"ids":["y:AAPL","y:AAPL"]}`, `{"ids":[]}`, `{"ids":["y:AAPL"],"key":"no"}`, `{"ids":["y:AAPL"]}{}`} {
		r := httptest.NewRequest("POST", "/api/quotes", strings.NewReader(body))
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(body, w.Code)
		}
	}
	for _, path := range []string{"/.env", "/.git/config", "/market.go", "/api/anything"} {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal(path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatal("health")
	}
}
func TestDailyQuoteBaselineAndNoCryptoMislabel(t *testing.T) {
	s := newService()
	crypto := false
	s.getJSON = func(ctx context.Context, u string, out any) error {
		if !strings.Contains(u, "interval=1d") {
			t.Error("daily quote must be independent of chart range")
		}
		kind := "EQUITY"
		if crypto {
			kind = "CRYPTOCURRENCY"
		}
		body := `{"chart":{"result":[{"meta":{"instrumentType":"` + kind + `","regularMarketTime":1794603600,"regularMarketPrice":248.5,"currency":"USD","exchangeTimezoneName":"America/New_York"},"timestamp":[1794407400,1794493800,1794580200],"indicators":{"quote":[{"close":[240,246.32,248.5],"open":[239,245,246.3],"high":[241,247,249.2],"low":[238,244,245.1],"volume":[20,30,40]}]}}]}}`
		return json.Unmarshal([]byte(body), out)
	}
	v, e := s.loadQuote(context.Background(), "y:AAPL")
	if e != nil {
		t.Fatal(e)
	}
	q := v.(Quote)
	if q.Previous == nil || *q.Previous != 246.32 || q.Open == nil || *q.Open != 246.3 {
		t.Fatal("wrong daily stats", q)
	}
	crypto = true
	if _, e = s.loadQuote(context.Background(), "y:BTC-USD"); e == nil {
		t.Fatal("Yahoo daily crypto change cannot be called 24h")
	}
}
