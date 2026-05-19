<script setup lang="ts">
import {ref, onMounted} from 'vue'
import {
  getConfig, saveConfig, getAuthStatus, sendCode, signIn, logout,
  setAdminPassword, getAdminPassword, clearAdminPassword, startDownload, getDownloadDirList, testProxy,
  type Config, type AuthStatus,
} from '../api/http'
import {useRouter} from 'vue-router'
import {ElMessage, ElMessageBox} from 'element-plus'
import {Lock, Unlock} from '@element-plus/icons-vue'

const router = useRouter()

const savedPwd = getAdminPassword() || sessionStorage.getItem('tg_admin_pwd') || ''
if (savedPwd) setAdminPassword(savedPwd)
const password = ref(savedPwd)
const showLogin = ref(!savedPwd)
const loginError = ref('')

const apiId = ref()
const apiHash = ref('')
const sessionName = ref('my_session')
const proxyScheme = ref('none')
const proxyHostname = ref('')
const proxyPort = ref(0)
const proxyUsername = ref('')
const proxyPassword = ref('')
const downloadDir = ref('')
const channels = ref<string[]>([])

const authLoading = ref(true)
const authLoggedIn = ref(false)
const authPhone = ref('')

const showCodeSection = ref(false)
const authCode = ref('')
const show2FA = ref(false)
const auth2FA = ref('')

const pickerDirs = ref<Array<{label: string; value: string}>>([])
const pickerLoading = ref(false)
const proxyLoading = ref(false)

async function loadDirOptions() {
  try {
    const data = await getDownloadDirList()
    pickerDirs.value = Array.isArray(data) ? data : []
  } catch {
    // ignore
  }
}

async function onDirSelectVisible(visible: boolean) {
  if (visible && pickerDirs.value.length === 0) {
    await loadDirOptions()
  }
}

function showToast(msg: string) {
  ElMessage({message: msg, type: 'success', duration: 2000})
}

function showError(msg: string) {
  ElMessage({message: msg, type: 'error', duration: 3000})
}

async function doLogin() {
  if (!password.value) {
    loginError.value = '请输入密码'
    return
  }
  setAdminPassword(password.value)
  try {
    const data = await getConfig()
    fillConfig(data)
    showLogin.value = false
    sessionStorage.setItem('tg_admin_pwd', password.value)
    await loadAuthStatus()
    await loadDirOptions()
  } catch {
    loginError.value = '密码错误'
  }
}

function fillConfig(data: Config) {
  apiId.value = data.api_id || 0
  apiHash.value = data.api_hash || ''
  sessionName.value = data.session_name || 'my_session'
  const p = data.proxy || {}
  proxyScheme.value = p.scheme || 'none'
  proxyHostname.value = p.hostname || ''
  proxyPort.value = p.port || 0
  proxyUsername.value = p.username || ''
  proxyPassword.value = p.password || ''
  downloadDir.value = data.download_dir || ''
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
  if (!authPhone.value) {
    showToast('请输入手机号')
    return
  }
  await doSaveConfig()
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

async function doLogout() {
  try {
    await ElMessageBox.confirm('确定要退出登录吗？', '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await logout()
    showToast('已退出登录')
    authLoggedIn.value = false
    showCodeSection.value = false
    show2FA.value = false
  } catch {
    // error shown by interceptor
  }
}

async function doSaveConfig() {
  if (!apiId.value || !apiHash.value) {
    showError('API ID 和 API Hash 为必填项')
    return
  }
  if (proxyScheme.value !== 'none' && (!proxyHostname.value || !proxyPort.value)) {
    showError('代理主机和端口为必填项')
    return
  }
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
    download_dir: downloadDir.value,
  }
  try {
    const res = await saveConfig(data)
    showToast(res.message || '配置已保存')
  } catch {
    // error shown by interceptor
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
  if (getAdminPassword()) {
    getConfig().then(fillConfig).then(loadAuthStatus).then(loadDirOptions).catch(() => {
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
          <el-input v-model="password" type="password" show-password
                    placeholder="输入管理密码" @keydown.enter="doLogin">
            <template #password-icon="{ visible }">
              <el-icon :size="16">
                <Unlock v-if="visible"/>
                <Lock v-else/>
              </el-icon>
            </template>
          </el-input>
        </div>
        <div class="login-error">{{ loginError }}</div>
        <el-button type="primary" style="width:50%" @click="doLogin">登录</el-button>
      </div>
    </div>

    <div v-if="!showLogin">
      <div class="card">
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <h2>API 凭证</h2>
          <el-button type="primary" @click="doSaveConfig">保存配置</el-button>
        </div>
        <div class="api_cert">
          <div class="form-group">
            <label>API ID <span class="required">*</span></label>
            <el-input type="text" v-model.number="apiId" placeholder="12345678"/>
          </div>
          <div class="form-group">
            <label>API Hash <span class="required">*</span></label>
            <el-input v-model="apiHash" type="password" placeholder="API Hash" show-password>
              <template #password-icon="{ visible }">
                <el-icon :size="16">
                  <Unlock v-if="visible"/>
                  <Lock v-else/>
                </el-icon>
              </template>
            </el-input>
          </div>
          <div class="form-group">
            <label>Session 名称</label>
            <el-input type="text" v-model="sessionName"/>
          </div>
        </div>
      </div>

      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <h2>代理配置</h2>
          <el-button v-if="proxyScheme !== 'none'" :loading="proxyLoading" :disabled="proxyLoading"
                     style="background-color:#76BCF0FF;color:#fff"
                     @click="doTestProxy">测试代理
          </el-button>
        </div>
        <div class="proxy_settings">
          <div class="form-group">
            <label>代理类型</label>
            <el-select v-model="proxyScheme" style="width:100%">
              <el-option value="none">无</el-option>
              <el-option value="socks5">SOCKS5</el-option>
            </el-select>
          </div>
          <div class="form-group">
            <label>主机地址 <span v-if="proxyScheme !== 'none'" class="required">*</span></label>
            <el-input type="text" v-model="proxyHostname" placeholder="127.0.0.1"
                      :disabled="proxyScheme === 'none'"/>
          </div>
          <div class="form-group">
            <label>端口 <span v-if="proxyScheme !== 'none'" class="required">*</span></label>
            <el-input type="text" v-model.number="proxyPort" placeholder="1080"
                      :disabled="proxyScheme === 'none'"/>
          </div>
          <div class="form-group">
            <label>用户名</label>
            <el-input type="text" v-model="proxyUsername" placeholder="可选"
                      :disabled="proxyScheme === 'none'"/>
          </div>
          <div class="form-group">
            <label>密码</label>
            <el-input type="password" v-model="proxyPassword" placeholder="可选" show-password
                      :disabled="proxyScheme === 'none'">
              <template #password-icon="{ visible }">
                <el-icon :size="16">
                  <Unlock v-if="visible"/>
                  <Lock v-else/>
                </el-icon>
              </template>
            </el-input>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>频道列表</h2>
        <div class="channel-tags">
          <el-tag v-for="(ch, i) in channels" :key="i" closable @close="removeChannel(i)" class="channel-tag">
            {{ ch || '未填写' }}
          </el-tag>
          <el-input v-if="showChannelInput" v-model="newChannel" size="small" placeholder="输入后回车"
                    style="width: 120px" @keyup.enter="confirmAddChannel" @blur="confirmAddChannel" autofocus/>
          <el-button v-else size="small" @click="showChannelInput = true">+ 添加</el-button>
        </div>
      </div>

      <div class="card">
        <h2 class="required">存储路径</h2>
        <el-select v-model="downloadDir" placeholder="选择下载目录" :loading="pickerLoading"
                   style="width: 100%" @visible-change="onDirSelectVisible">
          <el-option v-for="d in pickerDirs" :key="d.value" :label="d.label" :value="d.value"/>
        </el-select>
      </div>

      <div class="card">
        <h2>Telegram 认证</h2>
        <div v-if="authLoading">检查中...</div>
        <div v-else-if="!authLoggedIn">
          <div class="form-row" style="align-items: flex-end; gap: 4px">
            <div class="form-group" style="flex: none">
              <label>手机号</label>
              <el-input style="width: 140px" type="text" v-model="authPhone" placeholder="+86 13800138000"/>
            </div>
            <div class="form-group" style="flex: none">
              <el-button type="primary" :disabled="!authPhone" @click="doSendCode">发送验证码</el-button>
            </div>
            <div class="form-group" v-if="showCodeSection" style="flex: none">
              <label>验证码</label>
              <el-input style="width: 100px" type="text" v-model="authCode" placeholder="验证码"/>
            </div>
            <div class="form-group" v-if="showCodeSection" style="flex: none">
              <el-button type="primary" @click="doSignIn">登录</el-button>
            </div>
          </div>
          <div v-if="show2FA">
            <div class="form-row" style="align-items: flex-end; gap: 4px; margin-top: 2px">
              <div class="form-group" style="flex: none">
                <label>两步验证密码</label>
                <el-input v-model="auth2FA" type="password" show-password style="width: 200px" placeholder="2FA 密码">
                  <template #password-icon="{ visible }">
                    <el-icon :size="16">
                      <Unlock v-if="visible"/>
                      <Lock v-else/>
                    </el-icon>
                  </template>
                </el-input>
              </div>
              <div class="form-group" style="flex: none">
                <el-button type="primary" @click="doSignIn2FA">登录</el-button>
              </div>
            </div>
          </div>
        </div>
        <div v-else>
          <div class="status-bar">
            <span class="badge running">已登录</span>
            <span>{{ authPhone }}</span>
            <el-button type="danger" size="small" @click="doLogout">退出登录</el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
