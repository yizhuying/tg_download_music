import {http, unwrap, type ApiResponse, type Config, type AuthStatus, type DownloadStatus, type DownloadList} from './response'

export type {Config, AuthStatus, DownloadStatus, DownloadList}
export type {ScannedFile} from './response'

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

export async function getConfig(): Promise<Config> {
    const res = await http.get<any, ApiResponse<Config>>('/config')
    return unwrap(res)
}

export async function saveConfig(data: Partial<Config>): Promise<ApiResponse> {
    return http.post('/config', data)
}

export async function getAuthStatus(): Promise<AuthStatus> {
    const res = await http.get<any, ApiResponse<AuthStatus>>('/auth/status')
    return unwrap(res)
}

export async function sendCode(phoneNumber: string): Promise<ApiResponse> {
    return http.post('/auth/send_code', {phone_number: phoneNumber})
}

export async function signIn(phoneCode: string, password?: string): Promise<ApiResponse<{ need_2fa?: boolean }>> {
    return http.post('/auth/sign_in', {phone_code: phoneCode, password})
}

export async function logout(): Promise<ApiResponse> {
    return http.post('/auth/logout')
}

export async function getDownloadStatus(): Promise<DownloadStatus> {
    const res = await http.get<any, ApiResponse<DownloadStatus>>('/task/status')
    return unwrap(res)
}

export async function startDownload(): Promise<ApiResponse> {
    return http.post('/task/start')
}

export async function stopDownload(): Promise<ApiResponse> {
    return http.post('/task/stop')
}

export async function scanChannels(): Promise<ApiResponse> {
    return http.post('/task/scan')
}

export async function getDownloadList(): Promise<DownloadList> {
    const res = await http.get<any, ApiResponse<DownloadList>>('/task/list')
    return unwrap(res)
}

export async function downloadSingle(channel: string, msgId: number): Promise<ApiResponse> {
    return http.post('/task/single', {channel, msg_id: msgId})
}

export async function quickTest(): Promise<ApiResponse> {
    return http.post('/task/quick_test')
}

export async function getDirs(path: string): Promise<string[]> {
    const res = await http.get<any, ApiResponse<string[]>>('/task/dirs', {params: {path}})
    return res.data ?? []
}

export async function getDownloadDirList(): Promise<Array<{label: string; value: string}>> {
    const res = await http.get<any, ApiResponse<Array<{label: string; value: string}>>>('/task/dirs')
    return res.data ?? []
}

export async function testProxy(): Promise<ApiResponse<{ ok: boolean }>> {
    return http.post('/config/proxy/test')
}
