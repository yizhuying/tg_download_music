import {ref} from 'vue'

const THEME_KEY = 'fnos-theme-mode'

const THEME_LIGHT = '10'
const THEME_DARK = '20'
const THEME_SYSTEM = '30'

export const isDark = ref(false)

function getSystemDark(): boolean {
    return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function applyDark(dark: boolean) {
    isDark.value = dark
    document.documentElement.classList.toggle('dark', dark)
}

function resolveTheme(): boolean {
    const mode = localStorage.getItem(THEME_KEY)
    if (mode === THEME_DARK) return true
    if (mode === THEME_LIGHT) return false
    return getSystemDark()
}

function setTheme(mode: string) {
    const valid: Record<string, string> = {'10': 'light', '20': 'dark', '30': 'system'}
    if (!valid[mode]) {
        console.warn(`无效主题值，可选: 10=亮色, 20=暗色, 30=跟随系统`)
        return
    }
    localStorage.setItem(THEME_KEY, mode)
    applyDark(resolveTheme())
    console.log(`主题已切换为: ${valid[mode]}`)
}

export function useTheme() {
    applyDark(resolveTheme())

    // 方式1: URL 参数 ?theme=10|20|30
    const params = new URLSearchParams(window.location.search)
    const urlTheme = params.get('theme')
    if (urlTheme) {
        setTheme(urlTheme)
    }

    // 方式2: postMessage，主站调用 iframe.contentWindow.postMessage({type:'theme-change', mode:'20'})
    window.addEventListener('message', (e: MessageEvent) => {
        if (e.data?.type === 'theme-change' && e.data.mode) {
            setTheme(String(e.data.mode))
        }
    })

    // 方式3: 同源 localStorage 变化
    window.addEventListener('storage', (e) => {
        if (e.key === THEME_KEY) {
            applyDark(resolveTheme())
        }
    })

    // 方式4: 系统主题变化
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
        const mode = localStorage.getItem(THEME_KEY)
        if (mode === THEME_SYSTEM || !mode) {
            applyDark(getSystemDark())
        }
    })

    ;(window as any).setTheme = setTheme
}
