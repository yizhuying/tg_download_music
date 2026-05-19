package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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
		DownloadDir: "",
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

type DirOption struct {
	Path  string `json:"path"`
	Label string `json:"label"`
}

func (m *Manager) ResolveDebugInfo() map[string]string {
	accessible := m.loadAccessiblePaths()
	envPaths := os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS")
	m.mu.RLock()
	cfgDir := m.cfg.DownloadDir
	m.mu.RUnlock()
	return map[string]string{
		"cfg_download_dir": cfgDir,
		"accessible_paths": accessible,
		"env_accessible":   envPaths,
		"resolved":         m.Get().DownloadDir,
	}
}

func (m *Manager) AccessibleDirs() []DirOption {
	paths := m.loadAccessiblePaths()
	if paths == "" {
		paths = os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS")
	}

	var result []DirOption
	for _, p := range strings.Split(paths, ":") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, DirOption{Path: p, Label: dirLabel(p)})
		}
	}

	if len(result) == 0 {
		cfgDir := m.Get().DownloadDir
		if cfgDir != "" {
			result = append(result, DirOption{Path: cfgDir, Label: dirLabel(cfgDir)})
		} else {
			result = append(result, DirOption{Path: "", Label: "TuneGram/music"})
		}
	}

	return result
}

func dirLabel(path string) string {
	re := regexp.MustCompile(`^/vol\d+/@appshare/`)
	return re.ReplaceAllString(path, "")
}

func (m *Manager) resolveDownloadDir(cfgDir string) string {

	// User's explicit choice takes priority
	if cfgDir != "" && isWritable(cfgDir) {
		return cfgDir
	}

	// Auto-discover from accessible_paths file
	accessible := m.loadAccessiblePaths()
	if accessible != "" {
		first := strings.SplitN(accessible, ":", 2)[0]
		if isWritable(first) {
			return first
		}
	}

	// Env var
	if envPaths := os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS"); envPaths != "" {
		for _, p := range filepath.SplitList(envPaths) {
			if isWritable(p) {
				return p
			}
		}
	}

	// Shares directory
	shareDir := filepath.Join(filepath.Dir(m.path), "..", "shares")
	entries, err := os.ReadDir(shareDir)
	if err == nil {
		for _, e := range entries {
			p := filepath.Join(shareDir, e.Name())
			if isWritable(p) {
				return p
			}
		}
	}

	return ""
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
		// Fallback: merge env vars at runtime
		return mergePaths(os.Getenv("TRIM_DATA_SHARE_PATHS"), os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS"))
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

func mergePaths(sharePaths, accessiblePaths string) string {
	seen := make(map[string]bool)
	var result []string
	for _, p := range strings.Split(sharePaths, ":") {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	for _, p := range strings.Split(accessiblePaths, ":") {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return strings.Join(result, ":")
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
