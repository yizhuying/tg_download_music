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
  const valid: Record<string, string> = { '10': 'light', '20': 'dark', '30': 'system' }
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

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    const mode = localStorage.getItem(THEME_KEY)
    if (mode === THEME_SYSTEM || !mode) {
      applyDark(getSystemDark())
    }
  })

  window.addEventListener('storage', (e) => {
    if (e.key === THEME_KEY) {
      applyDark(resolveTheme())
    }
  })

  ;(window as any).setTheme = setTheme
}
