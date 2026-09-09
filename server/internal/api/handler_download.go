package api

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yizhuying/tg-music/internal/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()

	// Default entries: the app's own data-share dirs (TuneGram/music). They
	// never appear in the shared-access API result and are only delivered
	// via TRIM_DATA_SHARE_PATHS.
	paths := shareDirs()

	// Append admin-authorized folders; when the fnOS API is unavailable the
	// list degrades to the defaults instead of failing.
	apiPaths, err := s.trimClient.GetSharedAccessibleFolders(ctx)
	if err != nil {
		s.logger.Warn("trimapp getSharedAccessibleFolders failed", zap.Error(err))
	} else {
		s.logger.Info("authorized dirs from fnOS API", zap.Strings("paths", apiPaths))
		paths = appendUnique(paths, apiPaths)
	}
	s.logger.Info("final dir list", zap.Strings("paths", paths))

	if len(paths) == 0 {
		response.OK(c, []gin.H{})
		return
	}

	labels := map[string]string{}
	if converted, err := s.trimClient.ConvertPaths(ctx, paths, "zh-CN"); err != nil {
		s.logger.Warn("trimapp convertPath failed", zap.Error(err))
	} else {
		labels = converted
	}

	var result []gin.H
	for _, p := range paths {
		if p == "" {
			continue
		}
		label := labels[p]
		if label == "" {
			label = p
		}
		// The app's own music share keeps its short "TuneGram/music" name
		// instead of the semantic path.
		if short := shortenTuneGram(p); short != p {
			label = short
		}
		result = append(result, gin.H{
			"label": label,
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

// shareDirs lists the app data-share dirs from TRIM_DATA_SHARE_PATHS,
// skipping the logs share.
func shareDirs() []string {
	var shares []string
	for _, p := range filepath.SplitList(os.Getenv("TRIM_DATA_SHARE_PATHS")) {
		if p == "" || filepath.Base(p) == "logs" {
			continue
		}
		shares = append(shares, p)
	}
	return shares
}

// appendUnique appends extra paths to base, skipping duplicates (ignoring
// trailing slashes, which fnOS is not consistent about).
func appendUnique(base, extra []string) []string {
	seen := make(map[string]struct{}, len(base)+len(extra))
	normalize := func(p string) string { return strings.TrimRight(p, "/") }
	for _, p := range base {
		seen[normalize(p)] = struct{}{}
	}
	for _, p := range extra {
		if p == "" {
			continue
		}
		if _, ok := seen[normalize(p)]; ok {
			continue
		}
		seen[normalize(p)] = struct{}{}
		base = append(base, p)
	}
	return base
}
