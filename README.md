# TG Music Manager

从 Telegram 频道批量下载音乐文件的 Web 管理工具。

## 技术栈

| 层  | 技术                                       |
|----|------------------------------------------|
| 后端 | Go 1.25 + Gin + gotd/td (MTProto)        |
| 前端 | Vue 3 + TypeScript + Vite + Naive UI    |
| 通信 | REST API + WebSocket 实时推送                |

## 项目结构

```
server/
  cmd/server/main.go        # 入口，嵌入前端静态资源
  internal/
    api/                    # HTTP 路由、WebSocket、中间件
      server.go             # Server 初始化与路由注册
      handler_download.go   # 下载相关 API
      handler_config.go     # 配置管理 API
      handler_auth.go       # Telegram 登录认证
      ws.go                 # WebSocket Hub
      middleware.go          # CORS、鉴权中间件
    config/config.go        # 配置读写，支持热加载
    download/
      manager.go            # 下载/扫描/快速测试核心逻辑
      state.go              # 线程安全的下载状态管理
    telegram/
      client.go             # gotd/td MTProto 客户端封装
      auth.go               # Telegram 认证流程
frontend/
  src/
    api/http.ts             # Axios HTTP 客户端
    api/ws.ts               # WebSocket 自动重连
    views/ConfigView.vue    # 配置页面
    views/DownloadView.vue  # 下载管理页面
scripts/
  build-fnos.sh             # fnOS 安装包构建脚本
Makefile                    # 构建命令
```

## 快速开始

### 环境要求

- Go 1.25+
- Node.js 18+
- npm

### 开发模式

前后端分别启动：

```bash
# 终端 1：后端
go run ./server/cmd/server/ -port 45116

# 终端 2：前端
cd frontend && npm install && npm run dev
```

### 构建部署

```bash
# 完整构建（前端 + 后端）
make build

# 仅构建前端
make build-frontend

# 仅构建后端
make build-server

# 清理构建产物
make clean

# 构建 fnOS 安装包
make fnos
```

构建产物为单个二进制文件 `tg-music-server`，前端资源通过 `//go:embed dist/*` 嵌入。

## 启动参数

| 参数                | 默认值           | 说明                   |
|-------------------|---------------|----------------------|
| `-port`           | `45116`       | HTTP 监听端口            |
| `-config`         | `config.json` | 配置文件路径               |
| `-download-dir`   | `./downloads` | 覆盖下载目录               |
| `-session-dir`    | `./sessions`  | 覆盖 session 存储目录      |
| `-socket`         | 空             | Unix Socket 路径（网关模式） |

环境变量：

- `TRIM_DATA_ACCESSIBLE_PATHS`：fnOS 可写路径（冒号分隔）

## 配置文件 (config.json)

```json
{
  "api_id": 0,
  "api_hash": "",
  "session_name": "my_session",
  "channels": [
    "VmoMusic",
    "FLAC_HR",
    "cjCoolMusic"
  ],
  "proxy": {
    "scheme": "socks5",
    "hostname": "",
    "port": 0,
    "username": "",
    "password": ""
  },
  "download_dir": "./downloads",
  "download_time_start": "",
  "download_time_end": "",
  "audio_formats": [],
  "session_dir": "./sessions",
  "phone_number": ""
}
```

`audio_formats`：允许下载的音频格式列表（小写扩展名，如 `["mp3", "flac"]`），空数组表示下载全部格式。在设置页"音频格式"中勾选，对扫描和批量下载生效。

配置文件支持热加载——修改后自动生效，无需重启服务。

## API 接口

### Telegram 认证

| 方法   | 路径                        | 说明           |
|------|---------------------------|--------------|
| GET  | `/api/tg/auth/status`    | 查询认证状态       |
| POST | `/api/tg/auth/send_code` | 发送验证码        |
| POST | `/api/tg/auth/sign_in`   | 登录（验证码/二次密码） |
| POST | `/api/tg/auth/logout`    | 登出           |

### 配置

| 方法   | 路径                       | 说明      |
|------|--------------------------|---------|
| GET  | `/api/config`            | 获取当前配置  |
| POST | `/api/config`            | 保存配置    |
| POST | `/api/config/proxy/test` | 测试代理连通性 |

### 系统

| 方法  | 路径                 | 说明   |
|-----|--------------------|------|
| GET | `/api/system/info` | 系统信息 |

### 任务

| 方法   | 路径                     | 说明         |
|------|------------------------|------------|
| GET  | `/api/task/status`     | 下载状态       |
| GET  | `/api/task/list`       | 获取扫描结果列表   |
| GET  | `/api/task/dirs`       | 可用目录列表     |
| POST | `/api/task/start`      | 开始全部下载     |
| POST | `/api/task/stop`       | 停止下载       |
| POST | `/api/task/scan`       | 扫描频道音频     |
| POST | `/api/task/single`     | 下载单条消息     |
| POST | `/api/task/quick_test` | 快速测试（首条音频） |

### WebSocket

连接 `ws://host:port/api/ws`，实时接收下载进度和日志。

## 工作流程

1. **配置** → 填入 Telegram API 凭证、频道列表、代理
2. **认证** → 手机号发送验证码，完成 Telegram 登录
3. **快速测试** → 验证连接正常，下载第一条音频
4. **扫描** → 扫描频道获取音频消息列表
5. **下载** → 全部下载或选择单条下载

文件保存格式：`{频道名}/{文件名}`，按频道建子目录。

## 支持的音频格式

mp3, ogg, flac, wav, aac, m4a
