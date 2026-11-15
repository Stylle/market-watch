<script setup lang="ts">
import { computed } from "vue";
import {
  PhAppleLogo,
  PhCurrencyBtc,
  PhCurrencyEth,
  PhCoins,
  PhBuildings,
  PhChartLine,
} from "@phosphor-icons/vue";
const p = defineProps<{ id: string; kind?: string }>();
const icon = computed(() =>
  p.id === "y:AAPL"
    ? PhAppleLogo
    : p.id === "c:BTC-USD"
      ? PhCurrencyBtc
      : p.id === "c:ETH-USD"
        ? PhCurrencyEth
        : p.kind === "metal"
          ? PhCoins
          : p.kind === "index"
            ? PhChartLine
            : PhBuildings,
);
</script>
<template>
  <span
    class="asset-icon"
    :class="{
      bitcoin: id === 'c:BTC-USD',
      ethereum: id === 'c:ETH-USD',
      metal: kind === 'metal',
    }"
    ><img
      v-if="id === 'y:NVDA' || id === 'y:7203.T'"
      :src="id === 'y:NVDA' ? '/brands/nvidia.svg' : '/brands/toyota.svg'"
      alt=""
      width="30"
      height="30" />
    <component
      v-else
      :is="icon"
      :size="30"
      :weight="id === 'y:AAPL' ? 'fill' : 'regular'"
  /></span>
</template>
