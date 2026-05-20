package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yizhuying/tg-music/internal/api"
	"github.com/yizhuying/tg-music/internal/config"
)

//go:embed dist/*
var distFS embed.FS

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	port := flag.Int("port", 45116, "server listen port")
	configPath := flag.String("config", "config.json", "config file path")
	downloadDir := flag.String("download-dir", "", "override download directory")
	sessionDir := flag.String("session-dir", "", "override session directory")
	socketPath := flag.String("socket", "", "unix socket path for gateway mode")
	adminPassword := flag.String("admin-password", "", "set admin password")
	flag.Parse()

	if *adminPassword != "" {
		_ = os.Setenv("ADMIN_PASSWORD", *adminPassword)
	}

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

	webFS, _ := fs.Sub(distFS, "dist")

	srv := api.NewServer(cfg, webFS, version, buildTime)

	// Listen on HTTP port
	go func() {
		log.Printf("Starting Telegram Music Manager: http://localhost%s\n", listenAddr)
		if err := srv.Run(listenAddr); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Also listen on Unix socket if specified (for gateway mode)
	if *socketPath != "" {
		_ = os.Remove(*socketPath)
		//log.Printf("Also listening on unix socket: %s\n", *socketPath)
		go func() {
			if err := srv.RunUnix(*socketPath); err != nil {
				log.Fatalf("Unix socket server failed: %v", err)
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down...")
}
