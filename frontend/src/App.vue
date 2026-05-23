<script setup lang="ts">
import {computed} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {NConfigProvider, NMessageProvider, NDialogProvider, darkTheme} from 'naive-ui'
import {isDark} from './composables/useTheme'

const route = useRoute()
const router = useRouter()

const theme = computed(() => isDark.value ? darkTheme : null)
</script>

<template>
  <n-config-provider :theme="theme">
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
