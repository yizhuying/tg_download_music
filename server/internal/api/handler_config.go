package api

import (
	"fmt"
	"net"

	"go.uber.org/zap"
	"golang.org/x/net/proxy"

	"github.com/yizhuying/tg-music/internal/config"
	"github.com/yizhuying/tg-music/internal/response"

	"github.com/gin-gonic/gin"
)

func (s *Server) getConfig(c *gin.Context) {
	cfg := s.config.Get()
	response.OK(c, cfg)
}

func (s *Server) saveConfig(c *gin.Context) {
	var req struct {
		APIID             int          `json:"api_id"`
		APIHash           string       `json:"api_hash"`
		SessionName       string       `json:"session_name"`
		Channels          []string     `json:"channels"`
		Proxy             config.Proxy `json:"proxy"`
		DownloadDir       string       `json:"download_dir"`
		DownloadTimeStart string       `json:"download_time_start"`
		DownloadTimeEnd   string       `json:"download_time_end"`
		AudioFormats      []string     `json:"audio_formats"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid data")
		return
	}

	existing := s.config.Get()
	sessionDir := existing.SessionDir
	if sessionDir == "" {
		sessionDir = config.Defaults().SessionDir
	}
	cfg := config.Config{
		APIID:             req.APIID,
		APIHash:           req.APIHash,
		SessionName:       req.SessionName,
		Channels:          req.Channels,
		Proxy:             req.Proxy,
		DownloadDir:       req.DownloadDir,
		SessionDir:        sessionDir,
		PhoneNumber:       existing.PhoneNumber,
		DownloadTimeStart: req.DownloadTimeStart,
		DownloadTimeEnd:   req.DownloadTimeEnd,
		AudioFormats:      req.AudioFormats,
	}
	if err := s.config.Save(cfg); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OKMsg(c, "配置已保存")
}

func (s *Server) testProxy(c *gin.Context) {
	p := s.config.Get().Proxy
	if p.Scheme == "none" || p.Hostname == "" || p.Port == 0 {
		response.OKMsgData(c, "代理未配置", gin.H{"ok": false})
		return
	}

	addr := fmt.Sprintf("%s:%d", p.Hostname, p.Port)
	var auth *proxy.Auth
	if p.Username != "" {
		auth = &proxy.Auth{User: p.Username, Password: p.Password}
	}

	dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
	if err != nil {
		response.OKMsgData(c, fmt.Sprintf("代理配置错误: %s", err.Error()), gin.H{"ok": false})
		return
	}

	conn, err := dialer.Dial("tcp", "api.telegram.org:443")
	if err != nil {
		response.OKMsgData(c, fmt.Sprintf("连接失败: %s", err.Error()), gin.H{"ok": false})
		return
	}
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			s.logger.Warn("关闭连接失败", zap.Error(err))
		}
	}(conn)

	response.OKMsgData(c, fmt.Sprintf("代理 %s 可用，已连通 Telegram", addr), gin.H{"ok": true})
}
