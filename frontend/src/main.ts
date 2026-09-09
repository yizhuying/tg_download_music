import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import { useTheme } from './composables/useTheme'
import './style.css'

useTheme()

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('./views/ConfigView.vue') },
    { path: '/download', component: () => import('./views/DownloadView.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

const app = createApp(App)
app.use(router)
app.mount('#app')
