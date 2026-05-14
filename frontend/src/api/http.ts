import axios from 'axios'

const http = axios.create({
    baseURL: '/api',
    timeout: 30000,
})

export function setAdminPassword(pwd: string) {
    http.defaults.headers.common['X-Admin-Password'] = pwd
}

export function getAdminPassword(): string {
    const pwd = http.defaults.headers.common['X-Admin-Password']
    return typeof pwd === 'string' ? pwd : ''
}

export function clearAdminPassword() {
    delete http.defaults.headers.common['X-Admin-Password']
}

export async function getConfig() {
    const res = await http.get('/config')
    return res.data
}

export async function saveConfig(data: any) {
    const res = await http.post('/config', data)
    return res.data
}

export async function getAuthStatus() {
    const res = await http.get('/auth/status')
    return res.data
}

export async function sendCode(phoneNumber: string) {
    const res = await http.post('/auth/send_code', {phone_number: phoneNumber})
    return res.data
}

export async function signIn(phoneCode: string, password?: string) {
    const res = await http.post('/auth/sign_in', {phone_code: phoneCode, password})
    return res.data
}

export async function logout() {
    const res = await http.post('/auth/logout')
    return res.data
}

export async function getDownloadStatus() {
    const res = await http.get('/download/status')
    return res.data
}

export async function startDownload() {
    const res = await http.post('/download/start')
    return res.data
}

export async function stopDownload() {
    const res = await http.post('/download/stop')
    return res.data
}

export async function scanChannels() {
    const res = await http.post('/download/scan')
    return res.data
}

export async function getDownloadList() {
    const res = await http.get('/download/list')
    return res.data
}

export async function downloadSingle(channel: string, msgId: number) {
    const res = await http.post('/download/single', {channel, msg_id: msgId})
    return res.data
}

export async function quickTest() {
    const res = await http.post('/download/quick_test')
    return res.data
}

export async function getDirs(path: string) {
    const res = await http.get('/download/dirs', {params: {path}})
    return res.data as { parent: string; dirs: { name: string; path: string }[] }
}

export async function getDownloadDirList() {
    const res = await http.get('/download/dirs')
    return res.data
}

export async function testProxy() {
    const res = await http.post('/config/proxy/test')
    return res.data
}