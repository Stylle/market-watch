# Data sources and third-party notices

Verified 2026-11-15. The software license does not grant data redistribution rights.

A-share adapter verified 2026-11-16 against the provider's live JSON responses:

Tencent's returned `qt` snapshot is also used as a quote fallback: actual current price, previous close and source timestamp are validated. It is not derived from a historical candle. Missing first quotes display a dash, not 0.00.

- Eastmoney: https://quote.eastmoney.com/ . Chinese/code/pinyin search via `searchapi.eastmoney.com/api/suggest/get`; quotes via `push2.eastmoney.com/api/qt/stock/get`; unadjusted historical bars via `push2his.eastmoney.com/api/qt/stock/kline/get`. These are experimental public website endpoints, not a contracted API service. No availability, timeliness or redistribution license is implied. Deployment operators must verify permitted data usage.
- Price precision comes from `f59`; missing/zero OHLC fields remain unavailable rather than invented. Changes compare with provider's previous close. Volume is in hands (手), not shares. Latest-session intraday bars and monthly maximum history retain their actual source timestamps. Market status is marked unavailable when not supplied. Existing Yahoo-shaped China IDs remain compatible; a market suffix is mandatory.
- Tencent Finance historical fallback: https://gu.qq.com/ via fixed hosts `web.ifzq.gtimg.cn` and `web3.ifzq.gtimg.cn`. Used when Eastmoney history is unavailable (including observed EOF from Go containers). Unadjusted day/month bars use `/appstock/app/fqkline/get` without qfq/hfq, minute bars use `/appstock/app/kline/mkline` on the fixed web3 host (the old route returns 301). Day bars verified live; minute endpoints can remain unavailable, in which case the UI recommends switching to daily ranges rather than fabricating intraday data. Actual source is labeled separately. Same experimental availability/licensing limitations apply.

- Yahoo Finance https://finance.yahoo.com/ : experimental public `/v8/finance/chart` and `/v1/finance/search` adapters. Two Yahoo hosts are two routes of one source. No guarantees of availability, authorization, coverage or exact latency. Verify your usage against https://legal.yahoo.com/us/en/yahoo/terms/otos/index.html before public/commercial display.
- Coinbase Exchange public products, currencies, stats and candles: https://docs.cdp.coinbase.com/exchange/introduction/welcome and https://docs.cdp.coinbase.com/api-reference/exchange-api/rest-api/products/get-product-candles . Rate limits: https://docs.cdp.coinbase.com/exchange/rest-api/rate-limits . Product identity and USD pair retained. Stats are 24h, unlike equity prior-close comparison. Quote display records retrieval time, not an invented trade timestamp. Historical source limits are shown to users.
- Gold API https://gold-api.com/docs , https://gold-api.com/faqs , https://gold-api.com/terms : no key for spot price, optional key for history; free history/OHLC shared quota 10/h, prices per troy ounce, source timestamps UTC. In-app history uses daily average, not OHLC, and never falls back to futures. Private key sent only in header from server. Terms expressly permit commercial use but require respecting request limits.
- ECharts 6.1.0 / zrender (Apache-2.0 / BSD-3-Clause): https://echarts.apache.org/ . License files included alongside this document. Chart axes display device timezone, quote timestamps label the supplied exchange timezone.
- Phosphor Icons (MIT): https://phosphoricons.com/ . Local favicon and icon components; see PHOSPHOR-LICENSE.txt.
- Simple Icons brand SVGs (CC0; trademarks remain their owners'): https://github.com/simple-icons/simple-icons . NVIDIA and Toyota brand assets are local copies, see SIMPLE-ICONS-LICENSE.md. Other instruments use generic library icons rather than fabricated issuer logos.

No remote fonts, analytics or ad scripts. Test fixtures contain illustrative prices only and never enter production API output.
