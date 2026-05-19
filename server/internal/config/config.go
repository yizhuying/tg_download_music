package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		Channels:    []string{},
		Proxy:       Proxy{Scheme: "socks5"},
		DownloadDir: "./downloads",
		SessionDir:  "./sessions",
	}
}

type Manager struct {
	mu            sync.RWMutex
	cfg           Config
	path          string
	modTime       time.Time
	accessPathMu  sync.RWMutex
	accessPathMod time.Time
	cachedPaths   string
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

func (m *Manager) GetChannels() ([]string, string) {
	c := m.Get()
	return c.Channels, c.DownloadDir
}

func (m *Manager) ConfigDir() string {
	return filepath.Dir(m.path)
}

func (m *Manager) Get() Config {
	m.reloadIfChanged()
	m.mu.RLock()
	defer m.mu.RUnlock()
	c := m.cfg

	dir := m.resolveDownloadDir(c.DownloadDir)
	if dir != "" {
		c.DownloadDir = dir
	}
	return c
}

func (m *Manager) AccessibleDirs() []string {
	paths := m.loadAccessiblePaths()
	if paths == "" {
		paths = os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS")
	}
	if paths == "" {
		paths = m.Get().DownloadDir
	}
	if paths == "" {
		return nil
	}

	var result []string
	for _, p := range strings.Split(paths, ":") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (m *Manager) resolveDownloadDir(cfgDir string) string {
	accessible := m.loadAccessiblePaths()
	if accessible != "" {
		first := strings.SplitN(accessible, ":", 2)[0]
		if isWritable(first) {
			return first
		}
	}

	if envPaths := os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS"); envPaths != "" {
		for _, p := range filepath.SplitList(envPaths) {
			if isWritable(p) {
				return p
			}
		}
	}

	if !isWritable(cfgDir) {
		return ""
	}
	return cfgDir
}

func (m *Manager) loadAccessiblePaths() string {
	pathsFile := filepath.Join(filepath.Dir(m.path), "accessible_paths")

	m.accessPathMu.RLock()
	cached, cachedMod := m.cachedPaths, m.accessPathMod
	m.accessPathMu.RUnlock()

	info, err := os.Stat(pathsFile)
	if err != nil {
		m.accessPathMu.Lock()
		m.cachedPaths = ""
		m.accessPathMu.Unlock()
		return ""
	}

	if !info.ModTime().After(cachedMod) {
		m.accessPathMu.RLock()
		result := m.cachedPaths
		m.accessPathMu.RUnlock()
		return result
	}

	data, err := os.ReadFile(pathsFile)
	if err != nil {
		return cached
	}
	content := strings.TrimSpace(string(data))

	m.accessPathMu.Lock()
	m.cachedPaths = content
	m.accessPathMod = info.ModTime()
	m.accessPathMu.Unlock()

	return content
}

func isWritable(path string) bool {
	if path == "" {
		return false
	}
	f, err := os.CreateTemp(path, ".writetest_*")
	if err != nil {
		return false
	}
	err = f.Close()
	if err != nil {
		return false
	}
	err = os.Remove(f.Name())
	if err != nil {
		return false
	}
	return true
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
