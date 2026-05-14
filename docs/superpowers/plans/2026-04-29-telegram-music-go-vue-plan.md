# Telegram Music Download Go + Vue 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the Python Flask + Pyrogram Telegram music downloader to a Go (Gin + gotd/telegram) backend with Vue 3 + Vite SPA frontend, embedded as a single binary via go:embed, with WebSocket real-time status/log pushing.

**Architecture:** Single Go binary with embedded Vue 3 SPA. Gin serves pages + JSON APIs + WebSocket. gotd/telegram handles MTProto connections. Download manager runs tasks in goroutines with context cancellation. WebSocket Hub broadcasts state/log changes to all connected clients.

**Tech Stack:** Go 1.22+, Gin, gotd/telegram, gorilla/websocket, Vue 3, Vite, TypeScript, axios

---

### Task 1: Go Project Foundation

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`

- [ ] **Step 1: Initialize Go module and create config**

```bash
go mod init tg-music
```

Create `internal/config/config.go`:

```go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Proxy struct {
	Scheme   string `json:"scheme"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	APIID       int    `json:"api_id"`
	APIHash     string `json:"api_hash"`
	SessionName string `json:"session_name"`
	Channels    []string `json:"channels"`
	Proxy       Proxy  `json:"proxy"`
	DownloadDir string `json:"download_dir"`
	SessionDir  string `json:"session_dir"`
	PhoneNumber string `json:"phone_number"`
}

func Defaults() Config {
	return Config{
		SessionName: "my_session",
		Channels:    []string{"VmoMusic", "FLAC_HR", "cjCoolMusic"},
		Proxy:       Proxy{Scheme: "socks5"},
		DownloadDir: "./downloads",
		SessionDir:  "./sessions",
	}
}

type Manager struct {
	mu   sync.RWMutex
	cfg  Config
	path string
}

func NewManager(configPath string) (*Manager, error) {
	cfg := Defaults()
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	return &Manager{cfg: cfg, path: configPath}, nil
}

func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c := m.cfg
	return c
}

func (m *Manager) Save(c Config) error {
	m.mu.Lock()
	m.cfg = c
	m.mu.Unlock()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(m.path)
	_ = os.MkdirAll(dir, 0o755)
	return os.WriteFile(m.path, data, 0o644)
}
```

- [ ] **Step 2: Commit**

```bash
git add go.mod internal/config/config.go
git commit -m "feat: add Go module and config manager"
```

---

### Task 2: Gin Server + Basic Routes

**Files:**
- Create: `cmd/server/main.go`
- Create: `internal/api/server.go`
- Create: `internal/api/middleware.go`
- Create: `internal/api/handler_config.go`

- [ ] **Step 1: Create server bootstrap**

Create `internal/api/server.go`:

```go
package api

import (
	"embed"
	"io/fs"
	"net/http"
	"tg-music/internal/config"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var distFS embed.FS

type Server struct {
	router  *gin.Engine
	config  *config.Manager
}

func NewServer(cfg *config.Manager) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	s := &Server{router: r, config: cfg}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.router.Use(corsMiddleware())

	// SPA fallback — serve index.html for non-API routes
	s.router.NoRoute(s.spaFallback())

	// API routes
	api := s.router.Group("/api")
	api.Use(authMiddleware(s.config))

	api.GET("/config", s.getConfig)
	api.POST("/config", s.saveConfig)
}

func (s *Server) spaFallback() gin.HandlerFunc {
	indexHTML, err := distFS.ReadFile("dist/index.html")
	if err != nil {
		indexHTML = []byte("<h1>Frontend not built. Run: make build</h1>")
	}
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	}
}

func (s *Server) Run(addr string) error {
	// Serve static assets from embedded dist
	sub, _ := fs.Sub(distFS, "dist")
	s.router.StaticFS("/assets", http.FS(sub))

	return s.router.Run(addr)
}
```

Create `internal/api/middleware.go`:

```go
package api

import (
	"net/http"
	"tg-music/internal/config"

	"github.com/gin-gonic/gin"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-Admin-Password")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func authMiddleware(cfg *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminPwd := c.GetHeader("X-Admin-Password")
		if adminPwd == "" {
			adminPwd = c.Query("password")
		}

		// Default admin password from env, fallback "admin"
		expected := "admin"
		if envPwd := c.GetHeader("X-Admin-Password"); envPwd == "" {
			// Allow env override
		}
		_ = cfg // config reference available if needed

		// Simple: compare with X-Admin-Password header value "admin" as default
		if adminPwd != "admin" && adminPwd != "" {
			// Allow any non-empty password for now, matching Python behavior
			// In production this should be compared against env ADMIN_PASSWORD
		}
		if adminPwd == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
```

Create `internal/api/handler_config.go`:

```go
package api

import (
	"net/http"
	"tg-music/internal/config"

	"github.com/gin-gonic/gin"
)

func (s *Server) getConfig(c *gin.Context) {
	cfg := s.config.Get()
	c.JSON(http.StatusOK, cfg)
}

func (s *Server) saveConfig(c *gin.Context) {
	var cfg config.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data"})
		return
	}
	if err := s.config.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "配置已保存"})
}
```

- [ ] **Step 2: Create main.go**

Create `cmd/server/main.go`:

```go
package main

import (
	"fmt"
	"log"
	"os"
	"tg-music/internal/api"
	"tg-music/internal/config"
)

func main() {
	cfg, err := config.NewManager("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	srv := api.NewServer(cfg)
	fmt.Printf("🚀 Starting Telegram Music Manager: http://localhost%s\n", addr)

	if err := srv.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
```

- [ ] **Step 3: Install dependencies and verify build**

```bash
go get github.com/gin-gonic/gin@latest
go mod tidy
go build ./cmd/server/
```

Expected: binary builds successfully (will fail at runtime due to embed if dist/ doesn't exist yet — that's expected, we'll create it later).

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go internal/api/server.go internal/api/middleware.go internal/api/handler_config.go go.mod go.sum
git commit -m "feat: add Gin server with config API and SPA fallback"
```

---

### Task 3: gotd Telegram Client + Session

**Files:**
- Create: `internal/telegram/client.go`
- Create: `internal/telegram/auth.go`

- [ ] **Step 1: Install gotd and dependencies**

```bash
go get github.com/gotd/telegram@latest
go get github.com/gotd/td/telegram@latest
go get github.com/go-kit/kit@latest  # for log adapter if needed
go get golang.org/x/net/proxy@latest  # for SOCKS5 proxy
go get go.uber.org/zap@latest         # for gotd logging
```

- [ ] **Step 2: Create Telegram client wrapper**

Create `internal/telegram/client.go`:

```go
package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotd/telegram"
	"github.com/gotd/telegram/session"
	"github.com/gotd/telegram/tg"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
)

type Client struct {
	client   *telegram.Client
	raw      *tg.Client
	cfg      AppConfig
	logger   *zap.Logger
}

type AppConfig struct {
	APIID       int
	APIHash     string
	SessionDir  string
	SessionName string
	Proxy       ProxyConfig
}

type ProxyConfig struct {
	Scheme   string
	Hostname string
	Port     int
	Username string
	Password string
}

func NewClient(ctx context.Context, cfg AppConfig, logger *zap.Logger) (*Client, error) {
	if err := os.MkdirAll(cfg.SessionDir, 0o755); err != nil {
		return nil, err
	}

	sessPath := filepath.Join(cfg.SessionDir, cfg.SessionName+".json")
	store, err := session.NewFileStore(sessPath)
	if err != nil {
		return nil, fmt.Errorf("session store: %w", err)
	}

	opts := telegram.Options{
		SessionStorage: store,
		Logger:         logger,
	}

	// Proxy support
	if cfg.Proxy.Hostname != "" && cfg.Proxy.Port != 0 {
		var auth *proxy.Auth
		if cfg.Proxy.Username != "" {
			auth = &proxy.Auth{
				User:     cfg.Proxy.Username,
				Password: cfg.Proxy.Password,
			}
		}
		dialer, err := proxy.SOCKS5("tcp",
			fmt.Sprintf("%s:%d", cfg.Proxy.Hostname, cfg.Proxy.Port),
			auth,
			proxy.Direct,
		)
		if err != nil {
			return nil, fmt.Errorf("proxy: %w", err)
		}
		opts.Resolver = tg.DNSResolver{Dial: dialer.Dial}
	}

	tgClient := telegram.NewClient(cfg.APIID, cfg.APIHash, opts)

	return &Client{
		client: tgClient,
		cfg:    cfg,
		logger: logger,
	}, nil
}

func (c *Client) Run(ctx context.Context, f func(ctx context.Context) error) error {
	return c.client.Run(ctx, f)
}

func (c *Client) Raw() *tg.Client {
	return c.client.API()
}

func (c *Client) IsAuthorized(ctx context.Context) (bool, error) {
	status, err := c.client.API().AccountCheckAuthorization(ctx)
	if err != nil {
		return false, err
	}
	return status.AuthorizationType != nil, nil
}

func (c *Client) Disconnect(ctx context.Context) error {
	// gotd handles cleanup via context cancellation
	return nil
}
```

Create `internal/telegram/auth.go`:

```go
package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/telegram/tg"
	"github.com/gotd/telegram/tgauth"
)

type SendCodeResult struct {
	PhoneCodeHash string
}

func (c *Client) SendCode(ctx context.Context, phone string) (*SendCodeResult, error) {
	req := &tg.AuthSendCodeRequest{
		PhoneNumber: phone,
		APIID:       c.cfg.APIID,
		APIHash:     c.cfg.APIHash,
		Settings:    &tg.CodeSettings{},
	}

	sentCode, err := c.client.API().AuthSendCode(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send code: %w", err)
	}

	switch s := sentCode.(type) {
	case *tg.AuthSentCode:
		return &SendCodeResult{PhoneCodeHash: s.PhoneCodeHash}, nil
	case *tg.AuthSentCodeTypeTelegramMessage:
		return &SendCodeResult{PhoneCodeHash: s.PhoneCodeHash}, nil
	default:
		return &SendCodeResult{PhoneCodeHash: ""}, nil
	}
}

type SignInResult struct {
	Need2FA bool
	Success bool
	User    string
}

func (c *Client) SignIn(ctx context.Context, phone, code, phoneCodeHash, password2FA string) (*SignInResult, error) {
	auth := tgauth.Flow{
		Phone:          phone,
		Password:       password2FA,
		Code:           func(ctx context.Context, sentCode tg.AuthSentCodeClass) (string, error) { return code, nil },
		AcceptTermsOfService: func(ctx context.Context, tos tg.HelpTermsOfService) error {
			return &tgauth.AcceptTermsOfService{}
		},
	}

	_, err := tgauth.NewFlow(auth, tgauth.FlowOptions{}).SignIn(ctx, c.client.API(), phoneCodeHash, code)
	if err != nil {
		if _, ok := err.(*tgauth.PasswordNeededError); ok {
			return &SignInResult{Need2FA: true}, nil
		}
		return nil, fmt.Errorf("sign in: %w", err)
	}

	return &SignInResult{Success: true}, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/telegram/client.go internal/telegram/auth.go go.mod go.sum
git commit -m "feat: add gotd telegram client with auth and session"
```

---

### Task 4: Download Manager + State

**Files:**
- Create: `internal/download/state.go`
- Create: `internal/download/manager.go`

- [ ] **Step 1: Create download state**

Create `internal/download/state.go`:

```go
package download

import (
	"sync"
	"time"
)

type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

type State struct {
	Running         bool       `json:"running"`
	TotalDownloaded int        `json:"total_downloaded"`
	CurrentChannel  string     `json:"current_channel"`
	StartedAt       string     `json:"started_at"`
	Logs            []LogEntry `json:"logs"`
}

type ScannedMessage struct {
	Channel      string `json:"channel"`
	ChannelTitle string `json:"channel_title"`
	MsgID        int    `json:"msg_id"`
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	Exists       bool   `json:"exists"`
}

type DownloadState struct {
	mu       sync.RWMutex
	state    State
	messages []ScannedMessage
	scannedAt string
}

func NewDownloadState() *DownloadState {
	return &DownloadState{
		state: State{Logs: make([]LogEntry, 0, 100)},
	}
}

func (ds *DownloadState) Get() State {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	s := ds.state
	s.Logs = make([]LogEntry, len(ds.state.Logs))
	copy(s.Logs, ds.state.Logs)
	return s
}

func (ds *DownloadState) SetRunning(running bool, channel string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.state.Running = running
	ds.state.CurrentChannel = channel
	if running {
		ds.state.StartedAt = time.Now().Format("2006-01-02 15:04:05")
		ds.state.TotalDownloaded = 0
		ds.state.Logs = make([]LogEntry, 0, 100)
	}
}

func (ds *DownloadState) IncrementDownloaded() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.state.TotalDownloaded++
}

func (ds *DownloadState) AddLog(msg string) LogEntry {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Message: msg,
	}
	ds.state.Logs = append(ds.state.Logs, entry)
	if len(ds.state.Logs) > 500 {
		ds.state.Logs = ds.state.Logs[len(ds.state.Logs)-500:]
	}
	return entry
}

func (ds *DownloadState) SetMessages(msgs []ScannedMessage) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.messages = msgs
	ds.scannedAt = time.Now().Format("2006-01-02 15:04:05")
}

func (ds *DownloadState) GetMessages() ([]ScannedMessage, string) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	msgs := make([]ScannedMessage, len(ds.messages))
	copy(msgs, ds.messages)
	return msgs, ds.scannedAt
}

func (ds *DownloadState) MarkDownloaded(channel string, msgID int) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	for i := range ds.messages {
		if ds.messages[i].Channel == channel && ds.messages[i].MsgID == msgID {
			ds.messages[i].Exists = true
			break
		}
	}
}
```

- [ ] **Step 2: Create download manager**

Create `internal/download/manager.go`:

```go
package download

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gotd/telegram/tg"
)

type TelegramAPI interface {
	Run(ctx context.Context, f func(ctx context.Context) error) error
	Raw() *tg.Client
}

type Manager struct {
	state   *DownloadState
	client  TelegramAPI
	hub     Broadcaster
	mu      sync.Mutex
	cancel  context.CancelFunc
}

type Broadcaster interface {
	Broadcast(typ string, data interface{})
}

func NewManager(client TelegramAPI, state *DownloadState, hub Broadcaster) *Manager {
	return &Manager{client: client, state: state, hub: hub}
}

func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[\\/*?:"<>|]`)
	return re.ReplaceAllString(name, "_")
}

func humanReadableSize(size int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	s := float64(size)
	i := 0
	for s >= 1024 && i < len(units)-1 {
		s /= 1024
		i++
	}
	return fmt.Sprintf("%.1f%s", s, units[i])
}

func (m *Manager) Start(ctx context.Context, channels []string) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return fmt.Errorf("download already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	m.state.SetRunning(true, "")
	m.state.AddLog("🚀 开始下载任务")

	go func() {
		defer func() {
			m.mu.Lock()
			m.cancel = nil
			m.mu.Unlock()
			m.state.SetRunning(false, "")
		}()

		for _, ch := range channels {
			select {
			case <-ctx.Done():
				m.state.AddLog("⛔ 下载任务已手动停止")
				return
			default:
			}

			m.state.SetRunning(true, ch)
			m.state.AddLog(fmt.Sprintf("📡 开始处理频道: %s", ch))

			count, err := m.downloadChannel(ctx, ch)
			if err != nil {
				m.state.AddLog(fmt.Sprintf("❌ 频道 %s 下载失败: %v", ch, err))
				continue
			}
			m.state.AddLog(fmt.Sprintf("📦 频道 %s 下载完毕，共下载 %d 个文件", ch, count))
		}

		m.state.AddLog(fmt.Sprintf("🎉 全部完成，共下载 %d 个文件", m.state.Get().TotalDownloaded))
	}()

	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel == nil {
		return fmt.Errorf("no download running")
	}
	m.cancel()
	return nil
}

func (m *Manager) ScanChannels(ctx context.Context, channels []string) error {
	go func() {
		for _, ch := range channels {
			m.state.AddLog(fmt.Sprintf("📡 扫描频道: %s", ch))
			msgs, err := m.scanChannel(ctx, ch)
			if err != nil {
				m.state.AddLog(fmt.Sprintf("❌ 扫描频道 %s 失败: %v", ch, err))
				continue
			}

			// Merge messages
			existing, _ := m.state.GetMessages()
			existing = append(existing, msgs...)
			m.state.SetMessages(existing)
		}
		_, scannedAt := m.state.GetMessages()
		m.hub.Broadcast("scan_result", map[string]interface{}{
			"messages":   m.state.GetMessages(),
			"scanned_at": scannedAt,
		})
		m.state.AddLog("📋 扫描完成")
	}()

	return nil
}

func (m *Manager) DownloadSingle(ctx context.Context, channel, dir string, msgID int) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return fmt.Errorf("download already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	m.state.SetRunning(true, channel)

	go func() {
		defer func() {
			m.mu.Lock()
			m.cancel = nil
			m.mu.Unlock()
			m.state.SetRunning(false, "")
		}()

		success, err := m.downloadMessage(ctx, channel, dir, msgID)
		if err != nil || !success {
			m.state.AddLog("❌ 下载失败")
			return
		}
		m.state.MarkDownloaded(channel, msgID)
		m.state.IncrementDownloaded()
		m.state.AddLog("🎉 下载完成")
	}()

	return nil
}

func (m *Manager) QuickTest(ctx context.Context, channel, dir string) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return fmt.Errorf("download already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	m.state.SetRunning(true, channel)
	m.state.AddLog(fmt.Sprintf("📡 测试频道: %s", channel))
	m.state.AddLog("🔍 获取第一条音频...")

	go func() {
		defer func() {
			m.mu.Lock()
			m.cancel = nil
			m.mu.Unlock()
			m.state.SetRunning(false, "")
		}()

		// Get channel info
		chats, err := m.client.Raw().ContactsResolveUsername(ctx, strings.TrimPrefix(channel, "@"))
		if err != nil {
			m.state.AddLog(fmt.Sprintf("❌ 频道不存在: %v", err))
			return
		}

		peer := &tg.InputPeerChannel{
			ChannelID:  chats.Chats[0].GetID(),
			AccessHash: chats.Chats[0].(*tg.Channel).AccessHash,
		}

		// Get recent messages
		messages, err := m.client.Raw().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  peer,
			Limit: 20,
		})
		if err != nil {
			m.state.AddLog(fmt.Sprintf("❌ 获取消息失败: %v", err))
			return
		}

		for _, msg := range messages.(*tg.MessagesMessages).Messages {
			if mMsg, ok := msg.(*tg.Message); ok && mMsg.Media != nil {
				if _, ok := mMsg.Media.(*tg.MessageMediaDocument); ok {
					m.state.AddLog(fmt.Sprintf("🎵 找到音频，消息ID: %d", mMsg.ID))
					success, _ := m.downloadMessage(ctx, channel, dir, mMsg.ID)
					if success {
						m.state.MarkDownloaded(channel, mMsg.ID)
						m.state.IncrementDownloaded()
						m.state.AddLog("🎉 快速测试完成")
					} else {
						m.state.AddLog("❌ 下载失败")
					}
					return
				}
			}
		}

		m.state.AddLog("⚠️ 最近20条消息没有音频")
	}()

	return nil
}

func (m *Manager) downloadChannel(ctx context.Context, channel string) (int, error) {
	// Resolve username
	chats, err := m.client.Raw().ContactsResolveUsername(ctx, strings.TrimPrefix(channel, "@"))
	if err != nil {
		return 0, fmt.Errorf("resolve channel: %w", err)
	}

	ch := chats.Chats[0]
	channelTitle := ch.GetTitle()
	peer := &tg.InputPeerChannel{
		ChannelID:  ch.GetID(),
		AccessHash: ch.(*tg.Channel).AccessHash,
	}

	// Create download dir
	dirPath := filepath.Join("downloads", channel)
	_ = os.MkdirAll(dirPath, 0o755)

	// Paginate through messages
	count := 0
	maxID := 0
	for {
		select {
		case <-ctx.Done():
			return count, nil
		default:
		}

		history, err := m.client.Raw().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:    peer,
			MaxID:   maxID,
			Limit:   100,
			OffsetDate: 0,
			OffsetID:   0,
			AddOffset:  0,
		})
		if err != nil {
			return count, err
		}

		mm, ok := history.(*tg.MessagesMessages)
		if !ok || len(mm.Messages) == 0 {
			break
		}

		for _, msg := range mm.Messages {
			if msgMsg, ok := msg.(*tg.Message); ok && msgMsg.Media != nil {
				if mediaDoc, ok := msgMsg.Media.(*tg.MessageMediaDocument); ok {
					if _, ok := mediaDoc.Document.(*tg.Document); ok {
						// Check if it's audio
						if doc, ok := mediaDoc.Document.(*tg.Document); ok {
							if isAudio(doc) {
								savePath := filepath.Join(dirPath, fmt.Sprintf("%d_%s", msgMsg.ID, sanitizeFilename(doc.FileName)))
								if _, err := os.Stat(savePath); err == nil {
									m.state.AddLog(fmt.Sprintf("✅ 文件已存在，跳过: %s", filepath.Base(savePath)))
									continue
								}

								m.state.AddLog(fmt.Sprintf("🎵 正在下载: %s", filepath.Base(savePath)))
								if err := m.downloadDocument(ctx, doc, savePath); err != nil {
									m.state.AddLog(fmt.Sprintf("❌ 下载失败: %v", err))
									continue
								}
								m.state.AddLog("✅ 下载完成")
								m.state.IncrementDownloaded()
								count++
							}
						}
					}
				}
			}
			maxID = msg.GetID()
		}

		if len(mm.Messages) < 100 {
			break
		}
	}

	return count, nil
}

func (m *Manager) scanChannel(ctx context.Context, channel string) ([]ScannedMessage, error) {
	chats, err := m.client.Raw().ContactsResolveUsername(ctx, strings.TrimPrefix(channel, "@"))
	if err != nil {
		return nil, fmt.Errorf("resolve channel: %w", err)
	}

	ch := chats.Chats[0]
	channelTitle := ch.GetTitle()
	peer := &tg.InputPeerChannel{
		ChannelID:  ch.GetID(),
		AccessHash: ch.(*tg.Channel).AccessHash,
	}

	dirPath := filepath.Join("downloads", channel)
	_ = os.MkdirAll(dirPath, 0o755)

	var messages []ScannedMessage
	maxID := 0

	for {
		history, err := m.client.Raw().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  peer,
			MaxID: maxID,
			Limit: 100,
		})
		if err != nil {
			return messages, err
		}

		mm, ok := history.(*tg.MessagesMessages)
		if !ok || len(mm.Messages) == 0 {
			break
		}

		for _, msg := range mm.Messages {
			if msgMsg, ok := msg.(*tg.Message); ok && msgMsg.Media != nil {
				if mediaDoc, ok := msgMsg.Media.(*tg.MessageMediaDocument); ok {
					if doc, ok := mediaDoc.Document.(*tg.Document); ok {
						if isAudio(doc) {
							fileName := sanitizeFilename(doc.FileName)
							savePath := filepath.Join(dirPath, fmt.Sprintf("%d_%s", msgMsg.ID, fileName))
							messages = append(messages, ScannedMessage{
								Channel:      channel,
								ChannelTitle: channelTitle,
								MsgID:        msgMsg.ID,
								FileName:     fileName,
								FileSize:     doc.Size,
								Exists:       fileExists(savePath),
							})
						}
					}
				}
			}
			maxID = msg.GetID()
		}

		if len(mm.Messages) < 100 {
			break
		}
	}

	return messages, nil
}

func (m *Manager) downloadMessage(ctx context.Context, channel, dir string, msgID int) (bool, error) {
	chats, err := m.client.Raw().ContactsResolveUsername(ctx, strings.TrimPrefix(channel, "@"))
	if err != nil {
		return false, err
	}

	ch := chats.Chats[0]
	peer := &tg.InputPeerChannel{
		ChannelID:  ch.GetID(),
		AccessHash: ch.(*tg.Channel).AccessHash,
	}

	msgs, err := m.client.Raw().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: peer,
		ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}},
	})
	if err != nil {
		return false, err
	}

	container, ok := msgs.(*tg.MessagesChannelMessages)
	if !ok || len(container.Messages) == 0 {
		return false, fmt.Errorf("message not found")
	}

	msg, ok := container.Messages[0].(*tg.Message)
	if !ok || msg.Media == nil {
		return false, fmt.Errorf("no media in message")
	}

	mediaDoc, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok {
		return false, fmt.Errorf("not a document")
	}

	doc, ok := mediaDoc.Document.(*tg.Document)
	if !ok {
		return false, fmt.Errorf("not a document")
	}

	dirPath := filepath.Join(dir, channel)
	_ = os.MkdirAll(dirPath, 0o755)
	savePath := filepath.Join(dirPath, fmt.Sprintf("%d_%s", msgID, sanitizeFilename(doc.FileName)))

	if fileExists(savePath) {
		return true, nil
	}

	return true, m.downloadDocument(ctx, doc, savePath)
}

func (m *Manager) downloadDocument(ctx context.Context, doc *tg.Document, savePath string) error {
	location := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}

	file, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = m.client.Raw().Download(ctx, location, func(ctx context.Context, r io.Reader) error {
		_, err := io.Copy(file, r)
		return err
	})

	return err
}

func isAudio(doc *tg.Document) bool {
	for _, attr := range doc.Attributes {
		if _, ok := attr.(*tg.DocumentAttributeAudio); ok {
			return true
		}
	}
	// Also check MIME type
	return strings.HasPrefix(doc.MimeType, "audio/")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/download/state.go internal/download/manager.go
git commit -m "feat: add download manager with state tracking and broadcast"
```

---

### Task 5: WebSocket Hub + Auth/Download Handlers

**Files:**
- Create: `internal/api/ws.go`
- Create: `internal/api/handler_auth.go`
- Create: `internal/api/handler_download.go`
- Modify: `internal/api/server.go` (add WS route + auth/download routes)
- Modify: `internal/api/middleware.go` (fix auth to use env ADMIN_PASSWORD)

- [ ] **Step 1: Install websocket dependency**

```bash
go get github.com/gorilla/websocket@latest
```

- [ ] **Step 2: Create WebSocket hub**

Create `internal/api/ws.go`:

```go
package api

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[*websocket.Conn]bool
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan WSMessage
	upgrader   websocket.Upgrader
	getState   func() interface{}
}

func NewHub(getState func() interface{}) *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		broadcast:  make(chan WSMessage, 256),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		getState: getState,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			// Send initial state snapshot
			if h.getState != nil {
				state := h.getState()
				h.broadcast <- WSMessage{Type: "download_status", Payload: state}
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if err := client.WriteJSON(msg); err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Broadcast(typ string, data interface{}) {
	h.broadcast <- WSMessage{Type: typ, Payload: data}
}

func (h *Hub) HandleWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.register <- conn

	// Read loop (keep connection alive)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.unregister <- conn
			break
		}
	}
}
```

- [ ] **Step 3: Create auth handler**

Create `internal/api/handler_auth.go`:

```go
package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type AuthState struct {
	PhoneCodeHash string
	PhoneNumber   string
}

func (s *Server) getAuthStatus(c *gin.Context) {
	cfg := s.config.Get()
	sessionPath := filepath.Join(cfg.SessionDir, cfg.SessionName+".json")
	_, err := os.Stat(sessionPath)
	exists := err == nil

	c.JSON(http.StatusOK, gin.H{
		"authorized":   exists,
		"phone_number": cfg.PhoneNumber,
		"needs_2fa":    false,
	})
}

func (s *Server) sendCode(c *gin.Context) {
	var req struct {
		PhoneNumber string `json:"phone_number" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入手机号"})
		return
	}

	// Save phone number to config
	cfg := s.config.Get()
	cfg.PhoneNumber = req.PhoneNumber
	_ = s.config.Save(cfg)

	// Start persistent TG client if not running
	if err := s.ensureTelegramClient(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result, err := s.tgClient.SendCode(context.Background(), req.PhoneNumber)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.authStateMu.Lock()
	s.authState = AuthState{
		PhoneCodeHash: result.PhoneCodeHash,
		PhoneNumber:   req.PhoneNumber,
	}
	s.authStateMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"message":         "验证码已发送",
		"phone_code_hash": result.PhoneCodeHash,
	})
}

func (s *Server) signIn(c *gin.Context) {
	var req struct {
		PhoneCode string `json:"phone_code" binding:"required"`
		Password  string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入验证码"})
		return
	}

	s.authStateMu.Lock()
	state := s.authState
	s.authStateMu.Unlock()

	if state.PhoneCodeHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先发送验证码"})
		return
	}

	if err := s.ensureTelegramClient(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result, err := s.tgClient.SignIn(context.Background(), state.PhoneNumber, req.PhoneCode, state.PhoneCodeHash, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if result.Need2FA {
		c.JSON(http.StatusOK, gin.H{"need_2fa": true, "message": "需要输入两步验证密码"})
		return
	}

	if result.Success {
		s.authStateMu.Lock()
		s.authState = AuthState{}
		s.authStateMu.Unlock()
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "登录成功"})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "登录失败"})
}

func (s *Server) logout(c *gin.Context) {
	s.authStateMu.Lock()
	s.authState = AuthState{}
	s.authStateMu.Unlock()

	// Cancel persistent background connection
	if s.cancelRunning != nil {
		s.cancelRunning()
		s.cancelRunning = nil
	}
	s.tgClient = nil
	s.downloadManager = nil

	cfg := s.config.Get()
	sessionPath := filepath.Join(cfg.SessionDir, cfg.SessionName+".json")
	_ = os.Remove(sessionPath)

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已退出登录"})
}
```

- [ ] **Step 4: Create download handler**

Create `internal/api/handler_download.go`:

```go
package api

import (
	"context"
	"net/http"
	"sync"
	"tg-music/internal/config"

	"github.com/gin-gonic/gin"
)

func (s *Server) getDownloadStatus(c *gin.Context) {
	state := s.downloadState.Get()
	c.JSON(http.StatusOK, state)
}

func (s *Server) startDownload(c *gin.Context) {
	cfg := s.config.Get()
	if len(cfg.Channels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有配置频道"})
		return
	}

	if err := s.downloadManager.Start(context.Background(), cfg.Channels); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "下载任务已启动"})
}

func (s *Server) stopDownload(c *gin.Context) {
	if err := s.downloadManager.Stop(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.downloadState.AddLog("⛔ 下载任务已手动停止")
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "下载任务已停止"})
}

func (s *Server) scanChannels(c *gin.Context) {
	cfg := s.config.Get()
	if err := s.downloadManager.ScanChannels(context.Background(), cfg.Channels); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "扫描已开始"})
}

func (s *Server) getDownloadList(c *gin.Context) {
	msgs, scannedAt := s.downloadState.GetMessages()
	state := s.downloadState.Get()
	c.JSON(http.StatusOK, gin.H{
		"messages":   msgs,
		"scanned_at": scannedAt,
		"running":    state.Running,
	})
}

func (s *Server) downloadSingle(c *gin.Context) {
	var req struct {
		Channel string `json:"channel" binding:"required"`
		MsgID   int    `json:"msg_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing channel or msg_id"})
		return
	}

	cfg := s.config.Get()
	if err := s.downloadManager.DownloadSingle(context.Background(), req.Channel, cfg.DownloadDir, req.MsgID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "开始下载: " + req.Channel})
}

func (s *Server) quickTest(c *gin.Context) {
	cfg := s.config.Get()
	if len(cfg.Channels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有配置频道"})
		return
	}

	if err := s.downloadManager.QuickTest(context.Background(), cfg.Channels[0], cfg.DownloadDir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "快速测试已启动"})
}
```

- [ ] **Step 5: Update server.go with all routes and state**

Replace `internal/api/server.go` with:

```go
package api

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"tg-music/internal/config"
	"tg-music/internal/download"
	"tg-music/internal/telegram"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

//go:embed dist/*
var distFS embed.FS

type Server struct {
	router          *gin.Engine
	config          *config.Manager
	downloadState   *download.DownloadState
	downloadManager *download.Manager
	hub             *Hub
	authState       AuthState
	authStateMu     sync.RWMutex
	tgClient        *telegram.Client
	runningCtx      context.Context
	cancelRunning   context.CancelFunc
	logger          *zap.Logger
}

func NewServer(cfg *config.Manager) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	logger, _ := zap.NewProduction()
	dlState := download.NewDownloadState()

	s := &Server{
		router:        r,
		config:        cfg,
		downloadState: dlState,
		logger:        logger,
	}

	// Create Hub
	s.hub = NewHub(func() interface{} {
		return dlState.Get()
	})
	go s.hub.Run()

	s.registerRoutes()
	return s
}

// ensureTelegramClient creates the Telegram client (persistent) and download manager.
// Called lazily on first auth/download use.
func (s *Server) ensureTelegramClient() error {
	if s.tgClient != nil {
		return nil
	}
	cfg := s.config.Get()
	if cfg.APIID == 0 || cfg.APIHash == "" {
		return fmt.Errorf("telegram API not configured")
	}

	client, err := telegram.NewClient(context.Background(), telegram.AppConfig{
		APIID:       cfg.APIID,
		APIHash:     cfg.APIHash,
		SessionDir:  cfg.SessionDir,
		SessionName: cfg.SessionName,
		Proxy: telegram.ProxyConfig{
			Scheme:   cfg.Proxy.Scheme,
			Hostname: cfg.Proxy.Hostname,
			Port:     cfg.Proxy.Port,
			Username: cfg.Proxy.Username,
			Password: cfg.Proxy.Password,
		},
	}, s.logger)
	if err != nil {
		return err
	}

	s.tgClient = client
	s.downloadManager = download.NewManager(client, s.downloadState, s.hub)

	// Start persistent background connection using Run() with a blocking function
	ctx, cancel := context.WithCancel(context.Background())
	s.runningCtx = ctx
	s.cancelRunning = cancel
	go func() {
		_ = client.Run(ctx, func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		})
	}()
	return nil
}

func (s *Server) registerRoutes() {
	s.router.Use(corsMiddleware())
	s.router.NoRoute(s.spaFallback())

	// WebSocket
	s.router.GET("/api/ws", s.hub.HandleWS)

	// API routes
	api := s.router.Group("/api")
	api.Use(authMiddleware(s.config))

	// Config
	api.GET("/config", s.getConfig)
	api.POST("/config", s.saveConfig)

	// Auth
	api.GET("/auth/status", s.getAuthStatus)
	api.POST("/auth/send_code", s.sendCode)
	api.POST("/auth/sign_in", s.signIn)
	api.POST("/auth/logout", s.logout)

	// Download
	api.GET("/download/status", s.getDownloadStatus)
	api.POST("/download/start", s.startDownload)
	api.POST("/download/stop", s.stopDownload)
	api.POST("/download/scan", s.scanChannels)
	api.GET("/download/list", s.getDownloadList)
	api.POST("/download/single", s.downloadSingle)
	api.POST("/download/quick_test", s.quickTest)
}

func (s *Server) spaFallback() gin.HandlerFunc {
	indexHTML, err := distFS.ReadFile("dist/index.html")
	if err != nil {
		indexHTML = []byte("<h1>Frontend not built. Run: make build</h1>")
	}
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	}
}

func (s *Server) Run(addr string) error {
	sub, _ := fs.Sub(distFS, "dist")
	s.router.StaticFS("/assets", http.FS(sub))
	return s.router.Run(addr)
}
```

- [ ] **Step 6: Fix middleware to use ADMIN_PASSWORD env**

Replace `internal/api/middleware.go` with:

```go
package api

import (
	"net/http"
	"os"
	"tg-music/internal/config"

	"github.com/gin-gonic/gin"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-Admin-Password")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func authMiddleware(cfg *config.Manager) gin.HandlerFunc {
	expected := os.Getenv("ADMIN_PASSWORD")
	if expected == "" {
		expected = "admin"
	}

	return func(c *gin.Context) {
		pwd := c.GetHeader("X-Admin-Password")
		if pwd == "" || pwd != expected {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 7: Commit**

```bash
git add internal/api/ws.go internal/api/handler_auth.go internal/api/handler_download.go internal/api/server.go internal/api/middleware.go go.mod go.sum
git commit -m "feat: add WebSocket hub, auth handlers, and download handlers"
```

---

### Task 6: Vue 3 Frontend Setup

**Files:**
- Create: `web/package.json`
- Create: `web/tsconfig.json`
- Create: `web/vite.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.ts`
- Create: `web/src/App.vue`
- Create: `web/src/style.css`

- [ ] **Step 1: Initialize Vue project**

```bash
cd web && npm init -y
npm install vue@latest vue-router@4 axios
npm install -D typescript@latest vite@latest @vitejs/plugin-vue @vue/tsconfig
```

Create `web/package.json` (if not created by npm):

```json
{
  "name": "tg-music-web",
  "private": true,
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.5.0",
    "vue-router": "^4.5.0",
    "axios": "^1.7.0"
  },
  "devDependencies": {
    "typescript": "^5.7.0",
    "vite": "^6.0.0",
    "@vitejs/plugin-vue": "^5.2.0",
    "@vue/tsconfig": "^0.7.0",
    "vue-tsc": "^2.2.0"
  }
}
```

- [ ] **Step 2: Create config files**

Create `web/tsconfig.json`:

```json
{
  "extends": "@vue/tsconfig/tsconfig.dom.json",
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    }
  },
  "include": ["src/**/*.ts", "src/**/*.vue", "env.d.ts"]
}
```

Create `web/env.d.ts`:

```typescript
/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
```

Create `web/vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
  },
})
```

Create `web/index.html`:

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Telegram 音乐下载</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
```

- [ ] **Step 3: Create Vue entry and router**

Create `web/src/main.ts`:

```typescript
import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import ConfigView from './views/ConfigView.vue'
import DownloadView from './views/DownloadView.vue'
import './style.css'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: ConfigView },
    { path: '/download', component: DownloadView },
  ],
})

const app = createApp(App)
app.use(router)
app.mount('#app')
```

Create `web/src/App.vue`:

```vue
<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
</script>

<template>
  <div class="container">
    <header>
      <h1>Telegram 音乐下载</h1>
      <nav>
        <a href="/" :class="{ active: route.path === '/' }" @click.prevent="router.push('/')">配置管理</a>
        <a href="/download" :class="{ active: route.path === '/download' }" @click.prevent="router.push('/download')">下载状态</a>
      </nav>
    </header>
    <router-view />
  </div>
</template>
```

- [ ] **Step 4: Port CSS style**

Create `web/src/style.css` (copy from existing `static/style.css` with minor adjustments):

```css
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background: #f0f2f5;
  color: #1a1a1a;
  line-height: 1.6;
}

.container {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
  padding: 16px 0;
}

header h1 {
  font-size: 1.5rem;
  color: #1a1a1a;
}

nav a {
  text-decoration: none;
  color: #666;
  margin-left: 16px;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 0.9rem;
  transition: all 0.2s;
  cursor: pointer;
}

nav a.active, nav a:hover {
  background: #e8e8e8;
  color: #1a1a1a;
}

.card {
  background: #fff;
  border-radius: 10px;
  padding: 20px;
  margin-bottom: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

.card h2 {
  font-size: 1.1rem;
  margin-bottom: 16px;
  color: #333;
  border-bottom: 1px solid #eee;
  padding-bottom: 8px;
}

.form-group {
  margin-bottom: 12px;
  flex: 1;
}

.form-row {
  display: flex;
  gap: 12px;
}

.form-group label {
  display: block;
  font-size: 0.85rem;
  color: #666;
  margin-bottom: 4px;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 6px;
  font-size: 0.9rem;
  transition: border-color 0.2s;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: #4a90d9;
  box-shadow: 0 0 0 2px rgba(74, 144, 217, 0.15);
}

.channel-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
  align-items: center;
}

.channel-input {
  flex: 1;
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 6px;
  font-size: 0.9rem;
}

.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  font-size: 0.9rem;
  cursor: pointer;
  transition: all 0.2s;
  font-weight: 500;
}

.btn-primary { background: #4a90d9; color: #fff; }
.btn-primary:hover { background: #357abd; }
.btn-success { background: #34a853; color: #fff; }
.btn-success:hover { background: #2d8f47; }
.btn-danger { background: #ea4335; color: #fff; }
.btn-danger:hover { background: #c5221f; }
.btn-secondary { background: #f1f3f4; color: #333; }
.btn-secondary:hover { background: #e8eaed; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-sm { padding: 4px 10px; font-size: 0.8rem; }
.btn-info { background: #17a2b8; color: #fff; }
.btn-info:hover { background: #138496; }

.form-actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
}

.status-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.badge {
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 0.85rem;
  font-weight: 500;
}

.badge.running { background: #e6f4ea; color: #137333; }
.badge.idle { background: #f1f3f4; color: #666; }
.badge-done { background: #e6f4ea; color: #137333; }

.status-details {
  font-size: 0.9rem;
  color: #555;
  margin-bottom: 16px;
}

.status-details div {
  margin-bottom: 4px;
}

.logs-container {
  max-height: 500px;
  overflow-y: auto;
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 12px;
  border-radius: 8px;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.82rem;
}

.log-line {
  padding: 2px 0;
  border-bottom: 1px solid #2a2a2a;
}

.log-time {
  color: #6a9955;
  margin-right: 8px;
}

.toast {
  position: fixed;
  top: 20px;
  right: 20px;
  background: #333;
  color: #fff;
  padding: 12px 20px;
  border-radius: 8px;
  font-size: 0.9rem;
  opacity: 0;
  transform: translateY(-20px);
  transition: all 0.3s;
  z-index: 1000;
  pointer-events: none;
}

.toast.show {
  opacity: 1;
  transform: translateY(0);
}

.login-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
}

.login-card {
  background: #fff;
  border-radius: 12px;
  padding: 32px;
  width: 320px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
}

.login-card h2 {
  text-align: center;
  margin-bottom: 20px;
  color: #333;
}

.login-error {
  color: #ea4335;
  font-size: 0.85rem;
  min-height: 20px;
  margin-bottom: 8px;
}

.file-list {
  max-height: 600px;
  overflow-y: auto;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-bottom: 1px solid #f0f0f0;
  transition: background 0.15s;
}

.file-item:hover {
  background: #f8f9fa;
}

.file-item.file-exists {
  opacity: 0.6;
}

.file-checkbox {
  display: flex;
  align-items: center;
  min-width: 24px;
}

.file-info {
  flex: 1;
  min-width: 0;
}

.file-name {
  font-weight: 500;
  font-size: 0.9rem;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-meta {
  font-size: 0.8rem;
  color: #888;
  margin-top: 2px;
}

.filter-bar {
  display: flex;
  gap: 16px;
  align-items: center;
  margin-bottom: 12px;
  font-size: 0.85rem;
  color: #555;
}

.filter-bar input[type="text"] {
  flex: 1;
  padding: 6px 10px;
  border: 1px solid #ddd;
  border-radius: 6px;
  font-size: 0.85rem;
}

.filter-bar input[type="text"]:focus {
  outline: none;
  border-color: #4a90d9;
}

.empty-tip {
  text-align: center;
  color: #999;
  padding: 40px 0;
}
```

- [ ] **Step 5: Build Vue to verify**

```bash
cd web && npm install && npx vite build
```

- [ ] **Step 6: Commit**

```bash
git add web/
git commit -m "feat: add Vue 3 SPA with Vite and ported styles"
```

---

### Task 7: ConfigView.vue + Auth Flow

**Files:**
- Create: `web/src/api/http.ts`
- Create: `web/src/views/ConfigView.vue`

- [ ] **Step 1: Create HTTP API wrapper**

Create `web/src/api/http.ts`:

```typescript
import axios from 'axios'

const http = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

export function setAdminPassword(pwd: string) {
  http.defaults.headers.common['X-Admin-Password'] = pwd
}

export function getAdminPassword(): string {
  return http.defaults.headers.common['X-Admin-Password'] || ''
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
  const res = await http.post('/auth/send_code', { phone_number: phoneNumber })
  return res.data
}

export async function signIn(phoneCode: string, password?: string) {
  const res = await http.post('/auth/sign_in', { phone_code: phoneCode, password })
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
  const res = await http.post('/download/single', { channel, msg_id: msgId })
  return res.data
}

export async function quickTest() {
  const res = await http.post('/download/quick_test')
  return res.data
}
```

- [ ] **Step 2: Create ConfigView**

Create `web/src/views/ConfigView.vue`:

```vue
<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import {
  getConfig, saveConfig, getAuthStatus, sendCode, signIn, logout,
  setAdminPassword, getAdminPassword, startDownload,
} from '../api/http'
import { useRouter } from 'vue-router'

const router = useRouter()

const password = ref(getAdminPassword() || '')
const showLogin = ref(!getAdminPassword())
const loginError = ref('')

const apiId = ref(0)
const apiHash = ref('')
const sessionName = ref('my_session')
const proxyScheme = ref('socks5')
const proxyHostname = ref('')
const proxyPort = ref(0)
const proxyUsername = ref('')
const proxyPassword = ref('')
const sessionDir = ref('./sessions')
const downloadDir = ref('./downloads')
const channels = ref<string[]>([])

const authLoading = ref(true)
const authLoggedIn = ref(false)
const authPhone = ref('')

const showCodeSection = ref(false)
const authCode = ref('')
const show2FA = ref(false)
const auth2FA = ref('')

const toastMsg = ref('')

function showToast(msg: string) {
  toastMsg.value = msg
  setTimeout(() => { toastMsg.value = '' }, 3000)
}

async function doLogin() {
  if (!password.value) { loginError.value = '请输入密码'; return }
  setAdminPassword(password.value)
  try {
    const data = await getConfig()
    fillConfig(data)
    showLogin.value = false
    sessionStorage.setItem('tg_admin_pwd', password.value)
    await loadAuthStatus()
  } catch {
    loginError.value = '密码错误'
  }
}

function fillConfig(data: any) {
  apiId.value = data.api_id || 0
  apiHash.value = data.api_hash || ''
  sessionName.value = data.session_name || 'my_session'
  const p = data.proxy || {}
  proxyScheme.value = p.scheme || 'socks5'
  proxyHostname.value = p.hostname || ''
  proxyPort.value = p.port || 0
  proxyUsername.value = p.username || ''
  proxyPassword.value = p.password || ''
  sessionDir.value = data.session_dir || './sessions'
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
  if (!authPhone.value) return
  try {
    const data = await sendCode(authPhone.value)
    showCodeSection.value = true
    showToast('验证码已发送')
  } catch (e: any) {
    showToast(e.response?.data?.error || '发送失败')
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
    showToast(e.response?.data?.error || '登录失败')
  }
}

async function doSignIn2FA() {
  if (!authCode.value || !auth2FA.value) return
  try {
    const data = await signIn(authCode.value, auth2FA.value)
    showToast(data.message)
    await loadAuthStatus()
  } catch (e: any) {
    showToast(e.response?.data?.error || '登录失败')
  }
}

async function doLogout() {
  if (!confirm('确定要退出登录吗？')) return
  try {
    await logout()
    showToast('已退出登录')
    authLoggedIn.value = false
    showCodeSection.value = false
    show2FA.value = false
  } catch (e: any) {
    showToast(e.response?.data?.error || '退出失败')
  }
}

async function doSaveConfig() {
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
    session_dir: sessionDir.value,
    download_dir: downloadDir.value,
  }
  try {
    const res = await saveConfig(data)
    showToast(res.message)
  } catch {
    showToast('保存失败')
  }
}

async function doStartDownload() {
  try {
    await startDownload()
    router.push('/download')
  } catch (e: any) {
    showToast(e.response?.data?.error || '启动失败')
  }
}

function addChannel() { channels.value.push('') }
function removeChannel(i: number) { channels.value.splice(i, 1) }

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
    <!-- Login Overlay -->
    <div v-if="showLogin" class="login-overlay">
      <div class="login-card">
        <h2>管理登录</h2>
        <div class="form-group">
          <label>密码</label>
          <input type="password" v-model="password" placeholder="输入管理密码"
            @keydown.enter="doLogin" />
        </div>
        <div class="login-error">{{ loginError }}</div>
        <button class="btn btn-primary" style="width:100%" @click="doLogin">登录</button>
      </div>
    </div>

    <div v-if="!showLogin">
      <!-- Auth Card -->
      <div class="card">
        <h2>Telegram 认证</h2>
        <div v-if="authLoading">检查中...</div>
        <div v-else-if="!authLoggedIn">
          <div class="form-row">
            <div class="form-group">
              <label>手机号</label>
              <input type="text" v-model="authPhone" placeholder="+8613800138000" />
            </div>
            <div class="form-group">
              <button class="btn btn-info" @click="doSendCode">发送验证码</button>
            </div>
          </div>
          <div v-if="showCodeSection">
            <div class="form-row">
              <div class="form-group">
                <label>验证码</label>
                <input type="text" v-model="authCode" placeholder="SMS 验证码" />
              </div>
              <div class="form-group">
                <button class="btn btn-success" @click="doSignIn">登录</button>
              </div>
            </div>
            <div v-if="show2FA">
              <div class="form-row">
                <div class="form-group">
                  <label>两步验证密码</label>
                  <input type="password" v-model="auth2FA" placeholder="2FA 密码" />
                </div>
                <div class="form-group">
                  <button class="btn btn-success" @click="doSignIn2FA">登录</button>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-else>
          <div class="status-bar">
            <span class="badge running">已登录</span>
            <span>{{ authPhone }}</span>
          </div>
          <button class="btn btn-danger btn-sm" @click="doLogout">退出登录</button>
        </div>
      </div>

      <!-- Config Form -->
      <div class="card">
        <h2>API 凭证</h2>
        <div class="form-group">
          <label>API ID</label>
          <input type="number" v-model.number="apiId" />
        </div>
        <div class="form-group">
          <label>API Hash</label>
          <input type="text" v-model="apiHash" />
        </div>
        <div class="form-group">
          <label>Session 名称</label>
          <input type="text" v-model="sessionName" />
        </div>
      </div>

      <div class="card">
        <h2>代理配置</h2>
        <div class="form-row">
          <div class="form-group">
            <label>代理类型</label>
            <select v-model="proxyScheme">
              <option value="socks5">SOCKS5</option>
              <option value="socks4">SOCKS4</option>
              <option value="http">HTTP</option>
            </select>
          </div>
          <div class="form-group">
            <label>主机地址</label>
            <input type="text" v-model="proxyHostname" placeholder="127.0.0.1" />
          </div>
          <div class="form-group">
            <label>端口</label>
            <input type="number" v-model.number="proxyPort" placeholder="1080" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>用户名</label>
            <input type="text" v-model="proxyUsername" placeholder="可选" />
          </div>
          <div class="form-group">
            <label>密码</label>
            <input type="password" v-model="proxyPassword" placeholder="可选" />
          </div>
        </div>
      </div>

      <div class="card">
        <h2>频道列表</h2>
        <div v-for="(ch, i) in channels" :key="i" class="channel-row">
          <input class="channel-input" v-model="channels[i]" placeholder="频道用户名" />
          <button class="btn btn-danger btn-sm" @click="removeChannel(i)">删除</button>
        </div>
        <button class="btn btn-secondary" @click="addChannel">+ 添加频道</button>
      </div>

      <div class="card">
        <h2>存储路径</h2>
        <div class="form-row">
          <div class="form-group">
            <label>Session 目录</label>
            <input type="text" v-model="sessionDir" />
          </div>
          <div class="form-group">
            <label>下载目录</label>
            <input type="text" v-model="downloadDir" />
          </div>
        </div>
      </div>

      <div class="form-actions">
        <button class="btn btn-primary" @click="doSaveConfig">保存配置</button>
        <button class="btn btn-success" @click="doStartDownload">开始下载</button>
      </div>
    </div>

    <div class="toast" :class="{ show: toastMsg }">{{ toastMsg }}</div>
  </div>
</template>
```

- [ ] **Step 3: Commit**

```bash
git add web/src/api/http.ts web/src/views/ConfigView.vue
git commit -m "feat: add ConfigView with auth flow and config form"
```

---

### Task 8: DownloadView.vue + WebSocket Integration

**Files:**
- Create: `web/src/api/ws.ts`
- Create: `web/src/views/DownloadView.vue`

- [ ] **Step 1: Create WebSocket client**

Create `web/src/api/ws.ts`:

```typescript
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

export function connect(onStateSnapshot?: (payload: any) => void) {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${protocol}//${location.host}/api/ws`

  ws = new WebSocket(url)

  ws.onopen = () => {
    reconnectDelay = 5000
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
  ws?.close()
  handlers = []
}
```

- [ ] **Step 2: Create DownloadView**

Create `web/src/views/DownloadView.vue`:

```vue
<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import {
  getDownloadStatus, startDownload, stopDownload, scanChannels,
  getDownloadList, downloadSingle, quickTest,
  setAdminPassword, getAdminPassword,
} from '../api/http'
import { connect, onMessage, disconnect, WSMessage } from '../api/ws'

const showLogin = ref(!getAdminPassword())
const loginPassword = ref('')
const loginError = ref('')

const running = ref(false)
const totalDownloaded = ref(0)
const currentChannel = ref('-')
const startedAt = ref('-')
const scannedAt = ref('-')
const logs = ref<{ time: string; message: string }[]>([])

interface ScannedFile {
  channel: string
  channel_title: string
  msg_id: number
  file_name: string
  file_size: number
  exists: boolean
}
const messages = ref<ScannedFile[]>([])
const selectedSet = ref<Record<string, boolean>>({})
const filterExists = ref(false)
const searchInput = ref('')

function doLogin() {
  if (!loginPassword.value) { loginError.value = '请输入密码'; return }
  setAdminPassword(loginPassword.value)
  getDownloadStatus()
    .then(initPage)
    .then(() => { showLogin.value = false; sessionStorage.setItem('tg_admin_pwd', loginPassword.value) })
    .catch(() => { loginError.value = '密码错误' })
}

function initPage(data: any) {
  running.value = data.running
  totalDownloaded.value = data.total_downloaded
  startedAt.value = data.started_at || '-'
  logs.value = data.logs || []

  connect((snapshot: any) => {
    running.value = snapshot.running
    totalDownloaded.value = snapshot.total_downloaded
    currentChannel.value = snapshot.current_channel || '-'
    startedAt.value = snapshot.started_at || '-'
    logs.value = snapshot.logs || []
  })

  onMessage((msg: WSMessage) => {
    if (msg.type === 'log') {
      logs.value.push(msg.payload)
    } else if (msg.type === 'download_status') {
      running.value = msg.payload.running
      totalDownloaded.value = msg.payload.total_downloaded
      currentChannel.value = msg.payload.current_channel || '-'
    } else if (msg.type === 'scan_result') {
      messages.value = msg.payload.messages || []
      scannedAt.value = msg.payload.scanned_at || '-'
    }
  })

  getDownloadList().then((data: any) => {
    messages.value = data.messages || []
    if (data.scanned_at) scannedAt.value = data.scanned_at
  })
}

async function doQuickTest() {
  await quickTest()
}

async function doScan() {
  await scanChannels()
}

async function doStop() {
  await stopDownload()
  getDownloadList().then((d: any) => { messages.value = d.messages || [] })
}

async function doStartDownload() {
  await startDownload()
}

async function doDownloadSingle(ch: string, msgId: number) {
  await downloadSingle(ch, msgId)
  const d = await getDownloadList()
  messages.value = d.messages || []
}

function toggleAll(select: boolean) {
  selectedSet.value = {}
  if (select) {
    messages.value.forEach(m => {
      if (!m.exists) selectedSet.value[`${m.channel}_${m.msg_id}`] = true
    })
  }
}

function toggleSelect(m: ScannedFile) {
  const k = `${m.channel}_${m.msg_id}`
  selectedSet.value[k] = !selectedSet.value[k]
}

async function downloadSelected() {
  const keys = Object.keys(selectedSet.value).filter(k => selectedSet.value[k])
  for (const k of keys) {
    const [ch, id] = k.split('_')
    await downloadSingle(ch, parseInt(id))
    await pollUntilDone()
  }
  selectedSet.value = {}
  const d = await getDownloadList()
  messages.value = d.messages || []
}

function pollUntilDone(): Promise<void> {
  return new Promise(resolve => {
    const check = setInterval(async () => {
      const s = await getDownloadStatus()
      if (!s.running) { clearInterval(check); resolve() }
    }, 1500)
  })
}

function formatSize(bytes: number): string {
  if (!bytes) return '0B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0, s = bytes
  while (s >= 1024 && i < units.length - 1) { s /= 1024; i++ }
  return s.toFixed(1) + units[i]
}

const filteredMessages = ref<ScannedFile[]>([])

function applyFilters() {
  let result = messages.value
  if (filterExists.value) result = result.filter(m => !m.exists)
  if (searchInput.value) {
    const term = searchInput.value.toLowerCase()
    result = result.filter(m => m.file_name.toLowerCase().includes(term))
  }
  filteredMessages.value = result
}

onMounted(() => {
  if (getAdminPassword()) {
    getDownloadStatus().then(initPage).catch(() => { showLogin.value = true })
  }
  applyFilters()
})

onUnmounted(() => { disconnect() })
</script>

<template>
  <div>
    <div v-if="showLogin" class="login-overlay">
      <div class="login-card">
        <h2>管理登录</h2>
        <div class="form-group">
          <label>密码</label>
          <input type="password" v-model="loginPassword" placeholder="输入管理密码"
            @keydown.enter="doLogin" />
        </div>
        <div class="login-error">{{ loginError }}</div>
        <button class="btn btn-primary" style="width:100%" @click="doLogin">登录</button>
      </div>
    </div>

    <div v-if="!showLogin">
      <!-- Control Card -->
      <div class="card">
        <h2>下载控制</h2>
        <div class="form-actions" style="flex-wrap:wrap">
          <button class="btn btn-info" @click="doQuickTest">快速测试</button>
          <button class="btn btn-success" @click="doScan">扫描文件列表</button>
          <button class="btn btn-secondary" @click="toggleAll(true)">全选未下载</button>
          <button class="btn btn-secondary" @click="toggleAll(false)">取消全选</button>
          <button class="btn btn-primary" @click="downloadSelected">下载选中</button>
          <button class="btn btn-success" @click="doStartDownload">全部下载</button>
        </div>
        <div style="font-size:0.8rem;color:#888;margin-top:8px">
          快速测试：自动下载第一个频道的第一条音频，用于验证功能
        </div>
      </div>

      <!-- Status Card -->
      <div class="card">
        <h2>下载状态</h2>
        <div class="status-bar">
          <span class="badge" :class="running ? 'running' : 'idle'">{{ running ? '运行中' : '空闲' }}</span>
          <span class="status-text">{{ currentChannel }}</span>
        </div>
        <div class="status-details">
          <div><strong>已下载:</strong> {{ totalDownloaded }} 个文件</div>
          <div><strong>开始时间:</strong> {{ startedAt }}</div>
          <div><strong>扫描时间:</strong> {{ scannedAt }}</div>
        </div>
        <div class="form-actions">
          <button class="btn btn-danger" :disabled="!running" @click="doStop">停止下载</button>
        </div>
      </div>

      <!-- File List -->
      <div class="card">
        <h2>文件列表 <span>({{ filteredMessages.length }}/{{ messages.length }})</span></h2>
        <div class="filter-bar">
          <label><input type="checkbox" v-model="filterExists" @change="applyFilters"> 隐藏已下载</label>
          <input type="text" v-model="searchInput" placeholder="搜索文件名..." @input="applyFilters" />
        </div>
        <div class="file-list">
          <div v-if="filteredMessages.length === 0" class="empty-tip">暂无文件，请先扫描</div>
          <div v-for="m in filteredMessages" :key="m.msg_id"
            class="file-item" :class="{ 'file-exists': m.exists }">
            <label class="file-checkbox">
              <input type="checkbox" :checked="selectedSet[`${m.channel}_${m.msg_id}`]"
                :disabled="m.exists" @change="toggleSelect(m)" />
            </label>
            <div class="file-info">
              <div class="file-name">{{ m.file_name }}</div>
              <div class="file-meta">{{ m.channel_title }} | {{ formatSize(m.file_size) }} {{ m.exists ? '[已下载]' : '' }}</div>
            </div>
            <button v-if="!m.exists" class="btn btn-sm btn-primary"
              @click="doDownloadSingle(m.channel, m.msg_id)">下载</button>
            <span v-else class="badge badge-done">已完成</span>
          </div>
        </div>
      </div>

      <!-- Logs -->
      <div class="card">
        <h2>下载日志</h2>
        <div class="logs-container">
          <div v-for="(l, i) in logs" :key="i" class="log-line">
            <span class="log-time">[{{ l.time }}] </span>{{ l.message }}
          </div>
        </div>
      </div>
    </div>

    <div class="toast" :class="{ show: loginError }">{{ loginError }}</div>
  </div>
</template>
```

- [ ] **Step 3: Build and verify**

```bash
cd web && npx vite build
```

Expected: builds successfully, output in `web/dist/`.

- [ ] **Step 4: Commit**

```bash
git add web/src/api/ws.ts web/src/views/DownloadView.vue
git commit -m "feat: add DownloadView with WebSocket real-time updates"
```

---

### Task 9: Makefile + Docker + Final Build

**Files:**
- Create: `Makefile`
- Create: `.gitignore` (update for Go project)

- [ ] **Step 1: Create Makefile**

Create `Makefile`:

```makefile
.PHONY: build dev clean

build:
	cd web && npm install && npx vite build
	go build -o tg-music ./cmd/server/

dev:
	@echo "Start Go server on :8080 first, then run: cd web && npm run dev"

clean:
	rm -rf tg-music web/dist
```

- [ ] **Step 2: Update .gitignore**

Create `.gitignore`:

```
# Go
*.exe
tg-music
sessions/
downloads/
config.json

# Node
web/node_modules/
web/dist/

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
```

- [ ] **Step 3: Full build test**

```bash
make build
./tg-music
```

Expected: server starts, prints "Starting Telegram Music Manager: http://localhost:8080", serves the Vue SPA at `http://localhost:8080`.

- [ ] **Step 4: Final commit**

```bash
git add Makefile .gitignore
git commit -m "feat: add Makefile and .gitignore for Go + Vue project"
```

---
