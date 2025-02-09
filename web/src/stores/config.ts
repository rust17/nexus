import { defineStore } from 'pinia'

interface RouteItem {
  path: string
  service: string
}

interface ServiceItem {
  name: string
  status: '健康' | '异常'
  weight: number
  healthCheck: boolean
}

export const useConfigStore = defineStore('config', {
  state: () => ({
    activeConfigTab: 'yaml',
    yamlContent: '',
    form: {
      serviceName: 'example-service',
      port: 8080
    },
    services: [
      { name: 'Service A', status: '健康', weight: 50, healthCheck: true },
      { name: 'Service B', status: '异常', weight: 30, healthCheck: false }
    ] as ServiceItem[],
    routes: [
      { path: '/api/v1/users', service: 'UserService' },
      { path: '/api/v1/products', service: 'ProductService' }
    ] as RouteItem[]
  }),
  actions: {
    updateServiceWeight(index: number, value: number) {
      this.services[index].weight = value
    }
  }
})
