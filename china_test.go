package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestChinaSecurityIdentity(t *testing.T) {
	for id, want := range map[string]string{"y:000001.SS": "1.000001", "y:000001.SZ": "0.000001", "y:920002.BJ": "0.920002", "y:000300.SS": "1.000300"} {
		got, ok := chinaSecurity(id)
		if !ok || got != want || !validID(id) {
			t.Fatalf("%s: %s", id, got)
		}
	}
	for _, id := range []string{"y:0700.HK", "y:AAPL", "y:000300", "y:../000300.SS"} {
		if _, ok := chinaSecurity(id); ok {
			t.Fatal(id)
		}
	}
}

func TestChinaSearchChineseAndCodes(t *testing.T) {
	for _, query := range []string{"沪深300", "平安银行", "000001", "HS300"} {
		s := newService()
		s.getJSON = func(ctx context.Context, raw string, out any) error {
			u, _ := url.Parse(raw)
			if u.Host != "searchapi.eastmoney.com" {
				return errors.New("other provider unavailable")
			}
			if u.Query().Get("input") != query {
				t.Errorf("query lost: %s", raw)
			}
			return json.Unmarshal([]byte(`{"QuotationCodeTable":{"Status":0,"Data":[{"Code":"000300","Name":"沪深300","PinYin":"HS300","QuoteID":"1.000300","Classify":"Index"},{"Code":"000001","Name":"平安银行","QuoteID":"0.000001","Classify":"AStock"},{"Code":"510300","Name":"沪深300ETF","QuoteID":"1.510300","Classify":"Fund"},{"Code":"000001","Name":"fake","QuoteID":"https://localhost","Classify":"Index"}]}}`), out)
		}
		items := s.search(context.Background(), query)
		seen := map[string]int{}
		for _, i := range items {
			seen[i.ID]++
			if chinaID.MatchString(i.ID) && i.Currency != "CNY" {
				t.Fatal(i)
			}
		}
		for _, id := range []string{"y:000300.SS", "y:000001.SZ", "y:510300.SS"} {
			if seen[id] != 1 {
				t.Fatalf("%s %s count %d", query, id, seen[id])
			}
		}
	}
}

func TestChinaQuotePrecisionAndMissingFields(t *testing.T) {
	s := newService()
	s.getJSON = func(ctx context.Context, raw string, out any) error {
		return json.Unmarshal([]byte(`{"rc":0,"data":{"f57":"000300","f58":"沪深300","f59":2,"f43":401623,"f60":401629,"f46":0,"f44":"-","f45":null,"f47":0,"f86":1794791862}}`), out)
	}
	q, err := s.chinaQuote(context.Background(), "y:000300.SS", "1.000300")
	if err != nil || q.Price != 4016.23 || q.Previous == nil || *q.Previous != 4016.29 || q.Change == nil || *q.Change >= 0 || q.Open != nil || q.High != nil || q.Low != nil || q.MarketCap != nil || q.Currency != "CNY" || q.Kind != "index" {
		t.Fatalf("%+v %v", q, err)
	}
	for _, body := range []string{`{"rc":0,"data":null}`, `{"rc":0,"data":{"f59":2,"f43":"-","f86":1794791862}}`, `{"rc":0,"data":{"f57":"600519","f59":2,"f43":143400,"f86":1794791862}}`} {
		s.getJSON = func(ctx context.Context, raw string, out any) error { return json.Unmarshal([]byte(body), out) }
		if _, err = s.chinaQuote(context.Background(), "y:000300.SS", "1.000300"); err == nil {
			t.Fatal("invalid quote accepted", body)
		}
	}
}

func TestChinaChartsKeepProviderValuesAndSession(t *testing.T) {
	for _, period := range []string{"1d", "5d", "1mo", "3mo", "1y", "max"} {
		s := newService()
		s.getJSON = func(ctx context.Context, raw string, out any) error {
			u, _ := url.Parse(raw)
			if u.Host != "push2his.eastmoney.com" || u.Query().Get("fqt") != "0" {
				t.Errorf("%s", raw)
			}
			return json.Unmarshal([]byte(`{"rc":0,"data":{"code":"000300","klines":["2026-11-12 15:00,1,4000,1,1,25","2026-11-13 09:35,1,4016.23,1,1,40","2026-11-13 09:40,1,-1,1,1,40","malformed"]}}`), out)
		}
		c, err := s.chart(context.Background(), "y:000300.SS", period)
		want := 2
		if period == "1d" {
			want = 1
		}
		if err != nil || len(c.Points) != want || c.Points[len(c.Points)-1].Close != 4016.23 || *c.Points[len(c.Points)-1].Volume != 40 || c.Timezone != "Asia/Shanghai" || !strings.Contains(c.Source, "东方财富") {
			t.Fatalf("%s %+v %v", period, c, err)
		}
	}
}

func TestChinaSourceFailureDoesNotFabricateQuote(t *testing.T) {
	s := newService()
	s.getJSON = func(context.Context, string, any) error { return errors.New("offline") }
	q, err := s.quote(context.Background(), "y:000300.SS")
	if err == nil || q.Error == "" || q.Price != 0 {
		t.Fatal(q, err)
	}
}

func TestChinaHistoryFallbackIsExplicitAndUnadjusted(t *testing.T) {
	for _, period := range []string{"1d", "5d", "1mo", "3mo", "1y", "max"} {
		s := newService()
		s.getJSON = func(ctx context.Context, raw string, out any) error {
			u, _ := url.Parse(raw)
			if u.Host != "web.ifzq.gtimg.cn" && u.Host != "web3.ifzq.gtimg.cn" {
				return errors.New("primary EOF")
			}
			param := u.Query().Get("param")
			if strings.Contains(param, "qfq") || strings.Contains(param, "hfq") {
				t.Fatal(param)
			}
			parts := strings.Split(param, ",")
			frame := parts[1]
			stamp := time.Now().Format("2006-01-02")
			if frame == "m5" || frame == "m30" {
				stamp = time.Now().Format("200601021504")
			}
			body, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]any{"sh000300": map[string]any{frame: [][]string{{stamp, "4000", "4016.23", "4020", "3990", "25"}}}}})
			return json.Unmarshal(body, out)
		}
		c, err := s.chart(context.Background(), "y:000300.SS", period)
		if err != nil || len(c.Points) != 1 || !strings.Contains(c.Source, "腾讯财经") || c.Points[0].Close != 4016.23 {
			t.Fatalf("%s %+v %v", period, c, err)
		}
	}
}

func TestChinaQuoteFallbackUsesSnapshotNotCandle(t *testing.T) {
	s := newService()
	s.getJSON = func(ctx context.Context, raw string, out any) error {
		if !strings.Contains(raw, "web.ifzq.gtimg.cn") {
			return errors.New("EOF")
		}
		r := make([]string, 35)
		r[1] = "沪深300"
		r[2] = "000300"
		r[3] = "3981.39"
		r[4] = "4016.29"
		r[5] = "3984.77"
		r[6] = "17224491"
		r[30] = "20261116093700"
		r[33] = "3986.49"
		r[34] = "3975.58"
		body, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]any{"sh000300": map[string]any{"qt": map[string]any{"sh000300": r}}}})
		return json.Unmarshal(body, out)
	}
	q, err := s.quote(context.Background(), "y:000300.SS")
	if err != nil || q.Price != 3981.39 || q.Source != "腾讯财经" || q.Previous == nil || *q.Previous != 4016.29 {
		t.Fatal(q, err)
	}
}
