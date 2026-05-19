import axios, {type AxiosResponse} from 'axios'
import {ElMessage} from 'element-plus'

export interface ApiResponse<T = unknown> {
    code: number
    message?: string
    data?: T
}

export interface Config {
    api_id: number
    api_hash: string
    session_name: string
    channels: string[]
    proxy: {
        scheme: string
        hostname: string
        port: number
        username: string
        password: string
    }
    download_dir: string
    session_dir: string
    phone_number: string
}

export interface AuthStatus {
    authorized: boolean
    phone_number: string
    needs_2fa: boolean
}

export interface DownloadStatus {
    running: boolean
    total_downloaded: number
    current_channel: string
    started_at: string
    logs: { time: string; message: string }[]
    version: string
    build_time: string
}

export interface ScannedFile {
    channel: string
    channel_title: string
    msg_id: number
    file_name: string
    file_size: number
    exists: boolean
}

export interface DownloadList {
    messages: ScannedFile[]
    scanned_at: string
    running: boolean
}

export const http = axios.create({
    baseURL: '/api',
    timeout: 30000,
})

export function unwrap<T>(res: ApiResponse<T>): T {
    return res.data as T
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
http.interceptors.response.use(
    (res: AxiosResponse) => {
        const body = res.data as ApiResponse
        if (body.code !== undefined && body.code !== 0) {
            const msg = body.message || '请求失败'
            ElMessage({message: msg, type: 'error', duration: 3000})
            return Promise.reject(new Error(msg))
        }
        return body as any
    },
    (err) => {
        const msg = err.response?.data?.message || '网络错误'
        ElMessage({message: msg, type: 'error', duration: 3000})
        return Promise.reject(err)
    }
)