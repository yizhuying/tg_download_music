package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/yizhuying/tg-music/internal/api"
	"github.com/yizhuying/tg-music/internal/config"
	"github.com/yizhuying/tg-music/internal/logging"

	"go.uber.org/zap"
)

//go:embed dist/*
var distFS embed.FS

var (
	version   = "dev"
	buildTime = "unknown"
)

// resolveLogsDir picks the log directory: an explicit -logs-dir flag first,
// then a TuneGram/logs data share declared in the fnOS package, then a logs
// sibling next to the first data share (.../TuneGram/music -> .../TuneGram/logs),
// and finally a logs dir next to the config file for non-fnOS runs.
func resolveLogsDir(flagDir, configPath string) []string {
	if flagDir != "" {
		return []string{flagDir}
	}
	var candidates []string
	for _, p := range filepath.SplitList(os.Getenv("TRIM_DATA_SHARE_PATHS")) {
		if p == "" {
			continue
		}
		if filepath.Base(p) == "logs" {
			return []string{p}
		}
		if len(candidates) == 0 {
			candidates = append(candidates, filepath.Join(filepath.Dir(p), "logs"))
		}
	}
	return append(candidates, filepath.Join(filepath.Dir(configPath), "logs"))
}

func initLogger(flagDir, configPath string) *zap.Logger {
	for _, dir := range resolveLogsDir(flagDir, configPath) {
		logger, err := logging.New(dir)
		if err == nil {
			log.Printf("Logging to %s", dir)
			return logger
		}
		log.Printf("Logs dir %s unavailable: %v", dir, err)
	}
	log.Printf("No usable logs dir, logging to stderr only")
	return logging.NewStderr()
}

func main() {
	port := flag.Int("port", 45116, "server listen port")
	configPath := flag.String("config", "config.json", "config file path")
	downloadDir := flag.String("download-dir", "", "override download directory")
	sessionDir := flag.String("session-dir", "", "override session directory")
	socketPath := flag.String("socket", "", "unix socket path for gateway mode")
	logsDir := flag.String("logs-dir", "", "log directory (default: next to the fnOS data share)")
	flag.Parse()

	listenAddr := fmt.Sprintf(":%d", *port)
	cfg, err := config.NewManager(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *downloadDir != "" {
		c := cfg.Get()
		c.DownloadDir = *downloadDir
		_ = cfg.Save(c)
	}
	if *sessionDir != "" {
		c := cfg.Get()
		c.SessionDir = *sessionDir
		_ = cfg.Save(c)
	}

	logger := initLogger(*logsDir, *configPath)
	defer func() { _ = logger.Sync() }()

	webFS, _ := fs.Sub(distFS, "dist")

	srv := api.NewServer(cfg, webFS, version, buildTime, logger)

	logger.Info("starting Telegram Music Manager",
		zap.String("version", version),
		zap.String("build_time", buildTime),
		zap.String("addr", listenAddr),
		zap.String("config", *configPath),
	)

	// Listen on HTTP port
	go func() {
		if err := srv.Run(listenAddr); err != nil {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Also listen on Unix socket if specified (for gateway mode)
	if *socketPath != "" {
		_ = os.Remove(*socketPath)
		go func() {
			if err := srv.RunUnix(*socketPath); err != nil {
				logger.Fatal("unix socket server failed", zap.Error(err))
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down")
}
