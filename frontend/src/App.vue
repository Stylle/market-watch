<script setup lang="ts">
import {
  computed,
  nextTick,
  onMounted,
  onUnmounted,
  reactive,
  ref,
  watch,
} from "vue";
import {
  PhMagnifyingGlass,
  PhPlus,
  PhSun,
  PhMoon,
  PhDesktop,
  PhGearSix,
  PhX,
  PhArrowLeft,
  PhArrowClockwise,
  PhDotsThree,
  PhArrowUp,
  PhArrowDown,
  PhTrash,
  PhExport,
  PhUpload,
  PhWarningCircle,
  PhCheck,
  PhChartLine,
  PhInfo,
} from "@phosphor-icons/vue";
import AssetIcon from "./AssetIcon.vue";
import PriceChart from "./PriceChart.vue";
import { defaults, read, save, validate, searchLocal } from "./preferences.js";
import type { Instrument, Quote, Chart } from "./types";
const catalog = ref<Instrument[]>([]),
  prefs = reactive(defaults([] as Instrument[])),
  ready = ref(false),
  fatal = ref(""),
  storageFailed = ref(false),
  quotes = ref<Record<string, Quote>>({}),
  quoteLoading = ref(false),
  quoteError = ref(""),
  chart = ref<Chart | null>(null),
  chartLoading = ref(false),
  chartError = ref(""),
  period = ref("1mo"),
  filter = ref("all"),
  mobileDetail = ref(false),
  menu = ref(""),
  toast = ref(""),
  search = ref(""),
  searchItems = ref<Instrument[]>([]),
  searchLoading = ref(false),
  searchError = ref("");
const searchDialog = ref<HTMLDialogElement>(),
  settingsDialog = ref<HTMLDialogElement>(),
  searchInput = ref<HTMLInputElement>(),
  importInput = ref<HTMLInputElement>();
const selected = computed(
    () =>
      prefs.items.find((i: Instrument) => i.id === prefs.selected) as
        | Instrument
        | undefined,
  ),
  q = computed(() => quotes.value[prefs.selected]),
  items = computed(() =>
    prefs.items.filter(
      (i: Instrument) =>
        filter.value === "all" ||
        (filter.value === "stock" &&
          ["stock", "etf", "index", "future"].includes(i.kind)) ||
        (filter.value === "crypto" && i.kind === "crypto") ||
        (filter.value === "metal" && i.kind === "metal"),
    ),
  );
const tabs = [
  ["all", "全部"],
  ["stock", "股票"],
  ["crypto", "加密"],
  ["metal", "黄金"],
];
const periods = [
  ["1d", "1日"],
  ["5d", "1周"],
  ["1mo", "1月"],
  ["3mo", "3月"],
  ["1y", "1年"],
  ["max", "全部"],
];
const themeValue = ref("dark");
let quoteController: AbortController | undefined,
  chartController: AbortController | undefined,
  searchController: AbortController | undefined,
  quoteSeq = 0,
  chartSeq = 0,
  searchSeq = 0,
  searchTimer: ReturnType<typeof setTimeout>,
  timer: ReturnType<typeof setInterval> | undefined,
  toastTimer: ReturnType<typeof setTimeout>,
  returnFocus: HTMLElement | null = null,
  media: MediaQueryList | undefined;
const dateLabel = new Date().toLocaleDateString("zh-CN", {
  year: "numeric",
  month: "long",
  day: "numeric",
  weekday: "long",
});
const kinds: Record<string, string> = {
  stock: "股票",
  index: "指数",
  etf: "ETF",
  crypto: "加密货币",
  metal: "现货贵金属",
  future: "期货",
};
function number(n: number | null | undefined, d = 2) {
  return n == null || !Number.isFinite(n)
    ? "—"
    : n.toLocaleString("en-US", {
        minimumFractionDigits: d,
        maximumFractionDigits: d,
      });
}
function compact(n: number | null | undefined) {
  if (n == null) return "—";
  if (Math.abs(n) >= 1e12) return number(n / 1e12) + "万亿";
  if (Math.abs(n) >= 1e8) return number(n / 1e8) + "亿";
  if (Math.abs(n) >= 1e4) return number(n / 1e4) + "万";
  return number(n, 0);
}
function signed(n: number | null | undefined, suffix = "") {
  return n == null ? "—" : (n > 0 ? "+" : "") + number(n) + suffix;
}
function direction(n: number | null | undefined) {
  return n == null || n === 0 ? "neutral" : n > 0 ? "up" : "down";
}
function at(n: number | undefined, zone?: string) {
  if (!n) return "—";
  try {
    return new Date(n * 1000).toLocaleString("zh-CN", {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      timeZone: zone || undefined,
      hour12: false,
    });
  } catch {
    return "—";
  }
}
function notify(s: string) {
  toast.value = s;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (toast.value = ""), 3500);
}
function applyTheme() {
  themeValue.value =
    prefs.theme === "light"
      ? "light"
      : prefs.theme === "system" &&
          !matchMedia("(prefers-color-scheme:dark)").matches
        ? "light"
        : "dark";
  document.documentElement.dataset.theme = themeValue.value;
  document.documentElement.dataset.colors = prefs.colors;
}
watch(
  () => [prefs.items, prefs.selected, prefs.theme, prefs.colors, prefs.refresh],
  () => {
    if (ready.value) {
      try {
        storageFailed.value = !save(localStorage, prefs);
      } catch {
        storageFailed.value = true;
      }
    }
    applyTheme();
  },
  { deep: true },
);
watch(
  () => prefs.selected,
  () => {
    menu.value = "";
    loadChart();
  },
);
watch(period, () => loadChart());
watch(
  () => prefs.items.map((i: Instrument) => i.id).join(","),
  () => {
    if (ready.value) loadQuotes();
  },
);
watch(() => prefs.refresh, setupTimer);
async function jsonFetch(url: string, init: RequestInit = {}, timeout = 20000) {
  const controller = init.signal as AbortSignal;
  const timed = AbortSignal.timeout(timeout);
  const signal = controller ? AbortSignal.any([controller, timed]) : timed;
  const r = await fetch(url, { ...init, signal });
  let body;
  try {
    body = await r.json();
  } catch {
    throw Error("服务响应无效，请重试");
  }
  if (!r.ok) throw Error(body.error || body.message || "暂时无法读取行情");
  return body;
}
async function loadQuotes() {
  if (!ready.value) return;
  quoteController?.abort();
  const id = ++quoteSeq,
    controller = new AbortController();
  quoteController = controller;
  const wanted = prefs.items.map((i: Instrument) => i.id);
  if (!wanted.length) {
    quotes.value = {};
    quoteLoading.value = false;
    return;
  }
  quoteLoading.value = true;
  quoteError.value = "";
  try {
    const r = await jsonFetch(
      "/api/quotes",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: wanted }),
        signal: controller.signal,
      },
      26000,
    );
    if (id !== quoteSeq) return;
    const next: Record<string, Quote> = {};
    for (const v of r.quotes) {
      if (!wanted.includes(v.id)) continue;
      if (v.error && quotes.value[v.id]?.price)
        next[v.id] = { ...quotes.value[v.id], stale: true, error: v.error };
      else next[v.id] = v;
    }
    quotes.value = next;
  } catch (e) {
    if (id !== quoteSeq || controller.signal.aborted) return;
    quoteError.value = e instanceof Error ? e.message : "行情读取失败";
    quotes.value = Object.fromEntries(
      Object.entries(quotes.value).map(([id, q]) => [
        id,
        { ...q, stale: true },
      ]),
    );
  } finally {
    if (id === quoteSeq) quoteLoading.value = false;
  }
}
async function loadChart() {
  chartController?.abort();
  const id = ++chartSeq,
    controller = new AbortController();
  chartController = controller;
  chart.value = null;
  chartError.value = "";
  chartLoading.value = false;
  if (!prefs.selected) return;
  chartLoading.value = true;
  try {
    const r = await jsonFetch(
      "/api/chart?id=" +
        encodeURIComponent(prefs.selected) +
        "&range=" +
        period.value,
      { signal: controller.signal },
    );
    if (id === chartSeq) chart.value = r;
  } catch (e) {
    if (id !== chartSeq || controller.signal.aborted) return;
    chartError.value = e instanceof Error ? e.message : "图表暂不可用";
  } finally {
    if (id === chartSeq) chartLoading.value = false;
  }
}
function select(i: Instrument) {
  prefs.selected = i.id;
  mobileDetail.value = true;
  nextTick(() => document.querySelector<HTMLElement>(".detail-title")?.focus());
}
function back() {
  mobileDetail.value = false;
  nextTick(() =>
    document
      .querySelector<HTMLElement>(".watch-row.selected .row-select")
      ?.focus(),
  );
}
function move(id: string, offset: number) {
  const list = [...prefs.items],
    i = list.findIndex((v: Instrument) => v.id === id),
    to = i + offset;
  if (to < 0 || to >= list.length) return;
  [list[i], list[to]] = [list[to], list[i]];
  prefs.items = list;
}
function remove(id: string) {
  prefs.items = prefs.items.filter((v: Instrument) => v.id !== id);
  if (prefs.selected === id) prefs.selected = prefs.items[0]?.id || "";
  menu.value = "";
  if (!prefs.items.length) mobileDetail.value = false;
}
async function openSearch(e: Event) {
  returnFocus = e.currentTarget as HTMLElement;
  search.value = "";
  searchItems.value = catalog.value;
  searchError.value = "";
  searchDialog.value?.showModal();
  await nextTick();
  searchInput.value?.focus();
}
function closeSearch() {
  clearTimeout(searchTimer);
  searchDialog.value?.close();
  searchController?.abort();
  searchSeq++;
  searchLoading.value = false;
}
watch(search, () => {
  clearTimeout(searchTimer);
  searchController?.abort();
  const seq = ++searchSeq;
  searchError.value = "";
  searchItems.value = search.value.trim()
    ? searchLocal(catalog.value, search.value)
    : catalog.value;
  if (!search.value.trim()) {
    searchLoading.value = false;
    return;
  }
  searchLoading.value = true;
  searchTimer = setTimeout(async () => {
    const controller = new AbortController();
    searchController = controller;
    try {
      const r = await jsonFetch(
        "/api/search?q=" + encodeURIComponent(search.value.trim()),
        { signal: controller.signal },
        15000,
      );
      if (seq === searchSeq) searchItems.value = r.items;
    } catch (e) {
      if (seq === searchSeq && !controller.signal.aborted)
        searchError.value = "网络搜索暂不可用，仍可添加下方内置标的";
    } finally {
      if (seq === searchSeq) searchLoading.value = false;
    }
  }, 300);
});
function add(i: Instrument) {
  if (prefs.items.some((v: Instrument) => v.id === i.id)) return;
  if (prefs.items.length >= 50) {
    notify("最多添加50个自选标的");
    return;
  }
  prefs.items.push({ ...i });
  if (!prefs.selected) prefs.selected = i.id;
  notify("已添加 " + i.symbol);
}
function openSettings(e: Event) {
  returnFocus = e.currentTarget as HTMLElement;
  settingsDialog.value?.showModal();
}
function exportPrefs() {
  const blob = new Blob(
      [JSON.stringify(validate(prefs, catalog.value, true), null, 2)],
      { type: "application/json" },
    ),
    url = URL.createObjectURL(blob),
    a = document.createElement("a");
  a.href = url;
  a.download = "market-watch-preferences.json";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
async function importPrefs(e: Event) {
  const input = e.target as HTMLInputElement,
    file = input.files?.[0];
  input.value = "";
  if (!file) return;
  try {
    if (file.size > 100000) throw Error("偏好文件过大");
    Object.assign(
      prefs,
      validate(JSON.parse(await file.text()), catalog.value, true),
    );
    notify("偏好已导入");
  } catch (e) {
    notify(e instanceof Error ? e.message : "偏好文件无效");
  }
}
function reset() {
  if (confirm("恢复默认自选？这会替换此设备的自选与显示偏好。"))
    Object.assign(prefs, defaults(catalog.value));
}
function setupTimer() {
  if (timer) clearInterval(timer);
  if (prefs.refresh)
    timer = setInterval(() => {
      if (!document.hidden) {
        loadQuotes();
        loadChart();
      }
    }, prefs.refresh * 1000);
}
function onVisible() {
  if (!document.hidden && prefs.refresh) {
    loadQuotes();
    loadChart();
  }
}
function outside(e: MouseEvent) {
  if (!(e.target as Element).closest(".row-actions")) menu.value = "";
}
async function boot() {
  fatal.value = "";
  try {
    catalog.value = await jsonFetch("/api/catalog");
    try {
      Object.assign(prefs, read(localStorage, catalog.value));
    } catch {
      Object.assign(prefs, defaults(catalog.value));
      storageFailed.value = true;
    }
    ready.value = true;
    applyTheme();
    loadQuotes();
    loadChart();
    setupTimer();
  } catch {
    fatal.value = "页面加载失败，请检查连接后重试";
  }
}
onMounted(() => {
  boot();
  media = matchMedia("(prefers-color-scheme:dark)");
  media.addEventListener("change", applyTheme);
  document.addEventListener("visibilitychange", onVisible);
  document.addEventListener("click", outside);
});
onUnmounted(() => {
  quoteController?.abort();
  chartController?.abort();
  searchController?.abort();
  clearInterval(timer);
  clearTimeout(searchTimer);
  clearTimeout(toastTimer);
  media?.removeEventListener("change", applyTheme);
  document.removeEventListener("visibilitychange", onVisible);
  document.removeEventListener("click", outside);
});
</script>
<template>
  <header class="topbar">
    <a class="brand" href="/" aria-label="观市首页"
      ><strong>观市</strong><span>Market Watch</span></a
    >
    <button class="global-search" @click="openSearch">
      <PhMagnifyingGlass :size="22" /><span>搜索名称、代码或交易所</span>
    </button>
    <div class="top-actions">
      <button
        class="icon-button"
        :aria-label="themeValue === 'dark' ? '切换浅色' : '切换深色'"
        @click="prefs.theme = themeValue === 'dark' ? 'light' : 'dark'"
      >
        <PhSun v-if="themeValue === 'dark'" :size="23" /><PhMoon
          v-else
          :size="23"
        /></button
      ><button class="icon-button" aria-label="设置" @click="openSettings">
        <PhGearSix :size="23" />
      </button>
    </div>
  </header>
  <main v-if="fatal" class="fatal">
    <PhWarningCircle :size="32" />
    <p>{{ fatal }}</p>
    <button @click="boot">重试</button>
  </main>
  <main v-else class="workspace" :class="{ 'show-detail': mobileDetail }">
    <aside class="watch-panel">
      <div class="watch-heading">
        <div>
          <h1>我的自选</h1>
          <p>仅保存在此设备</p>
        </div>
        <button
          class="add-button"
          @click="openSearch"
          :disabled="prefs.items.length >= 50"
        >
          <PhPlus :size="20" />添加
        </button>
      </div>
      <p v-if="storageFailed" class="warning storage-warning">
        <PhWarningCircle :size="16" />浏览器无法保存偏好，请导出以免丢失。
      </p>
      <nav class="tabs" aria-label="资产类型">
        <button
          v-for="[id, name] in tabs"
          :key="id"
          :class="{ active: filter === id }"
          :aria-pressed="filter === id"
          @click="filter = id"
        >
          {{ name }}
        </button>
      </nav>
      <div class="watch-list" :aria-busy="quoteLoading">
        <div v-if="!ready" class="loading">正在加载自选…</div>
        <article
          v-for="i in items"
          :key="i.id"
          class="watch-row"
          :class="{ selected: prefs.selected === i.id }"
          :data-id="i.id"
        >
          <button
            class="row-select"
            :aria-label="'查看 ' + i.symbol + ' ' + i.name"
            :aria-pressed="prefs.selected === i.id"
            @click="select(i)"
          >
            <AssetIcon :id="i.id" :kind="i.kind" /><span class="row-identity"
              ><strong>{{ i.symbol }}</strong
              ><small
                >{{ i.name }} · {{ i.market || quotes[i.id]?.market }}</small
              ></span
            >
            <PriceChart
              v-if="quotes[i.id]?.spark?.length > 1"
              :points="quotes[i.id].spark"
              spark
              :theme="themeValue"
              :color="
                quotes[i.id].percent == null
                  ? '#9298a4'
                  : quotes[i.id].percent! >= 0
                    ? prefs.colors === 'green-up'
                      ? '#56e1b5'
                      : '#fb756c'
                    : prefs.colors === 'green-up'
                      ? '#fb756c'
                      : '#56e1b5'
              "
            />
            <span v-else class="spark-empty" aria-hidden="true"></span>
            <span class="row-quote"
              ><strong>{{ number((quotes[i.id]?.price ?? 0) > 0 ? quotes[i.id]?.price : null) }}</strong
              ><small :class="direction(quotes[i.id]?.percent)">{{
                quotes[i.id]?.stale
                  ? "旧缓存"
                  : quotes[i.id]?.error
                    ? "暂不可用"
                    : signed(quotes[i.id]?.percent, "%")
              }}</small></span
            >
          </button>
          <div class="row-actions">
            <button
              class="row-menu-button"
              :aria-label="'管理 ' + i.symbol"
              :aria-expanded="menu === i.id"
              @click="menu = menu === i.id ? '' : i.id"
            >
              <PhDotsThree :size="20" />
            </button>
            <div v-if="menu === i.id" class="row-menu">
              <button
                :disabled="prefs.items[0]?.id === i.id"
                @click="move(i.id, -1)"
              >
                <PhArrowUp :size="16" />上移</button
              ><button
                :disabled="prefs.items.at(-1)?.id === i.id"
                @click="move(i.id, 1)"
              >
                <PhArrowDown :size="16" />下移</button
              ><button class="danger" @click="remove(i.id)">
                <PhTrash :size="16" />删除自选
              </button>
            </div>
          </div>
        </article>
        <div v-if="ready && !items.length" class="empty-list">
          <PhChartLine :size="32" />
          <p>
            {{ prefs.items.length ? "此分类还没有自选" : "添加第一个观察标的" }}
          </p>
          <button @click="openSearch"><PhPlus :size="18" />添加标的</button>
        </div>
      </div>
      <div class="list-footer">
        <span>{{ prefs.items.length }} 个自选</span
        ><button
          class="text-button"
          :disabled="quoteLoading"
          @click="loadQuotes"
        >
          <PhArrowClockwise :size="15" :class="{ spin: quoteLoading }" />{{
            quoteLoading ? "更新中" : "刷新"
          }}
        </button>
      </div>
      <p v-if="quoteError" class="error list-error" role="status">
        {{ quoteError }}
      </p>
    </aside>
    <section class="detail-panel" :aria-busy="chartLoading">
      <template v-if="selected">
        <button class="back-button text-button" @click="back">
          <PhArrowLeft :size="20" />我的自选
        </button>
        <div class="detail-top">
          <div class="detail-identity">
            <AssetIcon :id="selected.id" :kind="selected.kind" />
            <div>
              <h2 class="detail-title" tabindex="-1">{{ selected.name }}</h2>
              <p>
                {{ selected.symbol }} ·
                {{ q?.market || selected.market || "交易所待确认" }} ·
                {{ q?.currency || selected.currency || "—"
                }}<span v-if="q?.unit"> · {{ q.unit }}</span>
              </p>
            </div>
          </div>
          <time class="today">{{ dateLabel }}</time>
        </div>
        <div class="price-line">
          <strong class="big-price">{{ number((q?.price ?? 0) > 0 ? q?.price : null) }}</strong>
          <div class="price-changes" :class="direction(q?.percent)">
            <span>{{ signed(q?.change) }}</span
            ><span>{{ signed(q?.percent, "%") }}</span>
          </div>
        </div>
        <p class="quote-meta">
          <span>{{
            q?.state || (quoteLoading ? "正在获取行情…" : "行情暂不可用")
          }}</span
          ><span v-if="q?.asOf">
            · {{ selected.id.startsWith("c:") ? "获取于 " : ""
            }}{{ at(q.asOf, q.timezone) }} · {{ q.timezone || "UTC" }}</span
          ><span v-if="q?.basis"> · {{ q.basis }}</span>
        </p>
        <p class="quote-disclosure">
          {{
            selected.kind === "metal"
              ? "现货价格，不含实物溢价"
              : selected.kind === "crypto"
                ? "特定交易所报价，涨跌为24小时口径"
                : "公开参考行情，可能延迟"
          }}<span v-if="q?.currency === 'GBp'"> · GBp 为英镑便士</span>
        </p>
        <p
          v-if="q?.stale || q?.error"
          class="warning quote-warning"
          role="status"
        >
          <PhWarningCircle :size="17" />{{
            q?.stale ? "正在显示上次成功报价。" : ""
          }}{{ q?.error }}
        </p>
        <div class="chart-toolbar">
          <nav class="periods" aria-label="图表时间区间">
            <button
              v-for="[id, label] in periods"
              :key="id"
              :class="{ active: period === id }"
              :aria-pressed="period === id"
              @click="period = id"
            >
              {{ label }}
            </button>
          </nav>
          <button
            class="icon-button refresh-chart"
            aria-label="刷新图表"
            :disabled="chartLoading"
            @click="loadChart"
          >
            <PhArrowClockwise :size="18" :class="{ spin: chartLoading }" />
          </button>
        </div>
        <div class="chart-area">
          <div v-if="chartLoading" class="chart-empty" role="status">
            <span class="loader"></span>
            <p>正在读取历史行情…</p>
          </div>
          <div v-else-if="chartError" class="chart-empty" role="status">
            <PhChartLine :size="34" />
            <p>{{ chartError }}</p>
            <button @click="loadChart">重新读取</button>
          </div>
          <PriceChart
            v-else-if="chart?.points?.length"
            :points="chart.points"
            :theme="themeValue"
            :timezone="chart.timezone"
          />
        </div>
        <div class="chart-caption">
          <span
            >{{ chart?.source || "历史行情"
            }}<span v-if="chart?.stale" class="warning"> · 旧缓存</span></span
          ><span>图表时间按本设备显示 · 可缩放</span>
        </div>
        <dl class="stats">
          <div>
            <dt>{{ selected.kind === "crypto" ? "24h开盘" : "开盘" }}</dt>
            <dd>{{ number(q?.open) }}</dd>
          </div>
          <div>
            <dt>{{ selected.kind === "crypto" ? "24h最高" : "最高" }}</dt>
            <dd>{{ number(q?.high) }}</dd>
          </div>
          <div>
            <dt>{{ selected.kind === "crypto" ? "24h最低" : "最低" }}</dt>
            <dd>{{ number(q?.low) }}</dd>
          </div>
          <div>
            <dt>成交量<span v-if="selected.kind === 'crypto'">（币）</span><span v-else-if="/^y:[0-9]{6}\.(SS|SZ|BJ)$/.test(selected.id)">（手）</span></dt>
            <dd>{{ compact(q?.volume) }}</dd>
          </div>
          <div>
            <dt>市值</dt>
            <dd>{{ compact(q?.marketCap) }}</dd>
          </div>
          <div>
            <dt>{{ selected.kind === "crypto" ? "24h基准" : "昨收" }}</dt>
            <dd>{{ number(q?.previous) }}</dd>
          </div>
        </dl>
        <footer class="detail-footer">
          <div>
            <span>来源：</span
            ><a
              v-if="q?.sourceUrl"
              :href="q.sourceUrl"
              target="_blank"
              rel="noopener noreferrer"
              >{{ q.source }}</a
            ><span v-else>等待报价</span
            ><span v-if="q?.fetched"> · 获取 {{ at(q.fetched) }}</span>
          </div>
          <span>自选仅本机保存</span>
        </footer>
      </template>
      <div v-else class="no-selection">
        <PhChartLine :size="40" />
        <h2>观察，从一个标的开始</h2>
        <p>添加股票、加密货币或黄金，查看行情与走势。</p>
        <button class="primary" @click="openSearch">添加标的</button>
      </div>
    </section>
  </main>
  <footer class="app-footer">
    <span>行情供观察参考，非可成交报价。</span
    ><button class="text-button" @click="openSettings">
      <PhInfo :size="15" />关于与数据来源
    </button>
  </footer>
  <dialog
    ref="searchDialog"
    class="search-dialog"
    aria-labelledby="search-title"
    @click="$event.target === searchDialog && closeSearch()"
    @close="returnFocus?.focus()"
  >
    <header>
      <h2 id="search-title">添加观察标的</h2>
      <button class="icon-button" aria-label="关闭搜索" @click="closeSearch">
        <PhX :size="22" />
      </button>
    </header>
    <div class="search-field">
      <PhMagnifyingGlass :size="22" /><input
        ref="searchInput"
        v-model="search"
        maxlength="100"
        placeholder="搜索名称、代码或交易所"
        aria-label="搜索标的"
      />
    </div>
    <p class="search-hint">股票保留交易所与币种，加密保留交易对。</p>
    <p v-if="searchLoading" class="muted search-status" role="status">
      正在搜索全球市场…
    </p>
    <p v-if="searchError" class="warning search-status">{{ searchError }}</p>
    <ul class="search-results">
      <li v-for="i in searchItems" :key="i.id">
        <button
          :disabled="
            prefs.items.some((x: Instrument) => x.id === i.id) ||
            prefs.items.length >= 50
          "
          @click="add(i)"
        >
          <AssetIcon :id="i.id" :kind="i.kind" /><span
            ><strong
              >{{ i.symbol }} <small>{{ kinds[i.kind] }}</small></strong
            ><small
              >{{ i.name }} · {{ i.market
              }}<span v-if="i.currency"> · {{ i.currency }}</span></small
            ></span
          ><PhCheck
            v-if="prefs.items.some((x: Instrument) => x.id === i.id)"
            :size="21"
          /><PhPlus v-else :size="21" />
        </button>
      </li>
    </ul>
    <p v-if="!searchItems.length && !searchLoading" class="empty-list">
      没有找到标的，可尝试代码或英文名称。
    </p>
    <footer>
      <span>{{ prefs.items.length }} / 50 个自选</span
      ><button class="primary" @click="closeSearch">完成</button>
    </footer>
  </dialog>
  <dialog
    ref="settingsDialog"
    class="settings-dialog"
    aria-labelledby="settings-title"
    @click="$event.target === settingsDialog && settingsDialog?.close()"
    @close="returnFocus?.focus()"
  >
    <header>
      <h2 id="settings-title">此设备设置</h2>
      <button
        class="icon-button"
        aria-label="关闭设置"
        @click="settingsDialog?.close()"
      >
        <PhX :size="22" />
      </button>
    </header>
    <label class="setting-field"
      >外观<select v-model="prefs.theme" aria-label="外观">
        <option value="dark">深色</option>
        <option value="light">浅色</option>
        <option value="system">跟随系统</option>
      </select></label
    ><label class="setting-field"
      >涨跌配色<select v-model="prefs.colors" aria-label="涨跌配色">
        <option value="green-up">绿涨红跌</option>
        <option value="red-up">红涨绿跌</option>
      </select></label
    ><label class="setting-field"
      >自动刷新<select v-model="prefs.refresh" aria-label="自动刷新">
        <option :value="60">每分钟</option>
        <option :value="120">每2分钟</option>
        <option :value="300">每5分钟</option>
        <option :value="0">关闭，仅手动</option>
      </select></label
    >
    <p class="muted setting-note">
      后台标签页暂停刷新。刷新受行情来源更新频率、共享缓存与限流约束。
    </p>
    <div class="settings-buttons">
      <button @click="exportPrefs"><PhExport :size="18" />导出偏好</button
      ><button @click="importInput?.click()">
        <PhUpload :size="18" />导入偏好</button
      ><input
        ref="importInput"
        type="file"
        accept=".json,application/json"
        hidden
        @change="importPrefs"
      />
    </div>
    <button class="text-button danger" @click="reset">恢复默认自选</button>
    <section class="about">
      <h3>关于观市 · Market Watch 0.1.2</h3>
      <p>
        自选、排序和设置只保存在当前浏览器。导出文件只包含设备偏好，不包含密钥。
      </p>
      <p>
        A 股、境内指数及 ETF 使用东方财富公开接口（历史不复权、成交量为手）；其他股票/指数/ETF 使用 Yahoo Finance
        公开接口，可能限流、延迟或调整；同来源线路切换不等于独立报价验证。加密为
        Coinbase USD 交易对；现货黄金/白银为 Gold
        API，单位为美元/金衡盎司。来源暂不支持的数据以横线显示。
      </p>
      <p>
        现货历史需部署者配置
        GOLD_API_KEY，免费历史接口共享每小时10次额度。未配置时仍可看当前现货价，不用黄金期货冒充现货曲线。
      </p>
      <p>
        服务端只保留公开行情的临时内存缓存，不建账户或保存设备自选。公开或商业展示前须核实各数据源授权。
      </p>
      <a
        href="https://finance.yahoo.com/"
        target="_blank"
        rel="noopener noreferrer"
        >Yahoo Finance</a
      >
      ·
      <a
        href="https://docs.cdp.coinbase.com/exchange/introduction/welcome"
        target="_blank"
        rel="noopener noreferrer"
        >Coinbase</a
      >
      ·
      <a
        href="https://gold-api.com/docs"
        target="_blank"
        rel="noopener noreferrer"
        >Gold API</a
      >
    </section>
  </dialog>
  <div v-if="toast" class="toast" role="status">{{ toast }}</div>
</template>
