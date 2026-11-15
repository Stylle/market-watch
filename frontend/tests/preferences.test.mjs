import test from "node:test";
import assert from "node:assert/strict";
import { read, save, validate, defaults, validID } from "../src/preferences.js";
const catalog = [
  {
    id: "y:AAPL",
    symbol: "AAPL",
    name: "Apple",
    market: "NASDAQ",
    currency: "USD",
    kind: "stock",
  },
  {
    id: "g:XAU",
    symbol: "XAU/USD",
    name: "Gold",
    market: "Spot",
    currency: "USD",
    kind: "metal",
  },
];
test("only versioned device preferences persist, no market prices or credentials", () => {
  let stored;
  const p = { ...defaults(catalog), price: 123, key: "never-store" };
  assert.equal(
    save(
      {
        setItem(k, v) {
          stored = JSON.parse(v);
        },
      },
      p,
    ),
    true,
  );
  assert.equal("price" in stored, false);
  assert.equal("key" in stored, false);
  assert.deepEqual(Object.keys(stored).sort(), [
    "colors",
    "items",
    "refresh",
    "selected",
    "theme",
    "version",
  ]);
});
test("empty watchlist persists and import rejects duplicates, oversized lists and invalid IDs", () => {
  const p = { ...defaults(catalog), items: [], selected: "" };
  assert.equal(validate(p, catalog, true).items.length, 0);
  assert.throws(() =>
    validate({ ...p, items: [catalog[0], catalog[0]] }, catalog, true),
  );
  assert.throws(() =>
    validate({ ...p, items: Array(51).fill(catalog[0]) }, catalog, true),
  );
  assert.equal(validID("y:../../etc"), false);
  assert.equal(validID("c:BTC-USDT"), false);
  assert.equal(validID("y:0700.HK"), true);
});
test("blocked and corrupt storage recover without exposing another device preferences", () => {
  assert.equal(
    read(
      {
        getItem() {
          throw Error("blocked");
        },
      },
      catalog,
    ).selected,
    "y:AAPL",
  );
  assert.equal(
    read(
      {
        getItem() {
          return "{broken";
        },
      },
      catalog,
    ).items.length,
    2,
  );
  assert.equal(
    save(
      {
        setItem() {
          throw Error("blocked");
        },
      },
      defaults(catalog),
    ),
    false,
  );
});
