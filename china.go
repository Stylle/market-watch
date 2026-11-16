package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var chinaID = regexp.MustCompile(`^y:([0-9]{6})\.(SS|SZ|BJ)$`)

// Keep existing Yahoo-shaped IDs, including imported watchlists. The market suffix
// is mandatory: Shanghai 000001 is an index, Shenzhen 000001 is a different stock.
func chinaSecurity(id string) (string, bool) {
	m := chinaID.FindStringSubmatch(id)
	if m == nil {
		return "", false
	}
	market := "0"
	if m[2] == "SS" {
		market = "1"
	}
	return market + "." + m[1], true
}

type chinaSuggestion struct {
	Code, Name, PinYin, QuoteID, SecurityTypeName, Classify string
}

func chinaInstrument(r chinaSuggestion) (Instrument, bool) {
	if !regexp.MustCompile(`^[01]\.[0-9]{6}$`).MatchString(r.QuoteID) || r.QuoteID[2:] != r.Code || r.Name == "" {
		return Instrument{}, false
	}
	kind := map[string]string{"AStock": "stock", "Index": "index", "Fund": "etf"}[r.Classify]
	if kind == "" {
		return Instrument{}, false
	}
	suffix, market := ".SZ", "SZSE"
	if r.QuoteID[0] == '1' {
		suffix, market = ".SS", "SSE"
	}
	if strings.Contains(r.SecurityTypeName, "京") || strings.HasPrefix(r.Code, "92") || strings.HasPrefix(r.Code, "83") || strings.HasPrefix(r.Code, "87") {
		suffix, market = ".BJ", "BSE"
	}
	// Do not label closed-end funds/LOFs as ETFs just because the provider says Fund.
	if kind == "etf" && !strings.Contains(strings.ToUpper(r.Name), "ETF") {
		return Instrument{}, false
	}
	return Instrument{ID: "y:" + r.Code + suffix, Symbol: r.Code, Name: r.Name, Market: market, Currency: "CNY", Kind: kind, Aliases: r.PinYin}, true
}

func (s *marketService) chinaSearch(ctx context.Context, query string) []Instrument {
	v, _, err := s.cached(ctx, "cn-search:"+strings.ToLower(query), 10*time.Minute, func(ctx context.Context) (any, error) {
		var response struct {
			QuotationCodeTable struct {
				Data   []chinaSuggestion
				Status int
			}
		}
		err := s.getJSON(ctx, "https://searchapi.eastmoney.com/api/suggest/get?type=14&count=20&input="+url.QueryEscape(query), &response)
		if err != nil {
			return nil, err
		}
		if response.QuotationCodeTable.Status != 0 {
			return nil, errors.New("A股搜索暂不可用")
		}
		out := []Instrument{}
		for _, r := range response.QuotationCodeTable.Data {
			if i, ok := chinaInstrument(r); ok {
				out = append(out, i)
			}
		}
		return out, nil
	})
	if err != nil {
		return nil
	}
	return v.([]Instrument)
}

// Missing fields (including provider's "-") remain nil, not invented zero prices.
func chinaNumber(raw json.RawMessage) *float64 {
	v, err := strconv.ParseFloat(strings.Trim(string(raw), `"`), 64)
	if err != nil {
		return nil
	}
	return number(v)
}

func (s *marketService) chinaQuote(ctx context.Context, id, secid string) (Quote, error) {
	var response struct {
		RC   int                        `json:"rc"`
		Data map[string]json.RawMessage `json:"data"`
	}
	err := s.getJSON(ctx, "https://push2.eastmoney.com/api/qt/stock/get?secid="+secid+"&fields=f57,f58,f59,f43,f60,f46,f44,f45,f47,f116,f86", &response)
	if err != nil {
		return Quote{}, err
	}
	d := response.Data
	precision, price, at := chinaNumber(d["f59"]), chinaNumber(d["f43"]), chinaNumber(d["f86"])
	var code, name string
	_ = json.Unmarshal(d["f57"], &code)
	_ = json.Unmarshal(d["f58"], &name)
	if response.RC != 0 || precision == nil || *precision < 0 || *precision > 6 || math.Trunc(*precision) != *precision || price == nil || *price <= 0 || at == nil || *at <= 0 || code != secid[2:] {
		return Quote{}, errors.New("A股来源暂无有效报价")
	}
	scale := math.Pow10(int(*precision))
	field := func(key string) *float64 {
		v := chinaNumber(d[key])
		if v == nil || *v <= 0 {
			return nil
		}
		return number(*v / scale)
	}
	i := instrument(id)
	market := "SZSE"
	if strings.HasSuffix(id, ".SS") {
		market = "SSE"
	}
	if strings.HasSuffix(id, ".BJ") {
		market = "BSE"
	}
	q := Quote{ID: id, Symbol: code, Name: name, Market: market, Currency: "CNY", Kind: i.Kind, Price: *price / scale, Previous: field("f60"), Open: field("f46"), High: field("f44"), Low: field("f45"), Volume: chinaNumber(d["f47"]), MarketCap: chinaNumber(d["f116"]), AsOf: int64(*at), Fetched: time.Now().Unix(), Timezone: "Asia/Shanghai", Basis: "较昨收", Source: "东方财富", SourceURL: "https://quote.eastmoney.com/", Spark: []Point{}, State: "交易状态未提供"}
	if q.Volume != nil && *q.Volume < 0 {
		q.Volume = nil
	}
	if q.MarketCap != nil && *q.MarketCap <= 0 {
		q.MarketCap = nil
	}
	changes(&q)
	return q, nil
}

func (s *marketService) chinaChart(ctx context.Context, id, secid, period string) (Chart, error) {
	c, err := s.eastmoneyChart(ctx, id, secid, period)
	if err == nil {
		return c, nil
	}
	if ctx.Err() != nil {
		return Chart{}, ctx.Err()
	}
	c, err = s.tencentChart(ctx, id, period)
	if err != nil && (period == "1d" || period == "5d") {
		return Chart{}, errors.New("A股分时来源暂不可用，请切换1月、3月或1年查看日线")
	}
	return c, err
}

func (s *marketService) eastmoneyChart(ctx context.Context, id, secid, period string) (Chart, error) {
	if _, ok := ranges[period]; !ok {
		return Chart{}, errors.New("无效历史区间")
	}
	now := time.Now()
	days := map[string]int{"1d": 1, "5d": 9, "1mo": 31, "3mo": 93, "1y": 366}[period]
	klt := "101"
	if period == "1d" {
		klt = "5"
	}
	if period == "5d" {
		klt = "30"
	}
	if period == "max" {
		klt = "103"
	}
	// Intraday requests use the latest trading session, including weekends.
	beg := "19900101"
	if period != "max" && period != "1d" {
		beg = now.AddDate(0, 0, -days).Format("20060102")
	}
	lmt := "10000"
	if period == "1d" {
		lmt = "48"
	}
	u := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=%s&fqt=0&beg=%s&end=20500101&lmt=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56", secid, klt, beg, lmt)
	var response struct {
		RC   int `json:"rc"`
		Data *struct {
			Code   string
			Klines []string
		} `json:"data"`
	}
	if err := s.getJSON(ctx, u, &response); err != nil {
		return Chart{}, err
	}
	if response.RC != 0 || response.Data == nil || response.Data.Code != secid[2:] {
		return Chart{}, errors.New("A股来源暂无历史数据")
	}
	c := Chart{ID: id, Range: period, Source: "东方财富 · 不复权", Timezone: "Asia/Shanghai", Fetched: now.Unix(), Points: []Point{}}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, line := range response.Data.Klines {
		fields := strings.Split(line, ",")
		if len(fields) < 6 {
			continue
		}
		layout := "2006-01-02"
		if strings.Contains(fields[0], " ") {
			layout = "2006-01-02 15:04"
		}
		stamp, err := time.ParseInLocation(layout, fields[0], loc)
		close, err2 := strconv.ParseFloat(fields[2], 64)
		if err != nil || err2 != nil || positive(close) == nil {
			continue
		}
		volume := chinaNumber(json.RawMessage(fields[5]))
		if volume != nil && *volume < 0 {
			volume = nil
		}
		c.Points = append(c.Points, Point{Time: stamp.Unix(), Close: close, Volume: volume})
	}
	sort.Slice(c.Points, func(i, j int) bool { return c.Points[i].Time < c.Points[j].Time })
	if period == "1d" && len(c.Points) > 0 {
		date := time.Unix(c.Points[len(c.Points)-1].Time, 0).In(loc).Format("2006-01-02")
		start := 0
		for start < len(c.Points) && time.Unix(c.Points[start].Time, 0).In(loc).Format("2006-01-02") != date {
			start++
		}
		c.Points = c.Points[start:]
		c.Message = "最近交易日的5分钟线；成交量单位：手"
	} else {
		c.Message = "不复权行情；成交量单位：手"
	}
	if len(c.Points) == 0 {
		return Chart{}, errors.New("此区间暂无A股历史数据")
	}
	return c, nil
}

// Tencent is an independent historical fallback, with its own explicit source
// label. Never substitute adjusted bars or daily bars for intraday silently.
func (s *marketService) tencentChart(ctx context.Context, id, period string) (Chart, error) {
	m := chinaID.FindStringSubmatch(id)
	if m == nil {
		return Chart{}, errors.New("无效A股标的")
	}
	prefix := map[string]string{"SS": "sh", "SZ": "sz", "BJ": "bj"}[m[2]]
	symbol := prefix + m[1]
	frame, count, path := "day", 40, "fqkline/get"
	switch period {
	case "1d":
		frame, count, path = "m5", 48, "kline/mkline"
	case "5d":
		frame, count, path = "m30", 80, "kline/mkline"
	case "1mo":
		count = 40
	case "3mo":
		count = 110
	case "1y":
		count = 400
	case "max":
		frame, count = "month", 1200
	default:
		return Chart{}, errors.New("无效历史区间")
	}
	param := fmt.Sprintf("%s,%s,,,%d,", symbol, frame, count)
	if strings.HasPrefix(frame, "m") && frame != "month" {
		param = fmt.Sprintf("%s,%s,,%d", symbol, frame, count)
	}
	// The minute route has one extra /kline segment; daily/monthly use fqkline.
	host := "web.ifzq.gtimg.cn"
	if frame == "m5" || frame == "m30" {
		host = "web3.ifzq.gtimg.cn"
	}
	u := "https://" + host + "/appstock/app/" + path + "?param=" + url.QueryEscape(param)
	var response struct {
		Code int                                   `json:"code"`
		Data map[string]map[string]json.RawMessage `json:"data"`
	}
	if err := s.getJSON(ctx, u, &response); err != nil {
		return Chart{}, err
	}
	var rows [][]string
	if response.Code != 0 || json.Unmarshal(response.Data[symbol][frame], &rows) != nil {
		return Chart{}, errors.New("A股后备来源暂无历史数据")
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now()
	days := map[string]int{"1mo": 31, "3mo": 93, "1y": 366}[period]
	c := Chart{ID: id, Range: period, Source: "腾讯财经 · 不复权", Timezone: "Asia/Shanghai", Fetched: now.Unix(), Points: []Point{}, Message: "后备历史来源；不复权行情，成交量单位：手"}
	for _, r := range rows {
		if len(r) < 6 {
			continue
		}
		layout := "2006-01-02"
		if frame == "m5" || frame == "m30" {
			layout = "200601021504"
		}
		stamp, err := time.ParseInLocation(layout, r[0], loc)
		close, err2 := strconv.ParseFloat(r[2], 64)
		if err != nil || err2 != nil || positive(close) == nil || (days > 0 && stamp.Before(now.AddDate(0, 0, -days))) {
			continue
		}
		volume := chinaNumber(json.RawMessage(r[5]))
		if volume != nil && *volume < 0 {
			volume = nil
		}
		c.Points = append(c.Points, Point{Time: stamp.Unix(), Close: close, Volume: volume})
	}
	sort.Slice(c.Points, func(i, j int) bool { return c.Points[i].Time < c.Points[j].Time })
	if period == "1d" && len(c.Points) > 0 {
		date := time.Unix(c.Points[len(c.Points)-1].Time, 0).In(loc).Format("2006-01-02")
		start := 0
		for start < len(c.Points) && time.Unix(c.Points[start].Time, 0).In(loc).Format("2006-01-02") != date {
			start++
		}
		c.Points = c.Points[start:]
	}
	if len(c.Points) == 0 {
		return Chart{}, errors.New("此区间暂无A股历史数据")
	}
	return c, nil
}

func (s *marketService) tencentQuote(ctx context.Context, id string) (Quote, error) {
	m := chinaID.FindStringSubmatch(id)
	if m == nil {
		return Quote{}, errors.New("无效A股标的")
	}
	symbol := map[string]string{"SS": "sh", "SZ": "sz", "BJ": "bj"}[m[2]] + m[1]
	var response struct {
		Code int `json:"code"`
		Data map[string]struct {
			QT map[string][]string `json:"qt"`
		} `json:"data"`
	}
	err := s.getJSON(ctx, "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param="+url.QueryEscape(symbol+",day,,,2,"), &response)
	if err != nil {
		return Quote{}, err
	}
	r := response.Data[symbol].QT[symbol]
	if response.Code != 0 || len(r) < 35 || r[2] != m[1] {
		return Quote{}, errors.New("A股后备来源暂无有效报价")
	}
	field := func(index int) *float64 {
		v, e := strconv.ParseFloat(r[index], 64)
		if e != nil {
			return nil
		}
		return positive(v)
	}
	price := field(3)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	stamp, e := time.ParseInLocation("20060102150405", r[30], loc)
	if price == nil || e != nil {
		return Quote{}, errors.New("A股后备报价价格或时间无效")
	}
	i := instrument(id)
	market := map[string]string{"SS": "SSE", "SZ": "SZSE", "BJ": "BSE"}[m[2]]
	q := Quote{ID: id, Symbol: m[1], Name: r[1], Market: market, Currency: "CNY", Kind: i.Kind, Price: *price, Previous: field(4), Open: field(5), High: field(33), Low: field(34), Volume: field(6), AsOf: stamp.Unix(), Fetched: time.Now().Unix(), Timezone: "Asia/Shanghai", Basis: "较昨收", Source: "腾讯财经", SourceURL: "https://gu.qq.com/" + symbol, Spark: []Point{}, State: "交易状态未提供"}
	changes(&q)
	return q, nil
}
