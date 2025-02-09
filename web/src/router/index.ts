import { createRouter, createWebHistory } from 'vue-router'
import ConfigManager from '../views/ConfigManager.vue'

export default createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      component: ConfigManager
    }
  ]
})
