export type WSMessageType = 'download_status' | 'log' | 'scan_result'

export interface WSMessage {
  type: WSMessageType
  payload: any
}

type Handler = (msg: WSMessage) => void

let ws: WebSocket | null = null
let handlers: Handler[] = []
let reconnectDelay = 5000
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

let heartbeatTimer: ReturnType<typeof setInterval> | null = null

export function connect(onStateSnapshot?: (payload: any) => void) {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${protocol}//${location.host}/api/ws`

  ws = new WebSocket(url)

  ws.onopen = () => {
    reconnectDelay = 5000
    if (heartbeatTimer) clearInterval(heartbeatTimer)
    heartbeatTimer = setInterval(() => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({type: 'ping'}))
      }
    }, 10000)
  }

  ws.onmessage = (event) => {
    try {
      const msg: WSMessage = JSON.parse(event.data)
      if (msg.type === 'download_status' && onStateSnapshot) {
        onStateSnapshot(msg.payload)
      }
      handlers.forEach(h => h(msg))
    } catch {}
  }

  ws.onclose = () => {
    reconnectTimer = setTimeout(() => connect(onStateSnapshot), reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 1.5, 30000)
  }

  ws.onerror = () => {
    ws?.close()
  }
}

export function onMessage(handler: Handler) {
  handlers.push(handler)
}

export function disconnect() {
  if (reconnectTimer) clearTimeout(reconnectTimer)
  if (heartbeatTimer) clearInterval(heartbeatTimer)
  ws?.close()
  handlers = []
}
