export const KEY = "market-watch.preferences.v1";
export const DEFAULT_IDS = [
  "y:AAPL",
  "y:NVDA",
  "y:0700.HK",
  "y:7203.T",
  "c:BTC-USD",
  "c:ETH-USD",
  "g:XAU",
  "y:^HSI",
];
const knownKinds = new Set([
  "stock",
  "index",
  "etf",
  "crypto",
  "metal",
  "future",
]);
export function validID(id) {
  return (
    typeof id === "string" &&
    id.length <= 28 &&
    /^(y:[A-Za-z0-9^][A-Za-z0-9.^=\-]{0,23}|c:[A-Z0-9]{2,12}-USD|g:XA[UG])$/.test(
      id,
    )
  );
}
export function defaults(catalog) {
  return {
    version: 1,
    items: DEFAULT_IDS.map((id) => catalog.find((i) => i.id === id)).filter(
      Boolean,
    ),
    selected: "y:AAPL",
    theme: "dark",
    colors: "green-up",
    refresh: 60,
  };
}
export function validate(raw, catalog, strict = false) {
  const fallback = defaults(catalog);
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    if (strict) throw Error("偏好文件无效");
    return fallback;
  }
  if (raw.version !== 1 || !Array.isArray(raw.items) || raw.items.length > 50) {
    if (strict) throw Error("偏好版本或标的数量无效");
    return fallback;
  }
  const seen = new Set();
  const items = [];
  for (const x of raw.items) {
    if (
      !x ||
      !validID(x.id) ||
      seen.has(x.id) ||
      !knownKinds.has(x.kind) ||
      typeof x.name !== "string" ||
      x.name.length > 150 ||
      typeof x.symbol !== "string" ||
      x.symbol.length > 30
    ) {
      if (strict) throw Error("标的内容无效或重复");
      continue;
    }
    seen.add(x.id);
    items.push({
      id: x.id,
      name: x.name,
      symbol: x.symbol,
      market: typeof x.market === "string" ? x.market.slice(0, 60) : "",
      currency: typeof x.currency === "string" ? x.currency.slice(0, 10) : "",
      kind: x.kind,
    });
  }
  return {
    version: 1,
    items,
    selected: items.some((i) => i.id === raw.selected)
      ? raw.selected
      : items[0]?.id || "",
    theme: ["light", "dark", "system"].includes(raw.theme) ? raw.theme : "dark",
    colors: raw.colors === "red-up" ? "red-up" : "green-up",
    refresh: [0, 60, 120, 300].includes(raw.refresh) ? raw.refresh : 60,
  };
}
export function read(storage, catalog) {
  try {
    const raw = storage.getItem(KEY);
    return raw ? validate(JSON.parse(raw), catalog) : defaults(catalog);
  } catch {
    return defaults(catalog);
  }
}
export function save(storage, prefs) {
  try {
    const p = validate(prefs, [], true);
    storage.setItem(KEY, JSON.stringify(p));
    return true;
  } catch {
    return false;
  }
}
export function searchLocal(items, query) {
  const q = query.toLowerCase().trim();
  return items.filter((x) =>
    `${x.symbol} ${x.name} ${x.market} ${x.aliases || ""}`
      .toLowerCase()
      .includes(q),
  );
}
