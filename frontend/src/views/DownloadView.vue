<script setup lang="ts">
import {ref, onMounted, onUnmounted} from 'vue'
import {
  getDownloadStatus, startDownload, stopDownload, scanChannels,
  getDownloadList, downloadSingle, quickTest,
  setAdminPassword, getAdminPassword,
  type DownloadStatus, type DownloadList,
} from '../api/http'
import {connect, onMessage, disconnect} from '../api/ws'

const savedPwd = getAdminPassword() || sessionStorage.getItem('tg_admin_pwd') || ''
if (savedPwd) setAdminPassword(savedPwd)
const showLogin = ref(!savedPwd)
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

function initPage(data: DownloadStatus) {
  running.value = data.running
  totalDownloaded.value = data.total_downloaded
  startedAt.value = data.started_at || '-'
  logs.value = data.logs || []

  connect((snapshot: DownloadStatus) => {
    running.value = snapshot.running
    totalDownloaded.value = snapshot.total_downloaded
    currentChannel.value = snapshot.current_channel || '-'
    startedAt.value = snapshot.started_at || '-'
    logs.value = snapshot.logs || []
  })

  onMessage((msg: { type: string; payload: { time?: string; message?: string; running?: boolean; total_downloaded?: number; current_channel?: string; messages?: ScannedFile[]; scanned_at?: string } }) => {
    if (msg.type === 'log') {
      logs.value.push({ time: msg.payload.time || '', message: msg.payload.message || '' })
    } else if (msg.type === 'download_status') {
      running.value = !!msg.payload.running
      totalDownloaded.value = msg.payload.total_downloaded ?? 0
      currentChannel.value = msg.payload.current_channel || '-'
    } else if (msg.type === 'scan_result') {
      messages.value = msg.payload.messages || []
      scannedAt.value = msg.payload.scanned_at || '-'
    }
  })

  getDownloadList().then((data: DownloadList) => {
    messages.value = data.messages || []
    if (data.scanned_at) scannedAt.value = data.scanned_at
  })
}

async function doQuickTest() {
  try {
    await quickTest()
    const data = await getDownloadStatus()
    running.value = data.running
  } catch {
    // error shown by interceptor
  }
}

async function doScan() {
  try {
    await scanChannels()
  } catch {
    // error shown by interceptor
  }
}

async function doStop() {
  await stopDownload()
  const data = await getDownloadStatus()
  running.value = data.running
}

async function doStartAll() {
  try {
    await startDownload()
    const data = await getDownloadStatus()
    running.value = data.running
  } catch {
    // error shown by interceptor
  }
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
  while (s >= 1024 && i < units.length - 1) {
    s /= 1024;
    i++
  }
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

onUnmounted(() => {
  disconnect()
})
</script>

<template>
  <div>
    <div v-if="showLogin" class="login-overlay">
      <div class="login-card">
        <h2>管理登录</h2>
        <div class="form-group">
          <label>密码</label>
          <el-input type="password" v-model="loginPassword" placeholder="输入管理密码"
            @keydown.enter="doLogin" />
        </div>
        <div class="login-error">{{ loginError }}</div>
        <el-button class="btn btn-primary" style="width:100%" @click="doLogin">登录</el-button>
      </div>
    </div>

    <div v-if="!showLogin">
      <div class="card">
        <h2>下载控制</h2>
        <div class="form-row" style="gap: 4px; align-items: flex-end">
          <div class="form-group" style="flex: none">
            <el-button type="info" @click="doQuickTest">快速测试</el-button>
          </div>
          <div class="form-group" style="flex: none">
            <el-button type="warning" @click="doScan">扫描文件列表</el-button>
          </div>
          <div class="form-group" style="flex: none">
            <el-button type="success" @click="doStartAll">全部下载</el-button>
          </div>
          <div class="form-group">
            <span style="font-size:0.75rem;color:#888">快速测试：自动下载第一个频道的第一条音频，用于验证功能</span>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>下载状态</h2>
        <div class="form-row" style="gap: 4px; align-items: flex-end">
          <div class="form-group" style="flex: none">
            <span class="badge" :class="running ? 'running' : 'idle'">{{ running ? '运行中' : '空闲' }}</span>
            <span style="font-size:0.78rem">{{ currentChannel }}</span>
          </div>
          <div class="form-group" style="flex: none">
            <el-button type="danger" :disabled="!running" @click="doStop">停止下载</el-button>
          </div>
          <div class="form-group">
            <span style="font-size:0.75rem;color:#888">已下载: {{ totalDownloaded }} 个 | 开始: {{ startedAt }} | 扫描: {{ scannedAt }}</span>
          </div>
        </div>
      </div>

      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #eee;padding-bottom:4px;margin-bottom:6px">
          <h2>文件列表 <span>({{ filteredMessages.length }}/{{ messages.length }})</span></h2>
          <div style="display:flex;gap:8px;align-items:center">
            <el-checkbox v-model="filterHideDownloaded" @change="applyFilters">隐藏已下载</el-checkbox>
            <el-input v-model="searchInput" placeholder="搜索文件名..." @input="applyFilters" style="width:150px"/>
          </div>
        </div>
        <div class="file-list">
          <div v-if="filteredMessages.length === 0" class="empty-tip">暂无文件，请先扫描</div>
          <div v-for="m in filteredMessages" :key="m.msg_id"
            class="file-item" :class="{ 'file-exists': m.exists }">
            <div class="file-info">
              <div class="file-name">{{ m.file_name }}</div>
              <div class="file-meta">{{ m.channel_title }} | {{ formatSize(m.file_size) }} {{ m.exists ? '[已下载]' : '' }}</div>
            </div>
            <el-button v-if="!m.exists" class="btn btn-sm btn-primary"
              @click="doDownloadSingle(m.channel, m.msg_id)">下载</el-button>
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
  </div>
</template>
