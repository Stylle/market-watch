package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Instrument struct {
	ID       string `json:"id"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Market   string `json:"market"`
	Currency string `json:"currency"`
	Kind     string `json:"kind"`
	Aliases  string `json:"aliases,omitempty"`
}
type Point struct {
	Time   int64    `json:"time"`
	Close  float64  `json:"close"`
	Volume *float64 `json:"volume,omitempty"`
}
type Quote struct {
	ID        string   `json:"id"`
	Symbol    string   `json:"symbol"`
	Name      string   `json:"name"`
	Market    string   `json:"market"`
	Currency  string   `json:"currency"`
	Kind      string   `json:"kind"`
	Price     float64  `json:"price"`
	Previous  *float64 `json:"previous"`
	Change    *float64 `json:"change"`
	Percent   *float64 `json:"percent"`
	Open      *float64 `json:"open"`
	High      *float64 `json:"high"`
	Low       *float64 `json:"low"`
	Volume    *float64 `json:"volume"`
	MarketCap *float64 `json:"marketCap"`
	AsOf      int64    `json:"asOf"`
	Timezone  string   `json:"timezone"`
	State     string   `json:"state"`
	Basis     string   `json:"basis"`
	Unit      string   `json:"unit"`
	Source    string   `json:"source"`
	SourceURL string   `json:"sourceUrl"`
	Fetched   int64    `json:"fetched"`
	Stale     bool     `json:"stale"`
	Spark     []Point  `json:"spark"`
	Error     string   `json:"error,omitempty"`
}
type Chart struct {
	ID       string  `json:"id"`
	Range    string  `json:"range"`
	Points   []Point `json:"points"`
	Source   string  `json:"source"`
	Timezone string  `json:"timezone"`
	Fetched  int64   `json:"fetched"`
	Stale    bool    `json:"stale"`
	Message  string  `json:"message,omitempty"`
}

var builtins = []Instrument{
	{"y:AAPL", "AAPL", "Apple Inc.", "NASDAQ", "USD", "stock", "苹果 pingguo apple"},
	{"y:NVDA", "NVDA", "NVIDIA Corporation", "NASDAQ", "USD", "stock", "英伟达 yingweida"},
	{"y:0700.HK", "0700", "腾讯控股", "HKEX", "HKD", "stock", "腾讯 騰訊 tencent 700"},
	{"y:7203.T", "7203", "丰田汽车", "TSE", "JPY", "stock", "丰田 豐田 toyota"},
	{"c:BTC-USD", "BTC/USD", "Bitcoin", "Coinbase", "USD", "crypto", "比特币 比特幣 btc bitcoin"},
	{"c:ETH-USD", "ETH/USD", "Ethereum", "Coinbase", "USD", "crypto", "以太坊 eth ethereum"},
	{"g:XAU", "XAU/USD", "现货黄金", "现货", "USD", "metal", "黄金 黃金 gold xau"},
	{"y:^HSI", "HSI", "恒生指数", "HKEX", "HKD", "index", "恒生 恆生 hsi"},
	{"y:MSFT", "MSFT", "Microsoft Corporation", "NASDAQ", "USD", "stock", "微软 微軟 microsoft"},
	{"y:TSLA", "TSLA", "Tesla, Inc.", "NASDAQ", "USD", "stock", "特斯拉 tesla"},
	{"y:9988.HK", "9988", "阿里巴巴", "HKEX", "HKD", "stock", "阿里 alibaba"},
	{"y:1810.HK", "1810", "小米集团", "HKEX", "HKD", "stock", "小米 xiaomi"},
	{"y:SPY", "SPY", "SPDR S&P 500 ETF", "NYSE Arca", "USD", "etf", "标普 标普500 标准普尔 sp500"},
	{"y:^GSPC", "SPX", "标普500指数", "S&P DJI", "USD", "index", "标普500 sp500 spx"},
	{"y:^IXIC", "IXIC", "纳斯达克综合指数", "NASDAQ", "USD", "index", "纳指 纳斯达克"},
	{"y:9984.T", "9984", "SoftBank Group", "TSE", "JPY", "stock", "软银 軟銀"},
	{"y:2330.TW", "2330", "台积电", "TWSE", "TWD", "stock", "台积电 台積電 tsmc"},
	{"y:005930.KS", "005930", "Samsung Electronics", "KRX", "KRW", "stock", "三星 samsung"},
	{"y:ASML.AS", "ASML", "ASML Holding", "Euronext", "EUR", "stock", "阿斯麦 阿斯麥"},
	{"y:SAP.DE", "SAP", "SAP SE", "XETRA", "EUR", "stock", "德国 德國"},
	{"y:NESN.SW", "NESN", "Nestlé", "SIX", "CHF", "stock", "雀巢"},
	{"y:VOD.L", "VOD", "Vodafone Group", "LSE", "GBp", "stock", "沃达丰 英国 英國"},
	{"y:GC=F", "GC=F", "黄金期货（连续合约）", "COMEX", "USD", "future", "黄金期货 gold futures"},
	{"g:XAG", "XAG/USD", "现货白银", "现货", "USD", "metal", "白银 白銀 silver"},
	{"c:SOL-USD", "SOL/USD", "Solana", "Coinbase", "USD", "crypto", "sol solana 索拉纳"},
	{"c:XRP-USD", "XRP/USD", "XRP", "Coinbase", "USD", "crypto", "瑞波币 xrp"},
}
var idRE = regexp.MustCompile(`^[A-Za-z0-9^][A-Za-z0-9.^=\-]{0,23}$`)
var ranges = map[string]string{"1d": "5m", "5d": "30m", "1mo": "1d", "3mo": "1d", "1y": "1d", "max": "1mo"}

func validID(id string) bool {
	if len(id) < 3 || len(id) > 28 || id[1] != ':' || !idRE.MatchString(id[2:]) {
		return false
	}
	switch id[0] {
	case 'y':
		return true
	case 'c':
		return regexp.MustCompile(`^[A-Z0-9]{2,12}-USD$`).MatchString(id[2:])
	case 'g':
		return id == "g:XAU" || id == "g:XAG"
	}
	return false
}
func instrument(id string) Instrument {
	for _, v := range builtins {
		if v.ID == id {
			return v
		}
	}
	return Instrument{ID: id, Symbol: id[2:], Name: id[2:], Kind: "stock"}
}
func number(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
func positive(v float64) *float64 {
	if v <= 0 {
		return nil
	}
	return number(v)
}
func changes(q *Quote) {
	if q.Previous != nil && *q.Previous > 0 {
		q.Change = number(q.Price - *q.Previous)
		q.Percent = number((q.Price / *q.Previous - 1) * 100)
	}
}

type entry struct {
	value   any
	expires time.Time
	retry   time.Time
	used    time.Time
	busy    chan struct{}
	err     error
}
type marketService struct {
	mu          sync.Mutex
	entries     map[string]*entry
	client      *http.Client
	slots       chan struct{}
	lastRequest map[string]time.Time
	cooldown    map[string]time.Time
	goldMu      sync.Mutex
	goldCalls   []time.Time
	goldKey     string
	getJSON     func(context.Context, string, any) error
}

func newService() *marketService {
	s := &marketService{entries: map[string]*entry{}, client: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, slots: make(chan struct{}, 4), lastRequest: map[string]time.Time{}, cooldown: map[string]time.Time{}, goldKey: os.Getenv("GOLD_API_KEY")}
	s.getJSON = s.request
	return s
}
func (s *marketService) request(ctx context.Context, raw string, out any) error {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" {
		return errors.New("invalid upstream")
	}
	switch u.Host {
	case "query1.finance.yahoo.com", "query2.finance.yahoo.com", "api.exchange.coinbase.com", "api.gold-api.com":
	default:
		return errors.New("invalid upstream")
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	s.mu.Lock()
	if time.Now().Before(s.cooldown[u.Host]) {
		s.mu.Unlock()
		return errors.New("行情来源限流，请稍后重试")
	}
	wait := time.Until(s.lastRequest[u.Host].Add(120 * time.Millisecond))
	s.lastRequest[u.Host] = time.Now().Add(max(wait, 0))
	s.mu.Unlock()
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	req, e := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if e != nil {
		return e
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")
	if u.Host == "api.gold-api.com" && u.Path == "/history" {
		req.Header.Set("x-api-key", s.goldKey)
	}
	r, e := s.client.Do(req)
	if e != nil {
		return errors.New("行情线路连接失败")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		if r.StatusCode == 429 || r.StatusCode == 503 {
			delay := time.Minute
			if seconds, e := strconv.Atoi(r.Header.Get("Retry-After")); e == nil && seconds > 0 {
				delay = min(time.Duration(seconds)*time.Second, 15*time.Minute)
			}
			s.mu.Lock()
			s.cooldown[u.Host] = time.Now().Add(delay)
			s.mu.Unlock()
		}
		return fmt.Errorf("行情服务暂不可用（HTTP %d）", r.StatusCode)
	}
	body, e := io.ReadAll(io.LimitReader(r.Body, 3*1024*1024+1))
	if e != nil || len(body) > 3*1024*1024 {
		return errors.New("行情响应无效")
	}
	if json.Unmarshal(body, out) != nil {
		return errors.New("行情响应格式无效")
	}
	return nil
}
func (s *marketService) cached(ctx context.Context, key string, ttl time.Duration, load func(context.Context) (any, error)) (any, bool, error) {
	s.mu.Lock()
	e := s.entries[key]
	now := time.Now()
	if e == nil {
		if len(s.entries) >= 512 {
			var oldest string
			var at time.Time
			for k, v := range s.entries {
				if v.busy == nil && (oldest == "" || v.used.Before(at)) {
					oldest = k
					at = v.used
				}
			}
			if oldest == "" {
				s.mu.Unlock()
				return nil, false, errors.New("请求繁忙，请稍后重试")
			}
			delete(s.entries, oldest)
		}
		e = &entry{}
		s.entries[key] = e
	}
	e.used = now
	if e.value != nil && now.Before(e.expires) {
		v := e.value
		s.mu.Unlock()
		return v, false, nil
	}
	if e.busy != nil {
		done := e.busy
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-done:
			return s.cached(ctx, key, ttl, load)
		}
	}
	if now.Before(e.retry) {
		v, err := e.value, e.err
		s.mu.Unlock()
		if v != nil {
			return v, true, nil
		}
		return nil, false, err
	}
	e.busy = make(chan struct{})
	done := e.busy
	s.mu.Unlock()
	// Shared refresh outlives individual browser cancellation, but has its own timeout.
	go func() {
		up, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		v, err := load(up)
		s.mu.Lock()
		if err == nil {
			e.value = v
			e.expires = time.Now().Add(ttl)
			e.err = nil
			e.retry = time.Time{}
		} else {
			e.err = err
			e.retry = time.Now().Add(time.Minute)
		}
		close(done)
		e.busy = nil
		s.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case <-done:
		return s.cached(ctx, key, ttl, load)
	}
}

type yahooEnvelope struct {
	Chart struct {
		Result []yahooChart `json:"result"`
		Error  any          `json:"error"`
	} `json:"chart"`
}
type yahooChart struct {
	Meta struct {
		Symbol           string                                                  `json:"symbol"`
		Currency         string                                                  `json:"currency"`
		FullExchangeName string                                                  `json:"fullExchangeName"`
		ExchangeName     string                                                  `json:"exchangeName"`
		InstrumentType   string                                                  `json:"instrumentType"`
		LongName         string                                                  `json:"longName"`
		ShortName        string                                                  `json:"shortName"`
		Price            float64                                                 `json:"regularMarketPrice"`
		At               int64                                                   `json:"regularMarketTime"`
		Timezone         string                                                  `json:"exchangeTimezoneName"`
		MarketCap        *float64                                                `json:"marketCap"`
		Periods          struct{ Pre, Regular, Post struct{ Start, End int64 } } `json:"currentTradingPeriod"`
	} `json:"meta"`
	Timestamp  []int64 `json:"timestamp"`
	Indicators struct {
		Quote []struct{ Open, High, Low, Close, Volume []*float64 } `json:"quote"`
	} `json:"indicators"`
}

func (s *marketService) yahoo(ctx context.Context, symbol, period string, override ...string) (yahooChart, error) {
	interval := ranges[period]
	if len(override) > 0 {
		interval = override[0]
	}
	var last error
	for _, host := range []string{"query1.finance.yahoo.com", "query2.finance.yahoo.com"} {
		var env yahooEnvelope
		e := s.getJSON(ctx, "https://"+host+"/v8/finance/chart/"+url.PathEscape(symbol)+"?range="+period+"&interval="+interval+"&includePrePost=false", &env)
		if e == nil && len(env.Chart.Result) == 1 && env.Chart.Error == nil {
			return env.Chart.Result[0], nil
		}
		if e == nil {
			e = errors.New("该标的暂无可用行情")
		}
		last = e
	}
	return yahooChart{}, last
}
func points(y yahooChart) []Point {
	out := []Point{}
	if len(y.Indicators.Quote) == 0 {
		return out
	}
	q := y.Indicators.Quote[0]
	for i, t := range y.Timestamp {
		if i >= len(q.Close) || q.Close[i] == nil || *q.Close[i] <= 0 || number(*q.Close[i]) == nil {
			continue
		}
		p := Point{Time: t, Close: *q.Close[i]}
		if i < len(q.Volume) && q.Volume[i] != nil && *q.Volume[i] >= 0 {
			p.Volume = number(*q.Volume[i])
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	return out
}
func marketState(y yahooChart, now int64) string {
	p := y.Meta.Periods
	if p.Regular.Start <= now && now < p.Regular.End {
		if loc, e := time.LoadLocation(y.Meta.Timezone); e == nil {
			t := time.Unix(now, 0).In(loc)
			minute := t.Hour()*60 + t.Minute()
			if y.Meta.Timezone == "Asia/Hong_Kong" && minute >= 720 && minute < 780 || y.Meta.Timezone == "Asia/Tokyo" && minute >= 690 && minute < 750 {
				return "午间休市"
			}
		}
		return "交易中"
	}
	if p.Pre.Start <= now && now < p.Pre.End {
		return "盘前"
	}
	if p.Post.Start <= now && now < p.Post.End {
		return "盘后"
	}
	return "休市"
}
func previousClose(y yahooChart) *float64 {
	p := points(y)
	loc, e := time.LoadLocation(y.Meta.Timezone)
	if e != nil {
		loc = time.UTC
	}
	date := time.Unix(y.Meta.At, 0).In(loc).Format("2006-01-02")
	for i := len(p) - 1; i >= 0; i-- {
		if time.Unix(p[i].Time, 0).In(loc).Format("2006-01-02") < date {
			return positive(p[i].Close)
		}
	}
	return nil
}
func (s *marketService) loadQuote(ctx context.Context, id string) (any, error) {
	i := instrument(id)
	q := Quote{ID: id, Symbol: i.Symbol, Name: i.Name, Market: i.Market, Currency: i.Currency, Kind: i.Kind, Timezone: "UTC", Basis: "较昨收", Fetched: time.Now().Unix(), Spark: []Point{}}
	switch id[0] {
	case 'y':
		y, e := s.yahoo(ctx, id[2:], "5d", "1d")
		if e != nil {
			return nil, e
		} // Quotes use daily bars, independent of selected chart window.
		if y.Meta.Price <= 0 || number(y.Meta.Price) == nil || y.Meta.At <= 0 {
			return nil, errors.New("该标的暂无有效报价")
		}
		daily := yahooEnvelope{}
		daily.Chart.Result = []yahooChart{y}
		q.Price = y.Meta.Price
		q.AsOf = y.Meta.At
		q.Currency = y.Meta.Currency
		q.Timezone = y.Meta.Timezone
		if y.Meta.LongName != "" && i.Name == id[2:] {
			q.Name = y.Meta.LongName
		}
		if q.Market == "" {
			q.Market = y.Meta.FullExchangeName
		}
		if y.Meta.InstrumentType == "INDEX" {
			q.Kind = "index"
		}
		if y.Meta.InstrumentType == "ETF" {
			q.Kind = "etf"
		}
		if y.Meta.InstrumentType == "FUTURE" {
			q.Kind = "future"
		}
		if y.Meta.InstrumentType == "CRYPTOCURRENCY" {
			return nil, errors.New("请添加Coinbase USD交易对，以保持24小时涨跌口径一致")
		}
		q.State = marketState(y, time.Now().Unix())
		q.Spark = points(y)
		if len(daily.Chart.Result) == 1 {
			d := daily.Chart.Result[0]
			q.Previous = previousClose(d)
			if len(d.Indicators.Quote) > 0 && len(d.Timestamp) > 0 {
				k := len(d.Timestamp) - 1
				v := d.Indicators.Quote[0]
				if k < len(v.Open) {
					q.Open = v.Open[k]
				}
				if k < len(v.High) {
					q.High = v.High[k]
				}
				if k < len(v.Low) {
					q.Low = v.Low[k]
				}
				if k < len(v.Volume) {
					q.Volume = v.Volume[k]
				}
			}
		}
		q.MarketCap = y.Meta.MarketCap
		q.Source = "Yahoo Finance"
		q.SourceURL = "https://finance.yahoo.com/quote/" + url.PathEscape(id[2:])
		changes(&q)
	case 'c':
		var v struct{ Open, High, Low, Last, Volume string }
		if e := s.getJSON(ctx, "https://api.exchange.coinbase.com/products/"+id[2:]+"/stats", &v); e != nil {
			return nil, e
		}
		f := func(v string) *float64 { n, _ := strconv.ParseFloat(v, 64); return positive(n) }
		p := f(v.Last)
		if p == nil {
			return nil, errors.New("交易对暂无有效报价")
		}
		q.Price = *p
		q.Previous = f(v.Open)
		q.Open = q.Previous
		q.High = f(v.High)
		q.Low = f(v.Low)
		q.Volume = f(v.Volume)
		q.Kind = "crypto"
		q.Currency = "USD"
		q.Market = "Coinbase"
		q.Symbol = strings.ReplaceAll(id[2:], "-", "/")
		q.State = "全天交易"
		q.Basis = "24小时"
		q.AsOf = time.Now().Unix()
		q.Source = "Coinbase Exchange"
		q.SourceURL = "https://www.coinbase.com/advanced-trade/spot/" + id[2:]
		changes(&q)
		// Historical candles are optional: a chart failure never hides a valid quote.
		if c, e := s.coinbaseChart(ctx, id, "1d"); e == nil {
			q.Spark = c.Points
		}
	case 'g':
		var v struct {
			Price     float64
			UpdatedAt string
			Currency  string
		}
		if e := s.getJSON(ctx, "https://api.gold-api.com/price/"+id[2:], &v); e != nil {
			return nil, e
		}
		if positive(v.Price) == nil {
			return nil, errors.New("现货报价无效")
		}
		at, e := time.Parse(time.RFC3339, v.UpdatedAt)
		if e != nil {
			return nil, errors.New("现货报价时间无效")
		}
		q.Price = v.Price
		q.AsOf = at.Unix()
		q.Currency = "USD"
		q.Kind = "metal"
		q.Unit = "美元 / 金衡盎司"
		q.State = "现货报价"
		q.Basis = "不提供日涨跌"
		q.Source = "Gold API"
		q.SourceURL = "https://gold-api.com/"
	}
	return q, nil
}
func (s *marketService) quote(ctx context.Context, id string) (Quote, error) {
	v, stale, e := s.cached(ctx, "q:"+id, 60*time.Second, func(ctx context.Context) (any, error) { return s.loadQuote(ctx, id) })
	if e != nil {
		return Quote{ID: id, Error: e.Error()}, e
	}
	q := v.(Quote)
	q.Stale = stale
	return q, nil
}
func (s *marketService) coinbaseChart(ctx context.Context, id, period string) (Chart, error) {
	now := time.Now()
	days := map[string]int{"1d": 1, "5d": 5, "1mo": 31, "3mo": 93, "1y": 365, "max": 365}[period]
	gran := 86400
	if period == "1d" {
		gran = 900
	}
	if period == "5d" {
		gran = 3600
	}
	if period == "1y" || period == "max" {
		return Chart{}, errors.New("此来源暂支持最长3个月历史")
	}
	start := now.Add(-time.Duration(days) * 24 * time.Hour)
	u := "https://api.exchange.coinbase.com/products/" + id[2:] + "/candles?granularity=" + strconv.Itoa(gran) + "&start=" + url.QueryEscape(start.UTC().Format(time.RFC3339)) + "&end=" + url.QueryEscape(now.UTC().Format(time.RFC3339))
	var v [][]float64
	if e := s.getJSON(ctx, u, &v); e != nil {
		return Chart{}, e
	}
	c := Chart{ID: id, Range: period, Source: "Coinbase Exchange", Timezone: "UTC", Fetched: now.Unix(), Points: []Point{}}
	for _, p := range v {
		if len(p) >= 6 && p[0] >= float64(start.Unix()) && p[0] <= float64(now.Unix()) && positive(p[4]) != nil {
			c.Points = append(c.Points, Point{Time: int64(p[0]), Close: p[4], Volume: number(p[5])})
		}
	}
	sort.Slice(c.Points, func(i, j int) bool { return c.Points[i].Time < c.Points[j].Time })
	if len(c.Points) == 0 {
		return c, errors.New("此区间暂无历史数据")
	}
	return c, nil
}
func (s *marketService) chart(ctx context.Context, id, period string) (Chart, error) {
	v, stale, e := s.cached(ctx, "c:"+id+":"+period, 5*time.Minute, func(ctx context.Context) (any, error) {
		switch id[0] {
		case 'c':
			return s.coinbaseChart(ctx, id, period)
		case 'g':
			if s.goldKey == "" {
				return nil, errors.New("现货历史图表需部署者配置 GOLD_API_KEY；当前报价不受影响")
			}
			if period == "1d" || period == "max" {
				return nil, errors.New("现货历史支持1周至1年日线")
			}
			s.goldMu.Lock()
			now := time.Now()
			fresh := s.goldCalls[:0]
			for _, t := range s.goldCalls {
				if now.Sub(t) < time.Hour {
					fresh = append(fresh, t)
				}
			}
			s.goldCalls = fresh
			if len(fresh) >= 9 {
				s.goldMu.Unlock()
				return nil, errors.New("现货历史接口额度已用完，请稍后重试")
			}
			s.goldCalls = append(s.goldCalls, now)
			s.goldMu.Unlock()
			days := map[string]int{"5d": 5, "1mo": 31, "3mo": 93, "1y": 365}[period]
			var rows []struct {
				Day   string  `json:"day"`
				Price float64 `json:"avg_price"`
			}
			u := fmt.Sprintf("https://api.gold-api.com/history?symbol=%s&startTimestamp=%d&endTimestamp=%d&groupBy=day&aggregation=avg&orderBy=asc", id[2:], now.Add(-time.Duration(days)*24*time.Hour).Unix(), now.Unix())
			if e := s.getJSON(ctx, u, &rows); e != nil {
				return nil, e
			}
			c := Chart{ID: id, Range: period, Source: "Gold API · 日均价", Timezone: "UTC", Fetched: now.Unix(), Points: []Point{}}
			for _, p := range rows {
				d, e := time.Parse("2006-01-02", p.Day)
				if e == nil && positive(p.Price) != nil {
					c.Points = append(c.Points, Point{Time: d.Unix(), Close: p.Price})
				}
			}
			if len(c.Points) == 0 {
				return nil, errors.New("此区间暂无现货历史")
			}
			return c, nil
		default:
			y, e := s.yahoo(ctx, id[2:], period)
			if e != nil {
				return nil, e
			}
			p := points(y)
			if len(p) == 0 {
				return nil, errors.New("此区间暂无历史数据")
			}
			return Chart{ID: id, Range: period, Points: p, Source: "Yahoo Finance", Timezone: y.Meta.Timezone, Fetched: time.Now().Unix()}, nil
		}
	})
	if e != nil {
		return Chart{ID: id, Range: period, Points: []Point{}, Message: e.Error()}, e
	}
	c := v.(Chart)
	c.Stale = stale
	return c, nil
}
func (s *marketService) cryptoSearch(ctx context.Context, query string) []Instrument {
	v, _, err := s.cached(ctx, "coinbase:catalog", time.Hour, func(ctx context.Context) (any, error) {
		var products []struct {
			ID       string `json:"id"`
			Base     string `json:"base_currency"`
			Quote    string `json:"quote_currency"`
			Status   string `json:"status"`
			Disabled bool   `json:"trading_disabled"`
		}
		if err := s.getJSON(ctx, "https://api.exchange.coinbase.com/products", &products); err != nil {
			return nil, err
		}
		var currencies []struct{ ID, Name string }
		_ = s.getJSON(ctx, "https://api.exchange.coinbase.com/currencies", &currencies)
		names := map[string]string{}
		for _, c := range currencies {
			names[c.ID] = c.Name
		}
		out := []Instrument{}
		for _, p := range products {
			if p.Quote != "USD" || p.Disabled || p.Status != "online" || !validID("c:"+p.ID) {
				continue
			}
			name := names[p.Base]
			if name == "" {
				name = p.Base
			}
			out = append(out, Instrument{ID: "c:" + p.ID, Symbol: p.Base + "/USD", Name: name, Market: "Coinbase", Currency: "USD", Kind: "crypto"})
		}
		return out, nil
	})
	if err != nil {
		return nil
	}
	out := []Instrument{}
	for _, i := range v.([]Instrument) {
		if strings.Contains(strings.ToLower(i.Symbol+" "+i.Name), strings.ToLower(query)) {
			out = append(out, i)
			if len(out) >= 12 {
				break
			}
		}
	}
	return out
}

func (s *marketService) search(ctx context.Context, query string) []Instrument {
	out := []Instrument{}
	seen := map[string]bool{}
	lower := strings.ToLower(query)
	for _, i := range builtins {
		if strings.Contains(strings.ToLower(i.Symbol+" "+i.Name+" "+i.Aliases+" "+i.Market), lower) {
			out = append(out, i)
			seen[i.ID] = true
		}
	}
	for _, i := range s.cryptoSearch(ctx, query) {
		if !seen[i.ID] {
			out = append(out, i)
			seen[i.ID] = true
		}
	}
	v, _, e := s.cached(ctx, "s:"+lower, 10*time.Minute, func(ctx context.Context) (any, error) {
		var v struct {
			Quotes []struct{ Symbol, Shortname, Longname, Exchange, ExchDisp, QuoteType string }
		}
		e := s.getJSON(ctx, "https://query1.finance.yahoo.com/v1/finance/search?q="+url.QueryEscape(query)+"&quotesCount=12&newsCount=0", &v)
		return v.Quotes, e
	})
	if e != nil {
		return out
	}
	// Use JSON normalization to keep the cache type independent of local anonymous types.
	b, _ := json.Marshal(v)
	var rows []struct{ Symbol, Shortname, Longname, Exchange, ExchDisp, QuoteType string }
	_ = json.Unmarshal(b, &rows)
	for _, r := range rows {
		kind := map[string]string{"EQUITY": "stock", "ETF": "etf", "INDEX": "index", "FUTURE": "future", "CRYPTOCURRENCY": "crypto"}[r.QuoteType]
		id := "y:" + r.Symbol
		if kind == "" || kind == "crypto" || seen[id] || !validID(id) {
			continue
		}
		name := r.Longname
		if name == "" {
			name = r.Shortname
		}
		out = append(out, Instrument{ID: id, Symbol: r.Symbol, Name: name, Market: r.ExchDisp, Kind: kind})
		seen[id] = true
	}
	return out
}
