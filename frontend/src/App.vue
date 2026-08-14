<script setup lang="ts">
import {computed, onMounted} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {NConfigProvider, NMessageProvider, NDialogProvider, darkTheme, zhCN, dateZhCN} from 'naive-ui'
import {isDark} from './composables/useTheme'
import {connect} from './api/ws'

const route = useRoute()
const router = useRouter()

const theme = computed(() => isDark.value ? darkTheme : null)

// The WebSocket connection is app-level: established once on startup and
// reused across route changes; views only subscribe to messages.
onMounted(() => {
  connect()
})
</script>

<template>
  <n-config-provider :theme="theme" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <div class="container">
          <header>
            <h1>Telegram 频道音乐下载</h1>
            <nav>
              <a href="/" :class="{ active: route.path === '/' }" @click.prevent="router.push('/')">配置管理</a>
              <a href="/download" :class="{ active: route.path === '/download' }" @click.prevent="router.push('/download')">下载状态</a>
            </nav>
          </header>
          <router-view/>
        </div>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
