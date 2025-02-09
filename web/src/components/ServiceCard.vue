<template>
  <div class="card">
    <h3>{{ service.name }}</h3>
    <p>状态:
      <el-tag :type="service.status === '健康' ? 'success' : 'danger'">
        {{ service.status }}
      </el-tag>
    </p>
    <el-slider v-model="service.weight" @change="updateWeight" />
    <el-switch
      v-model="service.healthCheck"
      active-text="启用健康检查"
      inactive-text="关闭健康检查"
    />
  </div>
</template>

<script setup lang="ts">
import { useConfigStore } from '../stores/config'

defineProps<{
  service: ServiceItem
  index: number
}>()

const store = useConfigStore()
const updateWeight = (value: number) => {
  store.updateServiceWeight(index, value)
}
</script>
