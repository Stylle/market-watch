<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch, nextTick } from "vue";
import * as echarts from "echarts/core";
import { LineChart, BarChart } from "echarts/charts";
import {
  GridComponent,
  TooltipComponent,
  DataZoomComponent,
} from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import type { Point } from "./types";
echarts.use([
  LineChart,
  BarChart,
  GridComponent,
  TooltipComponent,
  DataZoomComponent,
  CanvasRenderer,
]);
const props = defineProps<{
  points: Point[];
  spark?: boolean;
  color?: string;
  theme?: string;
  timezone?: string;
}>();
const root = ref<HTMLElement>();
let chart: echarts.ECharts | undefined;
let resize: ResizeObserver | undefined;
function draw() {
  if (!chart) return;
  const spark = !!props.spark,
    light = props.theme === "light",
    muted = light ? "#67717d" : "#a5aab4",
    line = light ? "#e2e6ec" : "#343a43",
    blue = props.color || "#3699ef";
  const volume = props.points.some((p) => p.volume != null);
  const rows = props.points.map((p) => [p.time * 1000, p.close]);
  const volumes = props.points.map((p) => [p.time * 1000, p.volume ?? null]);
  chart.setOption(
    {
      animation: false,
      backgroundColor: "transparent",
      textStyle: { fontFamily: "system-ui" },
      grid: spark
        ? { top: 4, right: 0, bottom: 4, left: 0 }
        : [
            { top: 20, right: 60, bottom: volume ? 130 : 45, left: 16 },
            { bottom: 35, height: 55, right: 60, left: 16 },
          ],
      tooltip: spark
        ? { show: false }
        : {
            trigger: "axis",
            confine: true,
            backgroundColor: light ? "#fff" : "#272b32",
            borderColor: line,
            textStyle: { color: light ? "#1a2029" : "#edf1f6" },
            axisPointer: { type: "line" },
            valueFormatter: (v: unknown) =>
              typeof v === "number"
                ? v.toLocaleString("en-US", { maximumFractionDigits: 4 })
                : String(v),
          },
      xAxis: spark
        ? { type: "time", show: false }
        : [
            {
              type: "time",
              show: !volume,
              axisLine: { lineStyle: { color: line } },
              axisLabel: { color: muted, hideOverlap: true },
            },
            {
              type: "time",
              gridIndex: 1,
              show: volume,
              axisLine: { lineStyle: { color: line } },
              axisLabel: { color: muted, hideOverlap: true },
            },
          ],
      yAxis: spark
        ? { type: "value", show: false, scale: true }
        : [
            {
              type: "value",
              position: "right",
              scale: true,
              splitNumber: 5,
              axisLabel: {
                color: muted,
                formatter: (v: number) =>
                  v.toLocaleString("en-US", { maximumFractionDigits: 2 }),
              },
              splitLine: { lineStyle: { color: line } },
            },
            { type: "value", gridIndex: 1, show: false },
          ],
      dataZoom: spark
        ? []
        : [
            {
              type: "inside",
              xAxisIndex: volume ? [0, 1] : [0],
              filterMode: "none",
            },
          ],
      series: [
        {
          type: "line",
          data: rows,
          showSymbol: false,
          lineStyle: { width: spark ? 2 : 2.5, color: blue },
          areaStyle: spark ? undefined : { color: blue, opacity: 0.12 },
          connectNulls: false,
        },
        ...(!spark && volume
          ? [
              {
                type: "bar",
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: volumes,
                itemStyle: { color: light ? "#9bacc6" : "#596d91" },
                barMaxWidth: 14,
              },
            ]
          : []),
      ],
    },
    true,
  );
}
watch(
  () => [props.points, props.color, props.theme],
  () => nextTick(draw),
  { deep: false },
);
onMounted(() => {
  if (root.value) {
    chart = echarts.init(root.value, undefined, { renderer: "canvas" });
    resize = new ResizeObserver(() => chart?.resize());
    resize.observe(root.value);
    draw();
  }
});
onUnmounted(() => {
  resize?.disconnect();
  chart?.dispose();
});
</script>
<template>
  <div
    ref="root"
    :class="spark ? 'spark-chart' : 'price-chart'"
    role="img"
    :aria-label="spark ? '价格走势' : '价格历史图表，可用鼠标滚轮或触控缩放'"
  />
</template>
