<script setup lang="ts">
import {useRoute, useRouter} from 'vue-router'

const route = useRoute()
const router = useRouter()
</script>

<template>
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
</template>
