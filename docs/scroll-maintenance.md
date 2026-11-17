# List and chart scrolling maintenance

Included in v0.1.5, 2026-11-17.

The watchlist and detail panel now have independent scrolling within a viewport-sized application frame. Adding up to50 instruments no longer stretches the chart or pushes it off-screen when scrolling the list. Wide-screen horizontal margins remain. Short viewports retain scrollable detail content rather than clipping statistics. Fatal/retry content also remains scrollable.

Sortable edge auto-scroll is contained to the nearest list and no longer bubbles to the document. Non-passive touchmove cancellation starts after long-press selection, preserving ordinary short-swipe scrolling before selection. Selection/end/unmount cleanup releases the gesture lock. Phone detail selection resets the detail scroll position; returning keeps the watchlist position. Preferences, stored order, APIs and prices are unchanged.

Validation uses isolated browser contexts and synthetic market responses only: existing desktop/mobile regression at375–3840 widths, 50 instruments across seven desktop/tablet/short-height sizes, eight touch reorders including edge scrolling on phone and touch-enabled tablet, visible chart sizing after navigation, return-position preservation and no document movement/overflow. Frontend tests/build and Go tests/vet passed. Screenshots were visually inspected; physical Safari/iOS hardware has not been exercised.
