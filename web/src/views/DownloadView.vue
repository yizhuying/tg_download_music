<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import {
  getDownloadStatus, startDownload, stopDownload, scanChannels,
  getDownloadList, downloadSingle, quickTest,
  setAdminPassword, getAdminPassword,
} from '../api/http'
import { connect, onMessage, disconnect } from '../api/ws'

const showLogin = ref(!getAdminPassword())
const loginPassword = ref('')
const loginError = ref('')

const running = ref(false)
const totalDownloaded = ref(0)
const currentChannel = ref('-')
const startedAt = ref('-')
const scannedAt = ref('-')
const logs = ref<{ time: string; message: string }[]>([])

interface ScannedFile {
  channel: string
  channel_title: string
  msg_id: number
  file_name: string
  file_size: number
  exists: boolean
}
const messages = ref<ScannedFile[]>([])
const searchInput = ref('')
const filterHideDownloaded = ref(false)

function doLogin() {
  if (!loginPassword.value) { loginError.value = '请输入密码'; return }
  setAdminPassword(loginPassword.value)
  getDownloadStatus()
    .then(initPage)
    .then(() => { showLogin.value = false; sessionStorage.setItem('tg_admin_pwd', loginPassword.value) })
    .catch(() => { loginError.value = '密码错误' })
}

function initPage(data: any) {
  running.value = data.running
  totalDownloaded.value = data.total_downloaded
  startedAt.value = data.started_at || '-'
  logs.value = data.logs || []

  connect((snapshot: any) => {
    running.value = snapshot.running
    totalDownloaded.value = snapshot.total_downloaded
    currentChannel.value = snapshot.current_channel || '-'
    startedAt.value = snapshot.started_at || '-'
    logs.value = snapshot.logs || []
  })

  onMessage((msg: any) => {
    if (msg.type === 'log') {
      logs.value.push(msg.payload)
    } else if (msg.type === 'download_status') {
      running.value = msg.payload.running
      totalDownloaded.value = msg.payload.total_downloaded
      currentChannel.value = msg.payload.current_channel || '-'
    } else if (msg.type === 'scan_result') {
      messages.value = msg.payload.messages || []
      scannedAt.value = msg.payload.scanned_at || '-'
    }
  })

  getDownloadList().then((data: any) => {
    messages.value = data.messages || []
    if (data.scanned_at) scannedAt.value = data.scanned_at
  })
}

async function doQuickTest() {
  await quickTest()
}

async function doScan() {
  await scanChannels()
}

async function doStop() {
  await stopDownload()
}

async function doStartAll() {
  await startDownload()
}

async function doDownloadSingle(ch: string, msgId: number) {
  await downloadSingle(ch, msgId)
  const d = await getDownloadList()
  messages.value = d.messages || []
}

function formatSize(bytes: number): string {
  if (!bytes) return '0B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0, s = bytes
  while (s >= 1024 && i < units.length - 1) { s /= 1024; i++ }
  return s.toFixed(1) + units[i]
}

const filteredMessages = ref<ScannedFile[]>([])

function applyFilters() {
  let result = messages.value
  if (filterHideDownloaded.value) result = result.filter(m => !m.exists)
  if (searchInput.value) {
    const term = searchInput.value.toLowerCase()
    result = result.filter(m => m.file_name.toLowerCase().includes(term))
  }
  filteredMessages.value = result
}

onMounted(async () => {
  if (getAdminPassword()) {
    getDownloadStatus().then(initPage).catch(() => { showLogin.value = true })
  }
})

onUnmounted(() => { disconnect() })
</script>

<template>
  <div>
    <div v-if="showLogin" class="login-overlay">
      <div class="login-card">
        <h2>管理登录</h2>
        <div class="form-group">
          <label>密码</label>
          <input type="password" v-model="loginPassword" placeholder="输入管理密码"
            @keydown.enter="doLogin" />
        </div>
        <div class="login-error">{{ loginError }}</div>
        <button class="btn btn-primary" style="width:100%" @click="doLogin">登录</button>
      </div>
    </div>

    <div v-if="!showLogin">
      <div class="card">
        <h2>下载控制</h2>
        <div class="form-actions" style="flex-wrap:wrap">
          <button class="btn btn-info" @click="doQuickTest">快速测试</button>
          <button class="btn btn-success" @click="doScan">扫描文件列表</button>
          <button class="btn btn-success" @click="doStartAll">全部下载</button>
        </div>
        <div style="font-size:0.8rem;color:#888;margin-top:8px">
          快速测试：自动下载第一个频道的第一条音频，用于验证功能
        </div>
      </div>

      <div class="card">
        <h2>下载状态</h2>
        <div class="status-bar">
          <span class="badge" :class="running ? 'running' : 'idle'">{{ running ? '运行中' : '空闲' }}</span>
          <span>{{ currentChannel }}</span>
        </div>
        <div class="status-details">
          <div><strong>已下载:</strong> {{ totalDownloaded }} 个文件</div>
          <div><strong>开始时间:</strong> {{ startedAt }}</div>
          <div><strong>扫描时间:</strong> {{ scannedAt }}</div>
        </div>
        <div class="form-actions">
          <button class="btn btn-danger" :disabled="!running" @click="doStop">停止下载</button>
        </div>
      </div>

      <div class="card">
        <h2>文件列表 <span>({{ filteredMessages.length }}/{{ messages.length }})</span></h2>
        <div class="filter-bar">
          <label><input type="checkbox" v-model="filterHideDownloaded" @change="applyFilters"> 隐藏已下载</label>
          <input type="text" v-model="searchInput" placeholder="搜索文件名..." @input="applyFilters" />
        </div>
        <div class="file-list">
          <div v-if="filteredMessages.length === 0" class="empty-tip">暂无文件，请先扫描</div>
          <div v-for="m in filteredMessages" :key="m.msg_id"
            class="file-item" :class="{ 'file-exists': m.exists }">
            <div class="file-info">
              <div class="file-name">{{ m.file_name }}</div>
              <div class="file-meta">{{ m.channel_title }} | {{ formatSize(m.file_size) }} {{ m.exists ? '[已下载]' : '' }}</div>
            </div>
            <button v-if="!m.exists" class="btn btn-sm btn-primary"
              @click="doDownloadSingle(m.channel, m.msg_id)">下载</button>
            <span v-else class="badge badge-done">已完成</span>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>下载日志</h2>
        <div class="logs-container">
          <div v-for="(l, i) in logs" :key="i" class="log-line">
            <span class="log-time">[{{ l.time }}] </span>{{ l.message }}
          </div>
        </div>
      </div>
    </div>

    <div class="toast" :class="{ show: toastMsg || loginError }">{{ toastMsg || loginError }}</div>
  </div>
</template>
