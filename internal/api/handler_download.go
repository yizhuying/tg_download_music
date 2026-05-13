package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) getDownloadStatus(c *gin.Context) {
	state := s.downloadState.Get()
	c.JSON(http.StatusOK, state)
}

func (s *Server) startDownload(c *gin.Context) {
	if err := s.ensureTelegramClient(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
	if err := s.ensureTelegramClient(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := s.downloadManager.Stop(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.downloadState.AddLog("下载任务已手动停止")
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "下载任务已停止"})
}

func (s *Server) scanChannels(c *gin.Context) {
	if err := s.ensureTelegramClient(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
	if err := s.ensureTelegramClient(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
	if err := s.ensureTelegramClient(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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

func (s *Server) getVolumes(c *gin.Context) {
	volumes := []string{}
	for i := 1; i <= 10; i++ {
		p := "/vol" + strconv.Itoa(i)
		if _, err := os.Stat(p); err == nil {
			volumes = append(volumes, p)
		}
	}
	if len(volumes) == 0 {
		volumes = append(volumes, "/vol1")
	}
	c.JSON(http.StatusOK, gin.H{"volumes": volumes})
}

func (s *Server) listDirs(c *gin.Context) {
	reqPath := c.Query("path")
	if reqPath == "" {
		vols := []string{}
		for i := 1; i <= 10; i++ {
			p := "/vol" + strconv.Itoa(i)
			if _, err := os.Stat(p); err == nil {
				vols = append(vols, p)
			}
		}
		if len(vols) == 0 {
			vols = []string{"/vol1"}
		}
		c.JSON(http.StatusOK, gin.H{"parent": "/", "dirs": volsToDirs(vols)})
		return
	}

	entries, err := os.ReadDir(reqPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"parent": parentOf(reqPath), "dirs": []gin.H{}})
		return
	}

	dirs := []gin.H{}
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, gin.H{
				"name": e.Name(),
				"path": filepath.Join(reqPath, e.Name()),
			})
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i]["name"].(string) < dirs[j]["name"].(string)
	})

	c.JSON(http.StatusOK, gin.H{"parent": parentOf(reqPath), "dirs": dirs})
}

func parentOf(p string) string {
	parent := filepath.Dir(p)
	if parent == p {
		return "/"
	}
	return parent
}

func volsToDirs(vols []string) []gin.H {
	dirs := []gin.H{}
	for _, v := range vols {
		dirs = append(dirs, gin.H{"name": v, "path": v})
	}
	return dirs
}
