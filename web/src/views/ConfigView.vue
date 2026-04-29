<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  getConfig, saveConfig, getAuthStatus, sendCode, signIn, logout,
  setAdminPassword, getAdminPassword, startDownload,
} from '../api/http'
import { useRouter } from 'vue-router'

const router = useRouter()

const password = ref(getAdminPassword() || '')
const showLogin = ref(!getAdminPassword())
const loginError = ref('')

const apiId = ref(0)
const apiHash = ref('')
const sessionName = ref('my_session')
const proxyScheme = ref('socks5')
const proxyHostname = ref('')
const proxyPort = ref(0)
const proxyUsername = ref('')
const proxyPassword = ref('')
const sessionDir = ref('./sessions')
const downloadDir = ref('./downloads')
const channels = ref<string[]>([])

const authLoading = ref(true)
const authLoggedIn = ref(false)
const authPhone = ref('')

const showCodeSection = ref(false)
const authCode = ref('')
const show2FA = ref(false)
const auth2FA = ref('')

const toastMsg = ref('')

function showToast(msg: string) {
  toastMsg.value = msg
  setTimeout(() => { toastMsg.value = '' }, 3000)
}

async function doLogin() {
  if (!password.value) { loginError.value = '请输入密码'; return }
  setAdminPassword(password.value)
  try {
    const data = await getConfig()
    fillConfig(data)
    showLogin.value = false
    sessionStorage.setItem('tg_admin_pwd', password.value)
    await loadAuthStatus()
  } catch {
    loginError.value = '密码错误'
  }
}

function fillConfig(data: any) {
  apiId.value = data.api_id || 0
  apiHash.value = data.api_hash || ''
  sessionName.value = data.session_name || 'my_session'
  const p = data.proxy || {}
  proxyScheme.value = p.scheme || 'socks5'
  proxyHostname.value = p.hostname || ''
  proxyPort.value = p.port || 0
  proxyUsername.value = p.username || ''
  proxyPassword.value = p.password || ''
  sessionDir.value = data.session_dir || './sessions'
  downloadDir.value = data.download_dir || './downloads'
  channels.value = data.channels || []
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
  if (!authPhone.value) return
  try {
    await sendCode(authPhone.value)
    showCodeSection.value = true
    showToast('验证码已发送')
  } catch (e: any) {
    showToast(e.response?.data?.error || '发送失败')
  }
}

async function doSignIn() {
  if (!authCode.value) return
  try {
    const data = await signIn(authCode.value)
    if (data.need_2fa) {
      show2FA.value = true
    } else {
      showToast(data.message)
      await loadAuthStatus()
    }
  } catch (e: any) {
    showToast(e.response?.data?.error || '登录失败')
  }
}

async function doSignIn2FA() {
  if (!authCode.value || !auth2FA.value) return
  try {
    const data = await signIn(authCode.value, auth2FA.value)
    showToast(data.message)
    await loadAuthStatus()
  } catch (e: any) {
    showToast(e.response?.data?.error || '登录失败')
  }
}

async function doLogout() {
  if (!confirm('确定要退出登录吗？')) return
  try {
    await logout()
    showToast('已退出登录')
    authLoggedIn.value = false
    showCodeSection.value = false
    show2FA.value = false
  } catch (e: any) {
    showToast(e.response?.data?.error || '退出失败')
  }
}

async function doSaveConfig() {
  const data = {
    api_id: apiId.value,
    api_hash: apiHash.value,
    session_name: sessionName.value,
    proxy: {
      scheme: proxyScheme.value,
      hostname: proxyHostname.value,
      port: proxyPort.value,
      username: proxyUsername.value,
      password: proxyPassword.value,
    },
    channels: channels.value.filter(c => c.trim()),
    session_dir: sessionDir.value,
    download_dir: downloadDir.value,
  }
  try {
    const res = await saveConfig(data)
    showToast(res.message)
  } catch {
    showToast('保存失败')
  }
}

async function doStartDownload() {
  try {
    await startDownload()
    router.push('/download')
  } catch (e: any) {
    showToast(e.response?.data?.error || '启动失败')
  }
}

function addChannel() { channels.value.push('') }
function removeChannel(i: number) { channels.value.splice(i, 1) }

onMounted(() => {
  if (password.value) {
    getConfig().then(fillConfig).then(loadAuthStatus).catch(() => {
      password.value = ''
      showLogin.value = true
    })
  }
})
</script>

<template>
  <div>
    <div v-if="showLogin" class="login-overlay">
      <div class="login-card">
        <h2>管理登录</h2>
        <div class="form-group">
          <label>密码</label>
          <input type="password" v-model="password" placeholder="输入管理密码"
            @keydown.enter="doLogin" />
        </div>
        <div class="login-error">{{ loginError }}</div>
        <button class="btn btn-primary" style="width:100%" @click="doLogin">登录</button>
      </div>
    </div>

    <div v-if="!showLogin">
      <div class="card">
        <h2>Telegram 认证</h2>
        <div v-if="authLoading">检查中...</div>
        <div v-else-if="!authLoggedIn">
          <div class="form-row">
            <div class="form-group">
              <label>手机号</label>
              <input type="text" v-model="authPhone" placeholder="+8613800138000" />
            </div>
            <div class="form-group">
              <button class="btn btn-info" @click="doSendCode">发送验证码</button>
            </div>
          </div>
          <div v-if="showCodeSection">
            <div class="form-row">
              <div class="form-group">
                <label>验证码</label>
                <input type="text" v-model="authCode" placeholder="SMS 验证码" />
              </div>
              <div class="form-group">
                <button class="btn btn-success" @click="doSignIn">登录</button>
              </div>
            </div>
            <div v-if="show2FA">
              <div class="form-row">
                <div class="form-group">
                  <label>两步验证密码</label>
                  <input type="password" v-model="auth2FA" placeholder="2FA 密码" />
                </div>
                <div class="form-group">
                  <button class="btn btn-success" @click="doSignIn2FA">登录</button>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-else>
          <div class="status-bar">
            <span class="badge running">已登录</span>
            <span>{{ authPhone }}</span>
          </div>
          <button class="btn btn-danger btn-sm" @click="doLogout">退出登录</button>
        </div>
      </div>

      <div class="card">
        <h2>API 凭证</h2>
        <div class="form-group">
          <label>API ID</label>
          <input type="number" v-model.number="apiId" />
        </div>
        <div class="form-group">
          <label>API Hash</label>
          <input type="text" v-model="apiHash" />
        </div>
        <div class="form-group">
          <label>Session 名称</label>
          <input type="text" v-model="sessionName" />
        </div>
      </div>

      <div class="card">
        <h2>代理配置</h2>
        <div class="form-row">
          <div class="form-group">
            <label>代理类型</label>
            <select v-model="proxyScheme">
              <option value="socks5">SOCKS5</option>
              <option value="socks4">SOCKS4</option>
              <option value="http">HTTP</option>
            </select>
          </div>
          <div class="form-group">
            <label>主机地址</label>
            <input type="text" v-model="proxyHostname" placeholder="127.0.0.1" />
          </div>
          <div class="form-group">
            <label>端口</label>
            <input type="number" v-model.number="proxyPort" placeholder="1080" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>用户名</label>
            <input type="text" v-model="proxyUsername" placeholder="可选" />
          </div>
          <div class="form-group">
            <label>密码</label>
            <input type="password" v-model="proxyPassword" placeholder="可选" />
          </div>
        </div>
      </div>

      <div class="card">
        <h2>频道列表</h2>
        <div v-for="(ch, i) in channels" :key="i" class="channel-row">
          <input class="channel-input" v-model="channels[i]" placeholder="频道用户名" />
          <button class="btn btn-danger btn-sm" @click="removeChannel(i)">删除</button>
        </div>
        <button class="btn btn-secondary" @click="addChannel">+ 添加频道</button>
      </div>

      <div class="card">
        <h2>存储路径</h2>
        <div class="form-row">
          <div class="form-group">
            <label>Session 目录</label>
            <input type="text" v-model="sessionDir" />
          </div>
          <div class="form-group">
            <label>下载目录</label>
            <input type="text" v-model="downloadDir" />
          </div>
        </div>
      </div>

      <div class="form-actions">
        <button class="btn btn-primary" @click="doSaveConfig">保存配置</button>
        <button class="btn btn-success" @click="doStartDownload">开始下载</button>
      </div>
    </div>

    <div class="toast" :class="{ show: toastMsg }">{{ toastMsg }}</div>
  </div>
</template>
