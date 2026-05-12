package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Proxy struct {
	Scheme   string `json:"scheme"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	APIID       int      `json:"api_id"`
	APIHash     string   `json:"api_hash"`
	SessionName string   `json:"session_name"`
	Channels    []string `json:"channels"`
	Proxy       Proxy    `json:"proxy"`
	DownloadDir string   `json:"download_dir"`
	SessionDir  string   `json:"session_dir"`
	PhoneNumber string   `json:"phone_number"`
}

func Defaults() Config {
	return Config{
		SessionName: "my_session",
		Channels:    []string{"VmoMusic", "FLAC_HR", "cjCoolMusic"},
		Proxy:       Proxy{Scheme: "socks5"},
		DownloadDir: "./downloads",
		SessionDir:  "./sessions",
	}
}

type Manager struct {
	mu     sync.RWMutex
	cfg    Config
	path   string
	modTime time.Time
}

func NewManager(configPath string) (*Manager, error) {
	cfg := Defaults()
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	var modTime time.Time
	if info, err := os.Stat(configPath); err == nil {
		modTime = info.ModTime()
	}

	return &Manager{cfg: cfg, path: configPath, modTime: modTime}, nil
}

func (m *Manager) Get() Config {
	m.reloadIfChanged()
	m.mu.RLock()
	defer m.mu.RUnlock()
	c := m.cfg
	return c
}

func (m *Manager) Save(c Config) error {
	m.mu.Lock()
	m.cfg = c
	m.mu.Unlock()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(m.path)
	_ = os.MkdirAll(dir, 0o755)
	return os.WriteFile(m.path, data, 0o644)
}

func (m *Manager) reloadIfChanged() {
	info, err := os.Stat(m.path)
	if err != nil {
		return
	}

	if info.ModTime().After(m.modTime) {
		data, err := os.ReadFile(m.path)
		if err != nil {
			return
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return
		}

		m.mu.Lock()
		m.cfg = cfg
		m.modTime = info.ModTime()
		m.mu.Unlock()
	}
}
