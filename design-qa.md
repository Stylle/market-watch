# Market Watch design and interaction QA

final result: passed

Selected target: the user-approved Graphite Watch image produced on 2026-11-15. Compared the selected image and current implementation together at 1487×1058, with the same AAPL-selected, one-month-chart, eight-instrument state using deterministic test-only quotes. A separate browser context confirmed real production endpoints render live provider values, not the fixture prices.

The implementation retains the graphite dual-column composition, compact watchlist, blue selection and area chart, large quote, period selector, volume bars and statistics. Missing market cap or spot baselines remain dashes; generic library symbols replace issuer marks for which no appropriate licensed local asset is available. API-required annotations distinguish reference-market latency, sessions, exchange timezones and source retrieval times. These are intentional factual differences from the generated concept, which used illustrative data.

Validated widths 375, 390, 700, 701, 834, 1024 and 1487 without horizontal overflow. Mobile list/detail navigation and back focus work; tablet retains both columns. Search/add, reorder/remove, asset filters, latest chart request ordering, optional gold-history errors with working spot quotes, preferences export/import and rejected invalid import all pass. Separate contexts keep independent settings. No browser runtime errors. Screenshots and fixture preference exports stay outside the repository under workspace runtime/qa-market-watch.

Backend tests cover prior trading-day close, sessions, identifiers, single-flight/cancellation, stale caching, 429 cooldown, fixed upstream hosts, Coinbase time bounds, gold-key omission and malformed API/static requests. Linux race detector, go vet, frontend tests/build, dependency audit and non-root Docker health/real-source checks pass.

Remaining P3: issuer-specific icon coverage can be expanded using properly licensed assets. Individual sources have deliberately documented limits: no gold history without a key; Coinbase selected history is limited to three months; Yahoo public adapters are experimental and can be delayed or unavailable. The UI displays these limits rather than claiming complete universal coverage.
## Wide-screen follow-up

The user reported large outer margins on a 4K display. Removed the 1600px cap above the wide-desktop breakpoint, constrained the watchlist to 400–560px, and gave the remaining width/height to the chart. On screens at least 2000px wide, statistics use six adjacent columns so labels and values remain together. Fixed ECharts volume-grid positioning to reserve a separate bottom band at every chart height; a percentage-positioned volume grid had overlapped the price grid when the chart grew vertically.

Rechecked 1920px, 2560×1440 and 3840×2160 in addition to phone/tablet/standard desktop. The app occupies at least 95% of wide viewports, has no horizontal overflow and preserves all navigation/settings interactions. Compared the original reference, unchanged standard-desktop composition and updated wide screenshot together. Result remains passed.
## Title encoding patch (v0.1.1)

The original HTML title contained literal U+FFFD replacement characters in source; this was not a browser font or response-charset problem. Replaced the damaged title with ASCII numeric HTML entities that decode to the intended Chinese title. Scanned tracked project text and the Currency Desk HTML title; no additional replacement-character corruption was found. Added a frontend title/charset regression test and a browser assertion on document.title. Existing responsive and interaction regression checks remain passing.
