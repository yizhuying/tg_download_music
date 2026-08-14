package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gotd/td/tg"
)

// TelegramAPI defines the interface the manager needs from a Telegram client.
type TelegramAPI interface {
	Run(ctx context.Context, f func(ctx context.Context) error) error
	API() *tg.Client
	DownloadToFile(ctx context.Context, location tg.InputFileLocationClass, path string, w io.WriterAt) error
	WaitReady(ctx context.Context) error
}

// Broadcaster interface for WebSocket messages.
type Broadcaster interface {
	Broadcast(typ string, data interface{})
}

// ConfigProvider provides dynamic access to channel configuration.
type ConfigProvider interface {
	GetChannels() ([]string, string)
	ConfigDir() string
	GetDownloadTimeRange() (start, end string)
	GetAudioFormats() []string
}

// Manager orchestrates channel scanning and audio downloading.
type Manager struct {
	state      *DownloadState
	client     TelegramAPI
	hub        Broadcaster
	config     ConfigProvider
	record     *DownloadRecord
	mu         sync.Mutex
	cancel     context.CancelFunc
	scanCancel context.CancelFunc
}

// NewManager creates a new Manager.
func NewManager(client TelegramAPI, state *DownloadState, hub Broadcaster, config ConfigProvider) *Manager {
	return &Manager{
		client: client,
		state:  state,
		hub:    hub,
		config: config,
		record: NewDownloadRecord(config.ConfigDir()),
	}
}

// filenameSanitizer strips characters that are invalid in file names.
var filenameSanitizer = regexp.MustCompile(`[\\/*?:"<>|]`)

func sanitizeFilename(name string) string {
	return filenameSanitizer.ReplaceAllString(name, "_")
}

func isAudio(doc *tg.Document) bool {
	for _, attr := range doc.Attributes {
		if _, ok := attr.(*tg.DocumentAttributeAudio); ok {
			return true
		}
	}
	return strings.HasPrefix(doc.MimeType, "audio/")
}

func extractAudio(msg tg.MessageClass) (*tg.Message, *tg.Document, bool) {
	msgMsg, ok := msg.(*tg.Message)
	if !ok || msgMsg.Media == nil {
		return nil, nil, false
	}
	mediaDoc, ok := msgMsg.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil, nil, false
	}
	doc, ok := mediaDoc.Document.(*tg.Document)
	if !ok {
		return nil, nil, false
	}
	return msgMsg, doc, true
}

// parseTime parses "HH:mm" string to minutes since midnight.
func parseTime(s string) int {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return -1
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}

// isWithinDownloadTime checks if the current time is within the allowed download window.
// Returns true if no restriction is set (both empty) or current time is within range.
func (m *Manager) isWithinDownloadTime() bool {
	start, end := m.config.GetDownloadTimeRange()
	if start == "" && end == "" {
		return true
	}
	startMin := parseTime(start)
	endMin := parseTime(end)
	if startMin < 0 || endMin < 0 {
		return true
	}
	if startMin == endMin {
		return true
	}
	now := time.Now()
	curMin := now.Hour()*60 + now.Minute()
	if startMin <= endMin {
		return curMin >= startMin && curMin <= endMin
	}
	// overnight range (e.g., 22:00-06:00)
	return curMin >= startMin || curMin <= endMin
}

func docFileName(doc *tg.Document) string {
	for _, attr := range doc.Attributes {
		if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
			return fn.FileName
		}
	}
	return fmt.Sprintf("audio_%d.%s", doc.ID, mimeExt(doc.MimeType))
}

func mimeExt(mime string) string {
	switch mime {
	case "audio/mpeg":
		return "mp3"
	case "audio/ogg":
		return "ogg"
	case "audio/flac":
		return "flac"
	case "audio/wav":
		return "wav"
	case "audio/aac":
		return "aac"
	case "audio/mp4":
		return "m4a"
	default:
		return "bin"
	}
}

// audioFormatOf returns the lowercase extension of the document's audio
// format, preferring the filename and falling back to the MIME type.
func audioFormatOf(doc *tg.Document) string {
	if ext := strings.TrimPrefix(filepath.Ext(docFileName(doc)), "."); ext != "" {
		return strings.ToLower(ext)
	}
	return mimeExt(doc.MimeType)
}

// formatAllowed reports whether the document passes the audio format filter.
// An empty filter allows every format.
func formatAllowed(formats []string, doc *tg.Document) bool {
	if len(formats) == 0 {
		return true
	}
	f := audioFormatOf(doc)
	for _, allowed := range formats {
		if strings.EqualFold(allowed, f) {
			return true
		}
	}
	return false
}

// resolveChannel resolves a @username to channel info and an InputPeer.
func (m *Manager) resolveChannel(ctx context.Context, channel string) (*tg.Channel, *tg.InputPeerChannel, error) {
	resolved, err := m.client.API().ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: strings.TrimPrefix(channel, "@"),
	})
	if err != nil {
		return nil, nil, err
	}

	for _, chat := range resolved.Chats {
		if ch, ok := chat.(*tg.Channel); ok {
			peer := &tg.InputPeerChannel{
				ChannelID:  ch.ID,
				AccessHash: ch.AccessHash,
			}
			return ch, peer, nil
		}
	}
	return nil, nil, fmt.Errorf("channel not found in resolved peer")
}

// resolveChannelInputPeer resolves a @username to an InputChannel (for ChannelsGetMessages).
func (m *Manager) resolveChannelInput(ctx context.Context, channel string) (*tg.Channel, *tg.InputChannel, error) {
	ch, peer, err := m.resolveChannel(ctx, channel)
	if err != nil {
		return nil, nil, err
	}
	input := &tg.InputChannel{
		ChannelID:  peer.ChannelID,
		AccessHash: peer.AccessHash,
	}
	return ch, input, nil
}

const readyTimeout = 60 * time.Second

func (m *Manager) finish() {
	m.mu.Lock()
	m.cancel = nil
	m.mu.Unlock()
	m.state.SetRunning(false, "")
}

func (m *Manager) waitReady(ctx context.Context) error {
	readyCtx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	return m.client.WaitReady(readyCtx)
}

func (m *Manager) afterDownloadSuccess(channel string, msgID int, fileName string) {
	m.record.Add(channel, msgID, fileName)
	m.state.MarkDownloaded(channel, msgID)
	m.state.IncrementDownloaded()
}

// Start begins a batch download of all configured channels.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return fmt.Errorf("download already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	m.state.SetRunning(true, "")
	m.state.AddLog("开始下载任务")

	go func() {
		defer m.finish()

		if err := m.waitReady(ctx); err != nil {
			m.state.AddLog(fmt.Sprintf("等待连接就绪失败: %v", err))
			return
		}

		processed := make(map[string]bool)
		for {
			select {
			case <-ctx.Done():
				m.state.AddLog("下载任务已手动停止")
				return
			default:
			}

			channels, downloadDir := m.config.GetChannels()

			var next string
			for _, ch := range channels {
				if !processed[ch] {
					next = ch
					break
				}
			}

			if next == "" {
				break
			}

			processed[next] = true
			m.state.SetRunning(true, next)
			m.state.AddLog(fmt.Sprintf("开始处理频道: %s", next))

			if !m.isWithinDownloadTime() {
				start, end := m.config.GetDownloadTimeRange()
				m.state.AddLog(fmt.Sprintf("当前时间不在下载时间范围(%s-%s)内，停止下载", start, end))
				return
			}

			count, err := m.downloadChannel(ctx, next, downloadDir)
			if err != nil {
				if errors.Is(err, syscall.ENOSPC) {
					m.state.AddLog("磁盘空间不足，停止所有下载任务，请清理磁盘空间后重试")
					return
				}
				m.state.AddLog(fmt.Sprintf("频道 %s 下载失败: %v", next, err))
				continue
			}
			m.state.AddLog(fmt.Sprintf("频道 %s 下载完毕，共下载 %d 个文件", next, count))
		}

		m.state.AddLog(fmt.Sprintf("全部完成，共下载 %d 个文件", m.state.Get().TotalDownloaded))
	}()

	return nil
}

// Stop cancels the current download task.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel == nil {
		return fmt.Errorf("no download running")
	}
	m.cancel()
	return nil
}

// Close stops any running download or scan and releases the record store.
func (m *Manager) Close() {
	_ = m.Stop()
	m.mu.Lock()
	scanCancel := m.scanCancel
	m.mu.Unlock()
	if scanCancel != nil {
		scanCancel()
	}
	if m.record != nil {
		_ = m.record.Close()
	}
}

// dedupeMessages merges scanned messages by channel+msgID, keeping the
// latest entry so repeated scans refresh the list instead of duplicating it.
func dedupeMessages(msgs []ScannedMessage) []ScannedMessage {
	seen := make(map[string]int, len(msgs))
	out := make([]ScannedMessage, 0, len(msgs))
	for _, msg := range msgs {
		key := msg.Channel + "#" + strconv.Itoa(msg.MsgID)
		if i, ok := seen[key]; ok {
			out[i] = msg
			continue
		}
		seen[key] = len(out)
		out = append(out, msg)
	}
	return out
}

// uniquePath returns a non-existing path, appending " (n)" before the
// extension when needed, so files with the same name never overwrite
// each other.
func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// ScanChannels starts background scanning of channels for audio messages.
// Only one scan may run at a time; repeated calls while scanning fail.
func (m *Manager) ScanChannels(ctx context.Context, channels []string, downloadDir string) error {
	m.mu.Lock()
	if m.scanCancel != nil {
		m.mu.Unlock()
		return fmt.Errorf("扫描正在进行中，请稍候")
	}
	taskCtx, cancel := context.WithCancel(ctx)
	m.scanCancel = cancel
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.scanCancel = nil
			m.mu.Unlock()
		}()

		readyCtx, readyCancel := context.WithTimeout(taskCtx, readyTimeout)
		defer readyCancel()
		if err := m.client.WaitReady(readyCtx); err != nil {
			m.state.AddLog(fmt.Sprintf("等待连接就绪失败: %v", err))
			return
		}
		scanCtx, scanCancel := context.WithTimeout(taskCtx, 10*time.Minute)
		defer scanCancel()
		for _, ch := range channels {
			select {
			case <-scanCtx.Done():
				return
			default:
			}

			m.state.AddLog(fmt.Sprintf("扫描频道: %s", ch))
			msgs, err := m.scanChannel(scanCtx, ch, downloadDir)
			if err != nil {
				m.state.AddLog(fmt.Sprintf("扫描频道 %s 失败: %v", ch, err))
				continue
			}

			existing, _ := m.state.GetMessages()
			m.state.SetMessages(dedupeMessages(append(existing, msgs...)))
		}
		msgs, scannedAt := m.state.GetMessages()
		m.hub.Broadcast("scan_result", map[string]interface{}{
			"messages":   msgs,
			"scanned_at": scannedAt,
		})
		m.state.AddLog("扫描完成")
	}()

	return nil
}

// DownloadSingle starts downloading a single message.
func (m *Manager) DownloadSingle(ctx context.Context, channel, dir string, msgID int) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return fmt.Errorf("download already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	m.state.SetRunning(true, channel)

	go func() {
		defer m.finish()

		if err := m.waitReady(ctx); err != nil {
			m.state.AddLog(fmt.Sprintf("等待连接就绪失败: %v", err))
			return
		}

		success, err := m.downloadMessage(ctx, channel, dir, msgID)
		if err != nil || !success {
			m.state.AddLog("下载失败")
			return
		}
		m.afterDownloadSuccess(channel, msgID, "")
		m.state.AddLog("下载完成")
	}()

	return nil
}

// QuickTest tests a channel by downloading the first audio message.
func (m *Manager) QuickTest(ctx context.Context, channel, dir string) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return fmt.Errorf("download already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	m.state.SetRunning(true, channel)
	m.state.AddLog(fmt.Sprintf("快速测试频道: %s", channel))
	m.state.AddLog("获取音频...")

	go func() {
		defer m.finish()

		if err := m.waitReady(ctx); err != nil {
			m.state.AddLog(fmt.Sprintf("等待连接就绪失败: %v", err))
			return
		}

		dlCtx, dlCancel := context.WithTimeout(ctx, 10*time.Minute)
		defer dlCancel()

		_, peer, err := m.resolveChannel(dlCtx, channel)
		if err != nil {
			m.state.AddLog(fmt.Sprintf("频道不存在: %v", err))
			return
		}

		m.state.AddLog("正在获取消息历史...")

		messages, err := m.client.API().MessagesGetHistory(dlCtx, &tg.MessagesGetHistoryRequest{
			Peer:  peer,
			Limit: 20,
		})
		if err != nil {
			m.state.AddLog(fmt.Sprintf("获取消息失败: %v", err))
			return
		}

		//m.state.AddLog(fmt.Sprintf("获取到消息类型: %T", messages))

		var messagesList []tg.MessageClass
		switch v := messages.(type) {
		case *tg.MessagesMessages:
			messagesList = v.Messages
		case *tg.MessagesMessagesSlice:
			messagesList = v.Messages
		case *tg.MessagesChannelMessages:
			messagesList = v.Messages
		default:
			m.state.AddLog(fmt.Sprintf("未知消息类型: %T", messages))
			return
		}

		for _, msg := range messagesList {
			msgMsg, doc, ok := extractAudio(msg)
			if !ok || !isAudio(doc) {
				continue
			}
			if !formatAllowed(m.config.GetAudioFormats(), doc) {
				continue
			}

			m.state.AddLog(fmt.Sprintf("找到音频: %s", sanitizeFilename(docFileName(doc))))
			saveName := sanitizeFilename(docFileName(doc))
			dirPath := filepath.Join(dir, sanitizeFilename(channel))
			if err := os.MkdirAll(dirPath, 0o755); err != nil {
				m.state.AddLog(fmt.Sprintf("创建目录失败: %v", err))
				return
			}
			savePath := uniquePath(filepath.Join(dirPath, saveName))
			if err := m.downloadDocument(dlCtx, doc, savePath); err != nil {
				if errors.Is(err, syscall.ENOSPC) {
					m.state.AddLog("磁盘空间不足，下载失败，请清理磁盘空间后重试")
					return
				}
				m.state.AddLog(fmt.Sprintf("下载错误: %v", err))
			} else {
				m.afterDownloadSuccess(channel, msgMsg.ID, filepath.Base(savePath))
				m.state.AddLog("快速测试完成，请检查下载目录")
			}
			return
		}

		m.state.AddLog("最近20条消息没有音频，请更换频道")
	}()

	return nil
}

func (m *Manager) downloadChannel(ctx context.Context, channel, downloadDir string) (int, error) {
	_, peer, err := m.resolveChannel(ctx, channel)
	if err != nil {
		return 0, fmt.Errorf("resolve channel: %w", err)
	}

	dirPath := filepath.Join(downloadDir, sanitizeFilename(channel))
	m.state.AddLog(fmt.Sprintf("文件下载目录: %s", dirPath))
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return 0, fmt.Errorf("创建目录 %s 失败: %w", dirPath, err)
	}

	count := 0
	offsetID := 0
	page := 0
	for {
		select {
		case <-ctx.Done():
			return count, nil
		default:
		}

		page++
		history, err := m.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     peer,
			OffsetID: offsetID,
			Limit:    100,
		})
		if err != nil {
			if ctx.Err() != nil {
				return count, nil
			}
			return count, err
		}

		var messagesList []tg.MessageClass
		switch v := history.(type) {
		case *tg.MessagesMessages:
			messagesList = v.Messages
		case *tg.MessagesMessagesSlice:
			messagesList = v.Messages
		case *tg.MessagesChannelMessages:
			messagesList = v.Messages
		default:
			m.state.AddLog(fmt.Sprintf("未知消息类型 %T，停止分页", history))
			return count, nil
		}

		if len(messagesList) == 0 {
			m.state.AddLog("没有更多消息")
			break
		}

		m.state.AddLog(fmt.Sprintf("第 %d 页，获取到 %d 条消息", page, len(messagesList)))

		for _, msg := range messagesList {
			offsetID = msg.GetID()
			msgMsg, doc, ok := extractAudio(msg)
			if !ok || !isAudio(doc) {
				continue
			}
			if !formatAllowed(m.config.GetAudioFormats(), doc) {
				continue
			}
			saveName := sanitizeFilename(docFileName(doc))
			savePath := uniquePath(filepath.Join(dirPath, saveName))
			if m.record.Exists(channel, msgMsg.ID) {
				m.state.AddLog(fmt.Sprintf("文件已下载，跳过: %s", saveName))
				continue
			}

			m.state.AddLog(fmt.Sprintf("正在下载: %s", filepath.Base(savePath)))
			if err := m.downloadDocument(ctx, doc, savePath); err != nil {
				if ctx.Err() != nil {
					return count, nil
				}
				if errors.Is(err, syscall.ENOSPC) {
					return count, fmt.Errorf("磁盘空间不足: %w", err)
				}
				m.state.AddLog(fmt.Sprintf("下载失败: %v", err))
				continue
			}
			m.state.AddLog("下载完成")
			m.afterDownloadSuccess(channel, msgMsg.ID, filepath.Base(savePath))
			count++
		}

		if len(messagesList) < 100 {
			break
		}
	}

	return count, nil
}

func (m *Manager) scanChannel(ctx context.Context, channel, downloadDir string) ([]ScannedMessage, error) {
	ch, peer, err := m.resolveChannel(ctx, channel)
	if err != nil {
		return nil, fmt.Errorf("resolve channel: %w", err)
	}

	dirPath := filepath.Join(downloadDir, sanitizeFilename(channel))
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return nil, fmt.Errorf("创建目录 %s 失败: %w", dirPath, err)
	}

	var messages []ScannedMessage
	offsetID := 0

	for {
		history, err := m.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     peer,
			OffsetID: offsetID,
			Limit:    100,
		})
		if err != nil {
			return messages, err
		}

		var messagesList []tg.MessageClass
		switch v := history.(type) {
		case *tg.MessagesMessages:
			messagesList = v.Messages
		case *tg.MessagesMessagesSlice:
			messagesList = v.Messages
		case *tg.MessagesChannelMessages:
			messagesList = v.Messages
		default:
			return messages, nil
		}

		if len(messagesList) == 0 {
			break
		}

		for _, msg := range messagesList {
			offsetID = msg.GetID()
			msgMsg, doc, ok := extractAudio(msg)
			if !ok || !isAudio(doc) {
				continue
			}
			if !formatAllowed(m.config.GetAudioFormats(), doc) {
				continue
			}
			fileName := sanitizeFilename(docFileName(doc))
			messages = append(messages, ScannedMessage{
				Channel:      channel,
				ChannelTitle: ch.Title,
				MsgID:        msgMsg.ID,
				FileName:     fileName,
				FileSize:     doc.Size,
				Exists:       m.record.Exists(channel, msgMsg.ID),
			})
		}

		if len(messagesList) < 100 {
			break
		}
	}

	return messages, nil
}

func (m *Manager) downloadMessage(ctx context.Context, channel, dir string, msgID int) (bool, error) {
	_, input, err := m.resolveChannelInput(ctx, channel)
	if err != nil {
		return false, err
	}

	msgs, err := m.client.API().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: input,
		ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}},
	})
	if err != nil {
		return false, err
	}

	container, ok := msgs.(*tg.MessagesChannelMessages)
	if !ok || len(container.Messages) == 0 {
		return false, fmt.Errorf("message not found")
	}

	_, doc, ok := extractAudio(container.Messages[0])
	if !ok {
		return false, fmt.Errorf("no audio document in message")
	}

	dirPath := filepath.Join(dir, sanitizeFilename(channel))
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return false, fmt.Errorf("创建目录 %s 失败: %w", dirPath, err)
	}
	savePath := filepath.Join(dirPath, sanitizeFilename(docFileName(doc)))

	if m.record.Exists(channel, msgID) {
		return true, nil
	}

	return true, m.downloadDocument(ctx, doc, uniquePath(savePath))
}

const maxDownloadRetries = 3

func (m *Manager) downloadDocument(ctx context.Context, doc *tg.Document, savePath string) error {
	location := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}

	var lastErr error
	for i := 1; i <= maxDownloadRetries; i++ {
		file, err := os.Create(savePath)
		if err != nil {
			return err
		}

		dlCtx, dlCancel := context.WithTimeout(ctx, 10*time.Minute)
		err = m.client.DownloadToFile(dlCtx, location, "", file)
		dlCancel()
		file.Close()

		if err == nil {
			return nil
		}

		lastErr = err
		os.Remove(savePath)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(lastErr, syscall.ENOSPC) {
			return fmt.Errorf("磁盘空间不足: %w", lastErr)
		}
		if i < maxDownloadRetries {
			m.state.AddLog(fmt.Sprintf("下载重试 %d/%d: %v", i, maxDownloadRetries, err))
			time.Sleep(time.Duration(i*2) * time.Second)
		}
	}
	return fmt.Errorf("下载失败，重试 %d 次后仍出错: %w", maxDownloadRetries, lastErr)
}
