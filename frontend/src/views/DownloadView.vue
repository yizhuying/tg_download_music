<script setup lang="ts">
import {ref, computed, onMounted, onUnmounted} from 'vue'
import {
  getDownloadStatus, startDownload, stopDownload, scanChannels,
  getDownloadList, downloadSingle, quickTest,
  type DownloadStatus, type DownloadList,
} from '../api/http'
import {NInput, NButton, NCheckbox} from 'naive-ui'
import {onMessage, wsStatus, wsRetryCount, latestDownloadStatus} from '../api/ws'

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

// applyLiveStatus applies a live download_status payload. Incremental
// updates omit some fields; only apply them when present.
function applyLiveStatus(snapshot: DownloadStatus) {
  running.value = snapshot.running
  totalDownloaded.value = snapshot.total_downloaded ?? 0
  currentChannel.value = snapshot.current_channel || '-'
  if (snapshot.started_at) startedAt.value = snapshot.started_at
  if (Array.isArray(snapshot.logs)) logs.value = snapshot.logs
}

function handleWSMessage(msg: { type: string; payload: { time?: string; message?: string; running?: boolean; total_downloaded?: number; current_channel?: string; messages?: ScannedFile[]; scanned_at?: string; channel?: string; msg_id?: number } }) {
  if (msg.type === 'log') {
    logs.value.push({ time: msg.payload.time || '', message: msg.payload.message || '' })
    if (logs.value.length > 500) logs.value = logs.value.slice(-500)
  } else if (msg.type === 'download_status') {
    applyLiveStatus(msg.payload as DownloadStatus)
  } else if (msg.type === 'file_downloaded') {
    if (msg.payload.channel && msg.payload.msg_id) {
      const item = messages.value.find(m => m.channel === msg.payload.channel && m.msg_id === msg.payload.msg_id)
      if (item) item.exists = true
    }
  } else if (msg.type === 'scan_result') {
    messages.value = msg.payload.messages || []
    scannedAt.value = msg.payload.scanned_at || '-'
  }
}

function initPage(data: DownloadStatus) {
  running.value = data.running
  totalDownloaded.value = data.total_downloaded
  startedAt.value = data.started_at || '-'
  logs.value = data.logs || []

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

const wsStatusLabel = computed(() => {
  switch (wsStatus.value) {
    case 'connected': return '已连接'
    case 'connecting': return '连接中...'
    case 'retrying': return `连接失败，重试中 (${wsRetryCount.value})`
    default: return '未连接'
  }
})

const filteredMessages = computed(() => {
  let result = messages.value
  if (filterHideDownloaded.value) result = result.filter(m => !m.exists)
  if (searchInput.value) {
    const term = searchInput.value.toLowerCase()
    result = result.filter(m => m.file_name.toLowerCase().includes(term))
  }
  return result
})

let offMessages: (() => void) | null = null

onMounted(() => {
  offMessages = onMessage(handleWSMessage)
  // The app-level connection may already have a cached status; apply it
  // immediately so the page is instant while the REST refresh is pending.
  if (latestDownloadStatus.value) applyLiveStatus(latestDownloadStatus.value)
  getDownloadStatus()
    .then(initPage)
    .catch(() => initPage({running: false, total_downloaded: 0, current_channel: '', started_at: '', logs: []}))
})

onUnmounted(() => {
  offMessages?.()
  offMessages = null
})
</script>

<template>
  <div>
    <div>
      <div class="card">
        <h2>下载控制</h2>
        <div class="btn-group">
          <n-button type="info" size="small" @click="doQuickTest">快速测试</n-button>
          <n-button type="warning" size="small" @click="doScan">扫描文件列表</n-button>
          <n-button type="success" size="small" @click="doStartAll">全部下载</n-button>
        </div>
        <p class="hint-text">快速测试：自动下载第一个频道的第一条音频，用于验证功能</p>
      </div>

      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <h2>下载状态</h2>
          <span class="badge"
                :class="wsStatus === 'connected' ? 'ws-ok' : wsStatus === 'retrying' ? 'ws-bad' : 'ws-wait'">
            {{ wsStatusLabel }}
          </span>
        </div>
        <div class="status-row">
          <div class="status-info">
            <span class="badge" :class="running ? 'running' : 'idle'">{{ running ? '运行中' : '空闲' }}</span>
            <span style="font-size:0.78rem">{{ currentChannel }}</span>
          </div>
          <n-button type="error" size="small" :disabled="!running" @click="doStop">停止下载</n-button>
        </div>
        <p class="hint-text">已下载: {{ totalDownloaded }} 个 | 开始: {{ startedAt }} | 扫描: {{ scannedAt }}</p>
      </div>

      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:4px;border-bottom:1px solid #eee;padding-bottom:4px;margin-bottom:6px">
          <h2>文件列表 <span>({{ filteredMessages.length }}/{{ messages.length }})</span></h2>
          <div style="display:flex;gap:8px;align-items:center">
            <n-checkbox v-model:checked="filterHideDownloaded">隐藏已下载</n-checkbox>
            <n-input v-model:value="searchInput" placeholder="搜索文件名..." style="width:120px" />
          </div>
        </div>
        <div class="file-list">
          <div v-if="filteredMessages.length === 0" class="empty-tip">暂无文件，请先扫描</div>
          <div v-for="m in filteredMessages" :key="m.channel + '_' + m.msg_id"
            class="file-item" :class="{ 'file-exists': m.exists }">
            <div class="file-info">
              <div class="file-name">{{ m.file_name }}</div>
              <div class="file-meta">{{ m.channel_title }} | {{ formatSize(m.file_size) }} {{ m.exists ? '[已下载]' : '' }}</div>
            </div>
            <n-button v-if="!m.exists" size="small" type="primary"
              @click="doDownloadSingle(m.channel, m.msg_id)">下载</n-button>
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
