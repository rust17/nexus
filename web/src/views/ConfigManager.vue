<template>
  <div class="container">
    <!-- 配置管理部分 -->
    <div class="section">
      <h2>配置管理</h2>
      <el-tabs v-model="activeConfigTab">
        <el-tab-pane label="YAML 编辑器" name="yaml">
          <el-input
            type="textarea"
            :rows="15"
            placeholder="请输入 YAML 配置"
            v-model="yamlContent"
          />
        </el-tab-pane>
        <el-tab-pane label="表单视图" name="form">
          <div class="card">
            <el-form label-width="100px">
              <el-form-item label="服务名称">
                <el-input v-model="form.serviceName" />
              </el-form-item>
              <el-form-item label="端口">
                <el-input-number v-model="form.port" />
              </el-form-item>
            </el-form>
          </div>
        </el-tab-pane>
      </el-tabs>
      <el-button type="primary">保存配置</el-button>
      <el-button>版本对比</el-button>
    </div>

    <!-- 监控看板 -->
    <div class="section">
      <h2>监控看板</h2>
      <el-row :gutter="20">
        <el-col :span="8" v-for="(chart, index) in charts" :key="index">
          <div class="card">
            <h3>{{ chart.title }}</h3>
            <MonitorChart :option="chart.option" />
          </div>
        </el-col>
      </el-row>
    </div>

    <!-- 服务管理 -->
    <div class="section">
      <h2>服务管理</h2>
      <el-row :gutter="20">
        <el-col :span="6" v-for="(service, index) in services" :key="index">
          <ServiceCard :service="service" :index="index" />
        </el-col>
      </el-row>
    </div>

    <!-- 路由管理 -->
    <div class="section">
      <h2>路由管理</h2>
      <el-table :data="routes" style="width: 100%">
        <el-table-column prop="path" label="路径" />
        <el-table-column prop="service" label="服务" />
        <el-table-column label="操作">
          <template #default="scope">
            <el-button type="text" size="small">编辑</el-button>
            <el-button type="text" size="small" style="color: #F56C6C">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-button type="primary" style="margin-top: 20px;">添加路由规则</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useConfigStore } from '../stores/config'
import ServiceCard from '../components/ServiceCard.vue'
import MonitorChart from '../components/MonitorChart.vue'

const store = useConfigStore()
const { activeConfigTab, yamlContent, form, services, routes } = storeToRefs(store)

// 临时图表配置
const charts = ref([
  { title: '请求数', option: {/* ECharts 配置 */} },
  { title: '错误率', option: {/* ECharts 配置 */} },
  { title: 'CPU 使用率', option: {/* ECharts 配置 */} }
])
</script>

<style scoped>
.container {
  padding: 20px;
}
.section {
  margin-bottom: 30px;
}
.card {
  border: 1px solid #EBEEF5;
  border-radius: 4px;
  padding: 20px;
  margin: 10px 0;
}
</style>
