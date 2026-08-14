import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import ConfigView from './views/ConfigView.vue'
import DownloadView from './views/DownloadView.vue'
import { useTheme } from './composables/useTheme'
import './style.css'

useTheme()

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: ConfigView },
    { path: '/download', component: DownloadView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

const app = createApp(App)
app.use(router)
app.mount('#app')
