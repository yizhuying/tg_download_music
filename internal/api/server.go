package api

import (
	"embed"
	"io/fs"
	"net/http"
	"telegram-music/internal/config"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var distFS embed.FS

type Server struct {
	router *gin.Engine
	config *config.Manager
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
