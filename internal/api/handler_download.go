package api

import (
	"context"
	"net/http"

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

	if err := s.downloadManager.Start(context.Background(), cfg.Channels, cfg.DownloadDir); err != nil {
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
	s.downloadState.AddLog("下载任务已手动停止")
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "下载任务已停止"})
}

func (s *Server) scanChannels(c *gin.Context) {
	cfg := s.config.Get()
	if err := s.downloadManager.ScanChannels(context.Background(), cfg.Channels, cfg.DownloadDir); err != nil {
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
