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
	APIID             int      `json:"api_id"`
	APIHash           string   `json:"api_hash"`
	SessionName       string   `json:"session_name"`
	Channels          []string `json:"channels"`
	Proxy             Proxy    `json:"proxy"`
	DownloadDir       string   `json:"download_dir"`
	SessionDir        string   `json:"session_dir"`
	PhoneNumber       string   `json:"phone_number"`
	DownloadTimeStart string   `json:"download_time_start"` // "HH:mm" 允许下载的开始时间
	DownloadTimeEnd   string   `json:"download_time_end"`   // "HH:mm" 允许下载的结束时间
	AudioFormats      []string `json:"audio_formats"`       // 允许下载的音频格式（小写扩展名），空表示全部
}

func Defaults() Config {
	return Config{
		SessionName:  "my_session",
		Channels:     []string{},
		Proxy:        Proxy{Scheme: "socks5"},
		DownloadDir:  "",
		SessionDir:   "./sessions",
		AudioFormats: []string{},
	}
}

type Manager struct {
	mu      sync.RWMutex
	cfg     Config
	path    string
	modTime time.Time

	wmu      sync.Mutex
	writable map[string]writableEntry
}

// writableEntry caches a directory writability check with its check time.
type writableEntry struct {
	ok bool
	at time.Time
}

// writableCacheTTL limits how long a writability result is reused, so the
// check does not hit the disk on every Get() call.
const writableCacheTTL = 30 * time.Second

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

func (m *Manager) GetDownloadTimeRange() (string, string) {
	c := m.Get()
	return c.DownloadTimeStart, c.DownloadTimeEnd
}

// GetAudioFormats returns the allowed audio formats; empty means all.
func (m *Manager) GetAudioFormats() []string {
	c := m.Get()
	return c.AudioFormats
}

func (m *Manager) ConfigDir() string {
	return filepath.Dir(m.path)
}

func (m *Manager) Get() Config {
	m.reloadIfChanged()
	m.mu.RLock()
	c := m.cfg
	m.mu.RUnlock()
	if !m.isWritableCached(c.DownloadDir) {
		if paths := os.Getenv("TRIM_DATA_ACCESSIBLE_PATHS"); paths != "" {
			for _, p := range filepath.SplitList(paths) {
				if p != "" && m.isWritableCached(p) {
					c.DownloadDir = p
					break
				}
			}
		}
	}
	return c
}

// isWritableCached reports whether path is writable, caching results for
// writableCacheTTL to avoid disk probes on every configuration read.
func (m *Manager) isWritableCached(path string) bool {
	if path == "" {
		return false
	}

	m.wmu.Lock()
	if e, ok := m.writable[path]; ok && time.Since(e.at) < writableCacheTTL {
		m.wmu.Unlock()
		return e.ok
	}
	m.wmu.Unlock()

	ok := isWritable(path)

	m.wmu.Lock()
	if m.writable == nil {
		m.writable = make(map[string]writableEntry)
	}
	m.writable[path] = writableEntry{ok: ok, at: time.Now()}
	m.wmu.Unlock()
	return ok
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
	if err := os.WriteFile(m.path, data, 0o644); err != nil {
		return err
	}
	// Record the new mod time so reloadIfChanged does not re-read the file
	// we just wrote on the next Get().
	if info, err := os.Stat(m.path); err == nil {
		m.mu.Lock()
		m.modTime = info.ModTime()
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) reloadIfChanged() {
	info, err := os.Stat(m.path)
	if err != nil {
		return
	}

	m.mu.RLock()
	changed := info.ModTime().After(m.modTime)
	m.mu.RUnlock()
	if !changed {
		return
	}

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
