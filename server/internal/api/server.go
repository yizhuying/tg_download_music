package api

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/yizhuying/tg-music/internal/config"
	"github.com/yizhuying/tg-music/internal/download"
	"github.com/yizhuying/tg-music/internal/telegram"
	"github.com/yizhuying/tg-music/internal/trimapp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Server holds the HTTP server, configuration, and runtime state.
type Server struct {
	router          *gin.Engine
	config          *config.Manager
	webFS           fs.FS
	downloadState   *download.DownloadState
	downloadManager *download.Manager
	hub             *Hub
	authState       AuthState
	authStateMu     sync.RWMutex
	tgClient        *telegram.Client
	trimClient      *trimapp.Client
	runningCtx      context.Context
	cancelRunning   context.CancelFunc
	logger          *zap.Logger
	version         string
	buildTime       string
}

// trimAppName must match appname in the fnOS package manifest; the open API
// gateway rejects requests for other app names.
const trimAppName = "TuneGram"

// NewServer creates a new Server with all subcomponents initialized.
func NewServer(cfg *config.Manager, webFS fs.FS, version, buildTime string, logger *zap.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	if logger == nil {
		logger = zap.NewNop()
	}
	dlState := download.NewDownloadState()

	s := &Server{
		router:        r,
		config:        cfg,
		webFS:         webFS,
		downloadState: dlState,
		trimClient:    trimapp.New(trimAppName),
		logger:        logger,
		version:       version,
		buildTime:     buildTime,
	}

	s.hub = NewHub(func() interface{} {
		return dlState.Get()
	})
	go s.hub.Run()

	s.registerRoutes()
	return s
}

// ensureTelegramClient lazily initializes the Telegram client and download manager.
func (s *Server) ensureTelegramClient() error {
	if s.tgClient != nil {
		return nil
	}
	cfg := s.config.Get()
	if cfg.APIID == 0 || cfg.APIHash == "" {
		return fmt.Errorf("telegram API未配置，请先在设置中填写相关信息")
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
	dlState := s.downloadState
	// Per-file download logs stay in the UI log panel only; mirroring them to
	// the log file made it grow too fast during batch downloads.
	dlState.SetBroadcaster(func(typ string, data interface{}) {
		s.hub.Broadcast(typ, data)
	})
	s.downloadManager = download.NewManager(client, dlState, s.hub, s.config)

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

	s.router.GET("/api/ws", s.hub.HandleWS)

	api := s.router.Group("/api")

	api.GET("/system/info", s.getSystemInfo)

	api.GET("/config", s.getConfig)
	api.POST("/config", s.saveConfig)
	api.POST("/config/proxy/test", s.testProxy)

	tgAuth := api.Group("/tg")
	tgAuth.GET("/auth/status", s.getAuthStatus)
	tgAuth.POST("/auth/send_code", s.initTelegramClient(), s.sendCode)
	tgAuth.POST("/auth/sign_in", s.requireTelegramClient(), s.signIn)
	tgAuth.POST("/auth/logout", s.logout)

	task := api.Group("/task")
	task.GET("/status", s.getDownloadStatus)
	task.GET("/list", s.getDownloadList)
	task.GET("/dirs", s.listDirs)
	task.Use(s.requireTelegramClient())
	task.POST("/start", s.startDownload)
	task.POST("/stop", s.stopDownload)
	task.POST("/scan", s.scanChannels)
	task.POST("/single", s.downloadSingle)
	task.POST("/quick_test", s.quickTest)

	// Serve static assets (for CGI / standalone mode)
	s.registerStaticRoutes()

	// SPA fallback for all other unmatched routes
	s.router.NoRoute(s.spaFallback())
}

func (s *Server) registerStaticRoutes() {
	indexHTML, err := fs.ReadFile(s.webFS, "index.html")
	if err != nil {
		indexHTML = []byte("<h1>Frontend not built</h1>")
	}

	s.router.GET("/assets/*filepath", func(c *gin.Context) {
		filepath := c.Param("filepath")
		if file, err := fs.ReadFile(s.webFS, "assets"+filepath); err == nil {
			c.Data(http.StatusOK, s.mimeFor(filepath), file)
		} else {
			c.Status(http.StatusNotFound)
		}
	})

	for _, name := range []string{"favicon.png", "icon-256.png"} {
		p := name
		s.router.GET("/"+p, func(c *gin.Context) {
			if file, err := fs.ReadFile(s.webFS, p); err == nil {
				c.Data(http.StatusOK, s.mimeFor(p), file)
			} else {
				c.Status(http.StatusNotFound)
			}
		})
	}

	s.router.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
}

func (s *Server) mimeFor(path string) string {
	switch {
	case len(path) > 3 && path[len(path)-3:] == ".js":
		return "application/javascript; charset=utf-8"
	case len(path) > 4 && path[len(path)-4:] == ".css":
		return "text/css; charset=utf-8"
	case len(path) > 4 && path[len(path)-4:] == ".png":
		return "image/png"
	case len(path) > 4 && path[len(path)-4:] == ".svg":
		return "image/svg+xml"
	case len(path) > 4 && path[len(path)-4:] == ".ico":
		return "image/x-icon"
	case len(path) > 4 && path[len(path)-4:] == ".woff":
		return "font/woff"
	case len(path) > 5 && path[len(path)-5:] == ".woff2":
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}

func (s *Server) spaFallback() gin.HandlerFunc {
	indexHTML, err := fs.ReadFile(s.webFS, "index.html")
	if err != nil {
		indexHTML = []byte("<h1>Frontend not built. Run: make build</h1>")
	}
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	}
}

// Run starts the HTTP server on the given address.
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// RunUnix starts the HTTP server on a Unix domain socket.
func (s *Server) RunUnix(socketPath string) error {
	_ = os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}

	if err := os.Chmod(socketPath, 0777); err != nil {
		return err
	}

	return http.Serve(ln, s.router)
}
