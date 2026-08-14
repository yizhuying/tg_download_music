<script setup lang="ts">
import {ref, onMounted} from 'vue'
import {
  getConfig, saveConfig, getAuthStatus, sendCode, signIn, logout,
  startDownload, getDownloadDirList, testProxy,
  type Config, type AuthStatus,
} from '../api/http'
import {useRouter} from 'vue-router'
import {useMessage, useDialog, NInput, NButton, NInputNumber, NSelect, NTag, NTimePicker, NCheckbox, NCheckboxGroup} from 'naive-ui'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()

const apiId = ref<number | undefined>()
const apiHash = ref('')
const sessionName = ref('my_session')
const proxyScheme = ref('none')
const proxyHostname = ref('')
const proxyPort = ref<number | undefined>()
const proxyUsername = ref('')
const proxyPassword = ref('')
const downloadDir = ref('')
const channels = ref<string[]>([])
const downloadTimeStart = ref<number | null>(null)
const downloadTimeEnd = ref<number | null>(null)
// 允许下载的音频格式；空数组表示全部
const audioFormats = ref<string[]>([])

const audioFormatOptions = [
  {label: 'MP3', value: 'mp3'},
  {label: 'FLAC', value: 'flac'},
  {label: 'OGG', value: 'ogg'},
  {label: 'M4A', value: 'm4a'},
  {label: 'WAV', value: 'wav'},
  {label: 'AAC', value: 'aac'},
]

function selectAllFormats() {
  audioFormats.value = []
}

const authLoading = ref(true)
const authLoggedIn = ref(false)
const authPhone = ref('')

const showCodeSection = ref(false)
const authCode = ref('')
const show2FA = ref(false)
const auth2FA = ref('')

const pickerDirs = ref<Array<{label: string; value: string}>>([])
const pickerLoading = ref(false)
// 是否已成功加载过目录列表；与结果是否为空区分开，
// 避免接口返回空列表时每次点开下拉都重新请求。
const dirsLoaded = ref(false)
const proxyLoading = ref(false)

const proxyOptions = [
  {label: '无', value: 'none'},
  {label: 'SOCKS5', value: 'socks5'},
]

function hhmmToTimestamp(hhmm: string): number | null {
  if (!hhmm) return null
  const [h, m] = hhmm.split(':').map(Number)
  if (isNaN(h) || isNaN(m)) return null
  const d = new Date()
  d.setHours(h, m, 0, 0)
  return d.getTime()
}

function timestampToHhmm(ts: number | null): string {
  if (ts == null) return ''
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

async function loadDirOptions() {
  pickerLoading.value = true
  try {
    const data = await getDownloadDirList()
    pickerDirs.value = Array.isArray(data) ? data : []
    dirsLoaded.value = true
  } catch {
    // 加载失败时保持 dirsLoaded 为 false，点开下拉可重试
  } finally {
    pickerLoading.value = false
  }
}

async function onDirSelectVisible(visible: boolean) {
  if (visible && !dirsLoaded.value) {
    await loadDirOptions()
  }
}

function showToast(msg: string) {
  message.success(msg, {duration: 2000})
}

function showError(msg: string) {
  message.error(msg, {duration: 3000})
}

function fillConfig(data: Config) {
  apiId.value = data.api_id || undefined
  apiHash.value = data.api_hash || ''
  sessionName.value = data.session_name || 'my_session'
  const p = data.proxy || {}
  proxyScheme.value = p.scheme || 'none'
  proxyHostname.value = p.hostname || ''
  proxyPort.value = p.port || undefined
  proxyUsername.value = p.username || ''
  proxyPassword.value = p.password || ''
  downloadDir.value = data.download_dir || ''
  channels.value = data.channels || []
  downloadTimeStart.value = hhmmToTimestamp(data.download_time_start || '')
  downloadTimeEnd.value = hhmmToTimestamp(data.download_time_end || '')
  audioFormats.value = (data.audio_formats || []).map(f => f.toLowerCase())
}

async function loadAuthStatus() {
  try {
    const data = await getAuthStatus()
    authLoggedIn.value = data.authorized
    authPhone.value = data.phone_number || ''
  } finally {
    authLoading.value = false
  }
}

async function doSendCode() {
  if (!authPhone.value) {
    showToast('请输入手机号')
    return
  }
  if (!(await doSaveConfig())) return
  try {
    await sendCode(authPhone.value)
    showCodeSection.value = true
    showToast('验证码已发送')
  } catch {
    // error shown by interceptor
  }
}

async function doSignIn() {
  if (!authCode.value) return
  try {
    const res = await signIn(authCode.value)
    if (res.data?.need_2fa) {
      show2FA.value = true
    } else {
      showToast(res.message || '登录成功')
      await loadAuthStatus()
    }
  } catch {
    // error shown by interceptor
  }
}

async function doSignIn2FA() {
  if (!authCode.value || !auth2FA.value) return
  try {
    await signIn(authCode.value, auth2FA.value)
    showToast('登录成功')
    await loadAuthStatus()
  } catch {
    // error shown by interceptor
  }
}

function doLogout() {
  dialog.warning({
    title: '确认',
    content: '确定要退出登录吗？',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await logout()
        showToast('已退出登录')
        authLoggedIn.value = false
        showCodeSection.value = false
        show2FA.value = false
        authCode.value = ''
        auth2FA.value = ''
      } catch {
        // error shown by interceptor
      }
    },
  })
}

async function doSaveConfig(): Promise<boolean> {
  if (!apiId.value || !apiHash.value) {
    showError('API ID 和 API Hash 为必填项')
    return false
  }
  if (proxyScheme.value !== 'none' && (!proxyHostname.value || !proxyPort.value)) {
    showError('代理主机和端口为必填项')
    return false
  }
  const data = {
    api_id: apiId.value,
    api_hash: apiHash.value,
    session_name: sessionName.value,
    proxy: {
      scheme: proxyScheme.value,
      hostname: proxyHostname.value,
      port: proxyPort.value || 0,
      username: proxyUsername.value,
      password: proxyPassword.value,
    },
    channels: channels.value.filter(c => c.trim()),
    download_dir: downloadDir.value,
    download_time_start: timestampToHhmm(downloadTimeStart.value),
    download_time_end: timestampToHhmm(downloadTimeEnd.value),
    audio_formats: audioFormats.value,
  }
  try {
    const res = await saveConfig(data)
    showToast(res.message || '配置已保存')
    return true
  } catch {
    // error shown by interceptor
    return false
  }
}

async function doTestProxy() {
  proxyLoading.value = true
  try {
    const res = await testProxy()
    if (res.data?.ok) {
      showToast(res.message || '代理可用')
    } else {
      showError(res.message || '代理不可用')
    }
  } catch {
    // error shown by interceptor
  } finally {
    proxyLoading.value = false
  }
}

async function doStartDownload() {
  try {
    await startDownload()
    await router.push('/download')
  } catch {
    // error shown by interceptor
  }
}

const showChannelInput = ref(false)
const newChannel = ref('')

function confirmAddChannel() {
  if (newChannel.value.trim()) {
    channels.value.push(newChannel.value.trim())
  }
  newChannel.value = ''
  showChannelInput.value = false
}

function removeChannel(i: number) {
  channels.value.splice(i, 1)
}

onMounted(() => {
  getConfig().then(fillConfig).then(loadAuthStatus).then(loadDirOptions).catch(() => {})
})
</script>

<template>
  <div>
    <div>
      <div class="card">
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <h2>API 凭证</h2>
          <n-button type="primary" @click="doSaveConfig">保存配置</n-button>
        </div>
        <div class="api_cert">
          <div class="form-group">
            <label>API ID <span class="required">*</span></label>
            <n-input-number v-model:value="apiId" placeholder="12345678" :show-button="false" style="width:100%" />
          </div>
          <div class="form-group">
            <label>API Hash <span class="required">*</span></label>
            <n-input v-model:value="apiHash" type="password" placeholder="API Hash" show-password-on="click" />
          </div>
          <!-- <div class="form-group">
            <label>Session 名称</label>
            <n-input v-model:value="sessionName" />
          </div> -->
        </div>
      </div>

      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <h2>代理配置</h2>
          <n-button v-if="proxyScheme !== 'none'" :loading="proxyLoading" :disabled="proxyLoading"
                    type="info" @click="doTestProxy">测试代理</n-button>
        </div>
        <div class="proxy_settings">
          <div class="form-group">
            <label>代理类型</label>
            <n-select v-model:value="proxyScheme" :options="proxyOptions" />
          </div>
          <div class="form-group">
            <label>主机地址 <span v-if="proxyScheme !== 'none'" class="required">*</span></label>
            <n-input v-model:value="proxyHostname" placeholder="127.0.0.1"
                     :disabled="proxyScheme === 'none'" />
          </div>
          <div class="form-group">
            <label>端口 <span v-if="proxyScheme !== 'none'" class="required">*</span></label>
            <n-input-number v-model:value="proxyPort" placeholder="1080" :show-button="false"
                            :disabled="proxyScheme === 'none'" style="width:100%" />
          </div>
          <div class="form-group">
            <label>用户名</label>
            <n-input v-model:value="proxyUsername" placeholder="可选"
                     :disabled="proxyScheme === 'none'" />
          </div>
          <div class="form-group">
            <label>密码</label>
            <n-input v-model:value="proxyPassword" type="password" placeholder="可选" show-password-on="click"
                     :disabled="proxyScheme === 'none'" />
          </div>
        </div>
      </div>

      <div class="card">
        <h2>频道列表</h2>
        <div class="channel-tags">
          <n-tag v-for="(ch, i) in channels" :key="i" closable @close="removeChannel(i)" size="small">
            {{ ch || '未填写' }}
          </n-tag>
          <n-input v-if="showChannelInput" v-model:value="newChannel" size="small" placeholder="输入后回车"
                   style="width: 120px" @keyup.enter="confirmAddChannel" @blur="confirmAddChannel" autofocus />
          <n-button v-else size="small" @click="showChannelInput = true">+ 添加</n-button>
        </div>
      </div>

      <div class="card">
        <h2>音频格式</h2>
        <p class="hint-text">勾选要扫描和下载的音频格式，都不勾选时下载全部格式</p>
        <div class="format-filter">
          <n-checkbox :checked="audioFormats.length === 0" @update:checked="selectAllFormats">全部</n-checkbox>
          <n-checkbox-group v-model:value="audioFormats">
            <n-checkbox v-for="opt in audioFormatOptions" :key="opt.value" :value="opt.value" :label="opt.label" />
          </n-checkbox-group>
        </div>
      </div>

      <div class="card">
        <h2 class="required">存储路径</h2>
        <n-select v-model:value="downloadDir" :options="pickerDirs" placeholder="选择下载目录"
                  :loading="pickerLoading" @update:show="onDirSelectVisible">
          <template #empty>
            <span style="font-size:0.8rem;color:#999">暂无可用目录</span>
          </template>
        </n-select>
      </div>

      <div class="card">
        <h2>下载时间</h2>
        <p style="font-size:0.78rem;color:#888;margin-bottom:8px">
          设置允许下载的时间范围，留空表示不限制。支持跨午夜（如 22:00 - 次日 06:00）。
        </p>
        <div class="time-range">
          <div class="time-input-wrapper">
            <label>开始时间</label>
            <n-time-picker v-model:value="downloadTimeStart" format="HH:mm" clearable
                           placeholder="不限制" size="small" />
          </div>
          <span class="time-separator">~</span>
          <div class="time-input-wrapper">
            <label>结束时间</label>
            <n-time-picker v-model:value="downloadTimeEnd" format="HH:mm" clearable
                           placeholder="不限制" size="small" />
          </div>
        </div>
      </div>

      <div class="card">
        <h2>Telegram 认证</h2>
        <div v-if="authLoading">检查中...</div>
        <div v-else-if="!authLoggedIn">
          <div class="form-row" style="align-items: flex-end; gap: 4px">
            <div class="form-group" style="flex: 1; min-width: 140px">
              <label>手机号</label>
              <n-input v-model:value="authPhone" placeholder="+86 13800138000" />
            </div>
            <div class="form-group" style="flex: none">
              <n-button type="primary" :disabled="!authPhone" @click="doSendCode">发送验证码</n-button>
            </div>
            <div class="form-group" v-if="showCodeSection" style="flex: none; width: 100px">
              <label>验证码</label>
              <n-input v-model:value="authCode" placeholder="验证码" />
            </div>
            <div class="form-group" v-if="showCodeSection" style="flex: none">
              <n-button type="primary" @click="doSignIn">登录</n-button>
            </div>
          </div>
          <div v-if="show2FA">
            <div class="form-row" style="align-items: flex-end; gap: 4px; margin-top: 2px">
              <div class="form-group" style="flex: 1; min-width: 160px">
                <label>两步验证密码</label>
                <n-input v-model:value="auth2FA" type="password" show-password-on="click" placeholder="2FA 密码" />
              </div>
              <div class="form-group" style="flex: none">
                <n-button type="primary" @click="doSignIn2FA">登录</n-button>
              </div>
            </div>
          </div>
        </div>
        <div v-else>
          <div class="status-row">
            <div class="status-info">
              <span class="badge running">已登录</span>
              <span>{{ authPhone }}</span>
            </div>
            <n-button type="error" size="small" @click="doLogout">退出登录</n-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
