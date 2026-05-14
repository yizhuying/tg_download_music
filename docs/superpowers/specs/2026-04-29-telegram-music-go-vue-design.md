---
title: Telegram Music Download Go + Vue 3 Port Design
date: 2026-04-29
status: approved
---

# Telegram Music Download — Go + Vue 3 移植设计

## 概述

将现有 Python (Flask + Pyrogram) 的 Telegram 频道音乐下载器移植为 Go 后端 + Vue 3 前端。核心变更：Pyrogram → gotd/telegram，Flask → Gin，原生 JS/HTML → Vue 3 + Vite SPA，轮询 → WebSocket 实时推送。构建产物通过 `go:embed` 嵌入单一二进制。

## 架构

```
┌─────────────────────────────────────────────────┐
│                  单一 Go 二进制                   │
│  ┌───────────────────────────────────────────┐  │
│  │           Gin HTTP Server                  │  │
│  │  页面路由: GET /, /download → 嵌入前端     │  │
│  │  API 路由: /api/* → JSON                   │  │
│  │  静态文件: //go:embed web/dist/*           │  │
│  │  WebSocket: GET /api/ws                    │  │
│  └──────────────┬─────────────────────────────┘  │
│                 │                                │
│  ┌──────────────▼─────────────────────────────┐  │
│  │           Download Manager                  │  │
│  │  - goroutine 管理下载任务                   │  │
│  │  - sync.Mutex 保护共享状态                  │  │
│  │  - context 支持手动停止                     │  │
│  │  - 日志环形缓冲 (500 条)                    │  │
│  └──────────────┬─────────────────────────────┘  │
│                 │                                │
│  ┌──────────────▼─────────────────────────────┐  │
│  │        Telegram Client (gotd)               │  │
│  │  - MTProto 连接管理                         │  │
│  │  - 认证 (验证码 → 2FA)                      │  │
│  │  - 频道扫描 / 文件下载                      │  │
│  │  - 代理支持 (SOCKS5/HTTP)                   │  │
│  │  - CDN 下载 + 并发流                        │  │
│  └────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

## 文件结构

```
cmd/server/main.go          # 入口
internal/
  api/
    server.go               # Gin 路由注册
    ws.go                   # WebSocket Hub
    handler_config.go       # 配置 CRUD
    handler_auth.go         # Telegram 认证
    handler_download.go     # 下载控制
  telegram/
    client.go               # gotd Client 封装
    auth.go                 # 认证流程
    download.go             # 扫描和下载
    session.go              # session 持久化
  download/
    manager.go              # 下载任务调度
    state.go                # 下载状态
  config/
    config.go               # 配置加载/保存
web/                        # Vue 3 前端
  src/
    api/http.ts            # axios 封装
    api/ws.ts              # WebSocket 客户端
    views/
      ConfigView.vue        # 配置页
      DownloadView.vue      # 下载页
  vite.config.ts
Makefile
go.mod
```

## API 设计

### HTTP API（需 X-Admin-Password 认证头）

| Method | Path | 说明 |
|--------|------|------|
| GET | `/api/config` | 获取配置 |
| POST | `/api/config` | 保存配置 |
| GET | `/api/auth/status` | 认证状态 |
| POST | `/api/auth/send_code` | 发送验证码 |
| POST | `/api/auth/sign_in` | 完成登录 |
| POST | `/api/auth/logout` | 退出登录 |
| GET | `/api/download/status` | 下载状态 |
| POST | `/api/download/start` | 开始下载 |
| POST | `/api/download/stop` | 停止下载 |
| POST | `/api/download/scan` | 扫描频道 |
| GET | `/api/download/list` | 文件列表 |
| POST | `/api/download/single` | 单文件下载 |
| POST | `/api/download/quick_test` | 快速测试 |

### WebSocket (`/api/ws`)

连接建立后推送全量状态快照，后续增量推送：

| type | 说明 |
|------|------|
| `download_status` | running, total_downloaded, current_channel, started_at |
| `log` | 单条日志: { time, message } |
| `scan_result` | 扫描结果: { messages[], scanned_at } |

前端行为：断线 5 秒指数退避重连，重连后自动拉取全量快照。

## 数据模型

### Config

```go
type Config struct {
    APIID       int      `json:"api_id"`
    APIHash     string   `json:"api_hash"`
    SessionName string   `json:"session_name"`
    Channels    []string `json:"channels"`
    Proxy       Proxy    `json:"proxy"`
    DownloadDir string   `json:"download_dir"`
    SessionDir  string   `json:"session_dir"`
    PhoneNumber string   `json:"phone_number"`
}

type Proxy struct {
    Scheme   string `json:"scheme"`
    Hostname string `json:"hostname"`
    Port     int    `json:"port"`
    Username string `json:"username"`
    Password string `json:"password"`
}
```

### DownloadState

```go
type DownloadState struct {
    Running         bool       `json:"running"`
    TotalDownloaded int        `json:"total_downloaded"`
    CurrentChannel  string     `json:"current_channel"`
    StartedAt       string     `json:"started_at"`
    Logs            []LogEntry `json:"logs"`
}

type LogEntry struct {
    Time    string `json:"time"`
    Message string `json:"message"`
}
```

### ScannedMessage

```go
type ScannedMessage struct {
    Channel      string `json:"channel"`
    ChannelTitle string `json:"channel_title"`
    MsgID        int    `json:"msg_id"`
    FileName     string `json:"file_name"`
    FileSize     int64  `json:"file_size"`
    Exists       bool   `json:"exists"`
}
```

## 核心流程

### 认证流程
1. 用户输入手机号 → 后端调用 `client.SendCode` → 返回 phone_code_hash
2. 用户输入验证码 → 后端调用 `client.SignIn` → 成功则保存 session
3. 如需 2FA → 用户输入密码 → `client.CheckPassword`
4. Session 以 JSON 文件存储在 session_dir/

### 下载流程
1. 用户点击开始下载 → POST `/api/download/start`
2. Manager 启动独立 goroutine + context
3. 遍历频道，扫描音频消息 → 逐个下载
4. 每条日志通过 WebSocket 广播
5. 用户可 POST `/api/download/stop` 取消 context 停止

### 扫描流程
1. 用户点击扫描 → POST `/api/download/scan`
2. Manager 启动扫描 goroutine
3. 遍历频道消息历史，收集音频元信息
4. 扫描完成通过 WebSocket 推送 `scan_result`

## 错误处理

- `FloodWait` → 自动等待，推送等待日志
- 频道私有/不存在 → HTTP 400
- 认证失败 → HTTP 401
- WebSocket 断线 → 指数退避重连
- 下载中断 → context 取消，清理状态

## 构建流程

```bash
make build:
  1. cd web && npm install && npx vite build
  2. go build -o tg-music ./cmd/server

make dev:
  1. cd web && npm run dev  (Vite dev server, proxy /api → Go)
  2. go run ./cmd/server    (仅启动 API 服务)
```

Go 使用 `//go:embed web/dist/*` 嵌入前端产物到单一二进制。

## 依赖

Go: `gin`, `gorilla/websocket`, `gotd/telegram`
Frontend: `vue 3`, `axios`, `vue-router`, `typescript`
