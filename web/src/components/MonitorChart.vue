<template>
  <div ref="chartRef" class="monitor-chart"></div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import * as echarts from 'echarts'

const props = defineProps<{
  option: echarts.EChartsOption
}>()

const chartRef = ref<HTMLElement>()
let chart: echarts.ECharts | null = null

onMounted(() => {
  if (chartRef.value) {
    chart = echarts.init(chartRef.value)
    chart.setOption(props.option)
  }
})

useResizeObserver(chartRef, () => {
  chart?.resize()
})
</script>
