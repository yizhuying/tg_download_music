package api

import (
	"net/http"
	"telegram-music/internal/config"

	"github.com/gin-gonic/gin"
)

func (s *Server) getConfig(c *gin.Context) {
	cfg := s.config.Get()
	c.JSON(http.StatusOK, cfg)
}

func (s *Server) saveConfig(c *gin.Context) {
	var req struct {
		APIID       int      `json:"api_id"`
		APIHash     string   `json:"api_hash"`
		SessionName string   `json:"session_name"`
		Channels    []string `json:"channels"`
		Proxy       config.Proxy `json:"proxy"`
		DownloadDir string   `json:"download_dir"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data"})
		return
	}

	existing := s.config.Get()
	sessionDir := existing.SessionDir
	if sessionDir == "" {
		sessionDir = config.Defaults().SessionDir
	}
	cfg := config.Config{
		APIID:       req.APIID,
		APIHash:     req.APIHash,
		SessionName: req.SessionName,
		Channels:    req.Channels,
		Proxy:       req.Proxy,
		DownloadDir: req.DownloadDir,
		SessionDir:  sessionDir,
		PhoneNumber: existing.PhoneNumber,
	}
	if err := s.config.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "配置已保存"})
}
