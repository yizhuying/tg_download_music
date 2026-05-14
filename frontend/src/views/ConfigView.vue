<script setup lang="ts">
import {ref, onMounted} from 'vue'
import {
  getConfig, saveConfig, getAuthStatus, sendCode, signIn, logout,
  setAdminPassword, getAdminPassword, startDownload, getDirs, getDownloadDirList, testProxy
} from '../api/http'
import {useRouter} from 'vue-router'
import {ElMessage, ElMessageBox} from 'element-plus'
import {Lock, Unlock} from '@element-plus/icons-vue'

const router = useRouter()

const password = ref(getAdminPassword() || '')
const showLogin = ref(!getAdminPassword())
const loginError = ref('')

const apiId = ref(0)
const apiHash = ref('')
const sessionName = ref('my_session')
const proxyScheme = ref('none')
const proxyHostname = ref('')
const proxyPort = ref(0)
const proxyUsername = ref('')
const proxyPassword = ref('')
const downloadDir = ref('./downloads')
const channels = ref<string[]>([])

const authLoading = ref(true)
const authLoggedIn = ref(false)
const authPhone = ref('')

const showCodeSection = ref(false)
const authCode = ref('')
const show2FA = ref(false)
const auth2FA = ref('')

const pickerDirs = ref<string[]>([])
const pickerLoading = ref(false)
const proxyLoading = ref(false)

async function loadDirOptions() {
  try {
    const data = await getDownloadDirList()
    pickerDirs.value = data.data || []
  } catch (e: any) {
    console.error('loadDirOptions error:', e)
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

function fillConfig(data: any) {
  apiId.value = data.api_id || 0
  apiHash.value = data.api_hash || ''
  sessionName.value = data.session_name || 'my_session'
  const p = data.proxy || {}
  proxyScheme.value = p.scheme || 'none'
  proxyHostname.value = p.hostname || ''
  proxyPort.value = p.port || 0
  proxyUsername.value = p.username || ''
  proxyPassword.value = p.password || ''
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
  if (!authPhone.value) {
    showToast('请输入手机号')
    return
  }
  await doSaveConfig()
  try {
    await sendCode(authPhone.value)
    showCodeSection.value = true
    showToast('验证码已发送')
  } catch (e: any) {
    showError(e.response?.data?.error || '发送失败')
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
    showError(e.response?.data?.error || '登录失败')
  }
}

async function doSignIn2FA() {
  if (!authCode.value || !auth2FA.value) return
  try {
    const data = await signIn(authCode.value, auth2FA.value)
    showToast(data.message)
    await loadAuthStatus()
  } catch (e: any) {
    showError(e.response?.data?.error || '登录失败')
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
  } catch (e: any) {
    showError(e.response?.data?.error || '退出失败')
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
    showToast(res.message)
  } catch {
    showToast('保存失败')
  }
}

async function doTestProxy() {
  if (proxyScheme.value === 'none') {
    showToast('代理未配置')
    return
  }
  proxyLoading.value = true
  try {
    const data = await testProxy()
    if (data.ok) {
      showToast(data.message)
    } else {
      showError(data.message)
    }
  } catch (e: any) {
    showError(e.response?.data?.error || '测试失败')
  } finally {
    proxyLoading.value = false
  }
}

async function doStartDownload() {
  try {
    await startDownload()
    await router.push('/download')
  } catch (e: any) {
    showToast(e.response?.data?.error || '启动失败')
  }
}

const showChannelInput = ref(false)
const newChannel = ref('')

function addChannel() {
  channels.value.push('')
}

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
          <div class="form-actions">
            <el-button type="primary" @click="doSaveConfig">保存配置</el-button>
          </div>
        </div>
        <div class="api_cert">
          <div class="form-group">
            <label>API ID <span class="required">*</span></label>
            <el-input type="text" v-model.number="apiId"/>
          </div>
          <div class="form-group">
            <label>API Hash <span class="required">*</span></label>
            <el-input v-model="apiHash" type="password" show-password>
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
          <el-button :loading="proxyLoading" :disabled="proxyLoading"
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
        <h2>存储路径</h2>
        <el-select v-model="downloadDir" placeholder="选择下载目录" :loading="pickerLoading"
                   style="width: 100%" @visible-change="onDirSelectVisible">
          <el-option v-for="p in pickerDirs" :key="p" :label="p" :value="p"/>
        </el-select>
      </div>

      <div class="card">
        <h2>Telegram 认证</h2>
        <div v-if="authLoading">检查中...</div>
        <div v-else-if="!authLoggedIn">
          <div class="form-row" style="align-items: flex-end; gap: 4px">
            <div class="form-group" style="flex: none">
              <label>手机号</label>
              <el-input style="width: 140px" type="text" v-model="authPhone" placeholder="+8613800138000"/>
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
