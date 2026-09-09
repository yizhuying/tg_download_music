package download

import (
	"sync"
	"time"
)

// LogEntry represents a single log line in the download state.
type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

// State represents the current download manager state exposed via API.
type State struct {
	Running         bool       `json:"running"`
	TotalDownloaded int        `json:"total_downloaded"`
	CurrentChannel  string     `json:"current_channel"`
	StartedAt       string     `json:"started_at"`
	Logs            []LogEntry `json:"logs"`
}

// ScannedMessage represents an audio message found during channel scanning.
type ScannedMessage struct {
	Channel      string `json:"channel"`
	ChannelTitle string `json:"channel_title"`
	MsgID        int    `json:"msg_id"`
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	Exists       bool   `json:"exists"`
}

// DownloadState holds thread-safe download state and scanned messages.
type DownloadState struct {
	mu          sync.RWMutex
	state       State
	messages    []ScannedMessage
	scannedAt   string
	broadcaster func(typ string, data interface{})
}

// NewDownloadState creates a new DownloadState with initialized logs slice.
func NewDownloadState() *DownloadState {
	return &DownloadState{
		state: State{Logs: make([]LogEntry, 0, 100)},
	}
}

// Get returns a snapshot copy of the current state.
func (ds *DownloadState) Get() State {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.snapshotLocked()
}

func (ds *DownloadState) snapshotLocked() State {
	s := ds.state
	s.Logs = make([]LogEntry, len(ds.state.Logs))
	copy(s.Logs, ds.state.Logs)
	return s
}

// SetRunning updates the running flag and optionally the current channel,
// and broadcasts the state change via WebSocket.
func (ds *DownloadState) SetRunning(running bool, channel string) {
	ds.mu.Lock()
	ds.state.Running = running
	ds.state.CurrentChannel = channel
	if running {
		ds.state.StartedAt = time.Now().Format("2006-01-02 15:04:05")
		ds.state.TotalDownloaded = 0
		ds.state.Logs = make([]LogEntry, 0, 100)
	}
	st := ds.snapshotLocked()
	bc := ds.broadcaster
	ds.mu.Unlock()

	if bc != nil {
		bc("download_status", st)
	}
}

// slimStatus is the incremental download progress broadcast. It omits the
// log history so per-file updates stay small during batch downloads.
type slimStatus struct {
	Running         bool   `json:"running"`
	TotalDownloaded int    `json:"total_downloaded"`
	CurrentChannel  string `json:"current_channel"`
}

// IncrementDownloaded atomically increases the total download counter
// and broadcasts a slim progress update via WebSocket.
func (ds *DownloadState) IncrementDownloaded() {
	ds.mu.Lock()
	ds.state.TotalDownloaded++
	st := slimStatus{
		Running:         ds.state.Running,
		TotalDownloaded: ds.state.TotalDownloaded,
		CurrentChannel:  ds.state.CurrentChannel,
	}
	bc := ds.broadcaster
	ds.mu.Unlock()

	if bc != nil {
		bc("download_status", st)
	}
}

// SetBroadcaster sets an optional callback for broadcasting log entries.
func (ds *DownloadState) SetBroadcaster(fn func(typ string, data interface{})) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.broadcaster = fn
}

// AddLog appends a timestamped log entry and returns it.
func (ds *DownloadState) AddLog(msg string) LogEntry {
	ds.mu.Lock()
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Message: msg,
	}
	ds.state.Logs = append(ds.state.Logs, entry)
	if len(ds.state.Logs) > 500 {
		ds.state.Logs = ds.state.Logs[len(ds.state.Logs)-500:]
	}
	bc := ds.broadcaster
	ds.mu.Unlock()

	if bc != nil {
		bc("log", map[string]string{
			"time":    entry.Time,
			"message": entry.Message,
		})
	}
	return entry
}

// Complete logs a task-level completion message and additionally broadcasts
// a download_complete event so the UI can surface a toast, not just a log line.
func (ds *DownloadState) Complete(msg string) {
	ds.AddLog(msg)
	ds.mu.RLock()
	bc := ds.broadcaster
	ds.mu.RUnlock()
	if bc != nil {
		bc("download_complete", map[string]string{"message": msg})
	}
}

// SetMessages replaces the scanned messages list.
func (ds *DownloadState) SetMessages(msgs []ScannedMessage) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.messages = msgs
	ds.scannedAt = time.Now().Format("2006-01-02 15:04:05")
}

// GetMessages returns a copy of the scanned messages and the scan timestamp.
func (ds *DownloadState) GetMessages() ([]ScannedMessage, string) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	msgs := make([]ScannedMessage, len(ds.messages))
	copy(msgs, ds.messages)
	return msgs, ds.scannedAt
}

// MarkDownloaded sets Exists=true for a specific channel+message combination
// and broadcasts a per-file update so clients can patch a single list entry
// instead of resending the whole scanned list.
func (ds *DownloadState) MarkDownloaded(channel string, msgID int) {
	ds.mu.Lock()
	for i := range ds.messages {
		if ds.messages[i].Channel == channel && ds.messages[i].MsgID == msgID {
			ds.messages[i].Exists = true
			break
		}
	}
	bc := ds.broadcaster
	ds.mu.Unlock()

	if bc != nil {
		bc("file_downloaded", map[string]interface{}{
			"channel": channel,
			"msg_id":  msgID,
		})
	}
}
