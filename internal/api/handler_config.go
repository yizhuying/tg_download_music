package api

import (
	"fmt"
	"net/http"

	"golang.org/x/net/proxy"

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

func (s *Server) testProxy(c *gin.Context) {
	cfg := s.config.Get()
	p := cfg.Proxy
	if p.Scheme == "none" || p.Hostname == "" || p.Port == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "代理未配置"})
		return
	}

	addr := fmt.Sprintf("%s:%d", p.Hostname, p.Port)
	var auth *proxy.Auth
	if p.Username != "" {
		auth = &proxy.Auth{User: p.Username, Password: p.Password}
	}

	dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": fmt.Sprintf("代理配置错误: %s", err.Error())})
		return
	}

	conn, err := dialer.Dial("tcp", "api.telegram.org:443")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": fmt.Sprintf("连接失败: %s", err.Error())})
		return
	}
	defer conn.Close()

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": fmt.Sprintf("代理 %s 可用，已连通 Telegram", addr)})
}
