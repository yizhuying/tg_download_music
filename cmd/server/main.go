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
	"telegram-music/internal/api"
	"telegram-music/internal/config"
)

//go:embed dist/*
var distFS embed.FS

func main() {
	port := flag.Int("port", 8080, "server listen port")
	configPath := flag.String("config", "config.json", "config file path")
	downloadDir := flag.String("download-dir", "", "override download directory")
	sessionDir := flag.String("session-dir", "", "override session directory")
	socketPath := flag.String("socket", "", "unix socket path for gateway mode")
	flag.Parse()
	// todo 增加返回授权文件夹列表接口
	// todo 增加按照时密码设置
	// todo 增加代理验证

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

	srv := api.NewServer(cfg, webFS)

	// Listen on HTTP port
	go func() {
		log.Printf("Starting Telegram Music Manager: http://localhost%s\n", listenAddr)
		if err := srv.Run(listenAddr); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Also listen on Unix socket if specified (for gateway mode)
	if *socketPath != "" {
		os.Remove(*socketPath)
		log.Printf("Also listening on unix socket: %s\n", *socketPath)
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
