import {ref} from 'vue'
import {createDiscreteApi} from 'naive-ui'

export type WSMessageType = 'download_status' | 'log' | 'scan_result' | 'file_downloaded' | 'pong'

export interface WSMessage {
  type: WSMessageType
  payload: any
}

// Connection status for UI display:
//   connecting - establishing the connection
//   connected  - connection is open
//   retrying   - connection failed, waiting for the next retry
//   closed     - deliberately disconnected
export type WSStatus = 'connecting' | 'connected' | 'retrying' | 'closed'

export const wsStatus = ref<WSStatus>('closed')
export const wsRetryCount = ref(0)

const {message} = createDiscreteApi(['message'])

type Handler = (msg: WSMessage) => void

let ws: WebSocket | null = null
let handlers: Handler[] = []
let stateHandler: ((payload: any) => void) | null = null
let reconnectDelay = 5000
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let heartbeatTimer: ReturnType<typeof setInterval> | null = null
let lastMessageAt = 0

function stopTimers() {
  if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
  if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
}

// Close the socket without triggering the auto-reconnect in onclose.
function closeSocket() {
  stopTimers()
  if (ws) {
    ws.onclose = null
    ws.close()
    ws = null
  }
}

function open() {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${protocol}//${location.host}/api/ws`

  wsStatus.value = 'connecting'
  ws = new WebSocket(url)
  lastMessageAt = Date.now()

  ws.onopen = () => {
    wsStatus.value = 'connected'
    wsRetryCount.value = 0
    reconnectDelay = 5000
    message.success('WebSocket 连接成功', {duration: 2000})
    heartbeatTimer = setInterval(() => {
      if (!ws || ws.readyState !== WebSocket.OPEN) return
      // No traffic for too long means a dead link; close to force reconnect.
      if (Date.now() - lastMessageAt > 30000) {
        ws.close()
        return
      }
      ws.send(JSON.stringify({type: 'ping'}))
    }, 10000)
  }

  ws.onmessage = (event) => {
    lastMessageAt = Date.now()
    try {
      const msg: WSMessage = JSON.parse(event.data)
      if (msg.type === 'pong') return
      if (msg.type === 'download_status' && stateHandler) stateHandler(msg.payload)
      handlers.forEach(h => h(msg))
    } catch {}
  }

  ws.onclose = () => {
    stopTimers()
    wsStatus.value = 'retrying'
    wsRetryCount.value++
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      open()
    }, reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 1.5, 30000)
  }

  ws.onerror = () => {
    ws?.close()
  }
}

export function connect(onStateSnapshot?: (payload: any) => void) {
  closeSocket()
  handlers = []
  stateHandler = onStateSnapshot ?? null
  reconnectDelay = 5000
  wsRetryCount.value = 0
  open()
}

export function onMessage(handler: Handler) {
  handlers.push(handler)
}

export function disconnect() {
  closeSocket()
  handlers = []
  stateHandler = null
  wsStatus.value = 'closed'
}
