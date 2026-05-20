package api

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/yizhuying/tg-music/internal/response"

	"github.com/gin-gonic/gin"
)

func (s *Server) getDownloadStatus(c *gin.Context) {
	state := s.downloadState.Get()
	response.OK(c, gin.H{
		"running":          state.Running,
		"total_downloaded": state.TotalDownloaded,
		"current_channel":  state.CurrentChannel,
		"started_at":       state.StartedAt,
		"logs":             state.Logs,
	})
}

func (s *Server) startDownload(c *gin.Context) {
	cfg := s.config.Get()
	if len(cfg.Channels) == 0 {
		response.BadRequest(c, "没有配置频道")
		return
	}

	if err := s.downloadManager.Start(context.Background()); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "下载任务已启动")
}

func (s *Server) stopDownload(c *gin.Context) {
	if err := s.downloadManager.Stop(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	s.downloadState.AddLog("下载任务已手动停止")
	s.downloadState.SetRunning(false, "")
	response.OKMsg(c, "下载任务已停止")
}

func (s *Server) scanChannels(c *gin.Context) {
	cfg := s.config.Get()
	if err := s.downloadManager.ScanChannels(context.Background(), cfg.Channels, cfg.DownloadDir); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "扫描已开始")
}

func (s *Server) getDownloadList(c *gin.Context) {
	msgs, scannedAt := s.downloadState.GetMessages()
	state := s.downloadState.Get()
	response.OK(c, gin.H{
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
		response.BadRequest(c, "missing channel or msg_id")
		return
	}

	cfg := s.config.Get()
	if err := s.downloadManager.DownloadSingle(context.Background(), req.Channel, cfg.DownloadDir, req.MsgID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "开始下载: "+req.Channel)
}

func (s *Server) quickTest(c *gin.Context) {
	cfg := s.config.Get()
	if len(cfg.Channels) == 0 {
		response.BadRequest(c, "没有配置频道")
		return
	}

	if err := s.downloadManager.QuickTest(context.Background(), cfg.Channels[0], cfg.DownloadDir); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "快速测试已启动")
}

func (s *Server) listDirs(c *gin.Context) {
	paths := ""

	pathsFile := filepath.Join(s.config.ConfigDir(), "accessible_paths")
	if data, err := os.ReadFile(pathsFile); err == nil {
		paths = strings.TrimSpace(string(data))
	}

	if paths == "" {
		paths = os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS")
	}

	if paths == "" {
		cfg := s.config.Get()
		paths = cfg.DownloadDir
	}

	if paths == "" {
		response.OK(c, []gin.H{})
		return
	}

	var result []gin.H
	for _, p := range strings.Split(paths, ":") {
		if p == "" {
			continue
		}
		result = append(result, gin.H{
			"label": shortenTuneGram(p),
			"value": p,
		})
	}

	response.OK(c, result)
}

func shortenTuneGram(p string) string {
	if strings.HasSuffix(strings.TrimRight(p, "/"), "/TuneGram/music") {
		return "TuneGram/music"
	}
	return p
}
