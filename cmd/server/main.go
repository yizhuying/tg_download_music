package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"telegram-music/internal/api"
	"telegram-music/internal/config"
)

//go:embed dist/*
var distFS embed.FS

func main() {
	cfg, err := config.NewManager("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	webFS, _ := fs.Sub(distFS, "dist")

	srv := api.NewServer(cfg, webFS)
	log.Printf("Starting Telegram Music Manager: http://localhost%s\n", addr)

	if err := srv.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
