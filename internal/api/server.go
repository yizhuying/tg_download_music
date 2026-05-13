package api

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"sync"
	"telegram-music/internal/config"
	"telegram-music/internal/download"
	"telegram-music/internal/telegram"

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
	runningCtx      context.Context
	cancelRunning   context.CancelFunc
	logger          *zap.Logger
}

// NewServer creates a new Server with all subcomponents initialized.
func NewServer(cfg *config.Manager, webFS fs.FS) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	logger, _ := zap.NewProduction()
	dlState := download.NewDownloadState()

	s := &Server{
		router:        r,
		config:        cfg,
		webFS:         webFS,
		downloadState: dlState,
		logger:        logger,
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
	api.Use(authMiddleware(s.config))

	api.GET("/config", s.getConfig)
	api.POST("/config", s.saveConfig)
	api.POST("/config/proxy/test", s.testProxy)

	// Auth routes
	api.GET("/auth/status", s.getAuthStatus)
	api.POST("/auth/send_code", s.sendCode)
	api.POST("/auth/sign_in", s.signIn)
	api.POST("/auth/logout", s.logout)

	// Download routes
	api.GET("/download/status", s.getDownloadStatus)
	api.POST("/download/start", s.startDownload)
	api.POST("/download/stop", s.stopDownload)
	api.POST("/download/scan", s.scanChannels)
	api.GET("/download/list", s.getDownloadList)
	api.POST("/download/single", s.downloadSingle)
	api.POST("/download/quick_test", s.quickTest)
	api.GET("/download/dirs", s.listDirs)

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
