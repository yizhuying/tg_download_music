package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Proxy struct {
	Scheme   string `json:"scheme"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	APIID       int        `json:"api_id"`
	APIHash     string     `json:"api_hash"`
	SessionName string     `json:"session_name"`
	Channels    []string   `json:"channels"`
	Proxy       Proxy      `json:"proxy"`
	DownloadDir string     `json:"download_dir"`
	SessionDir  string     `json:"session_dir"`
	PhoneNumber string     `json:"phone_number"`
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
	mu   sync.RWMutex
	cfg  Config
	path string
}

func NewManager(configPath string) (*Manager, error) {
	cfg := Defaults()
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	return &Manager{cfg: cfg, path: configPath}, nil
}

func (m *Manager) Get() Config {
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
