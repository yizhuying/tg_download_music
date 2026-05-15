package download

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
}

// Manager orchestrates channel scanning and audio downloading.
type Manager struct {
	state  *DownloadState
	client TelegramAPI
	hub    Broadcaster
	config ConfigProvider
	mu     sync.Mutex
	cancel context.CancelFunc
}

// NewManager creates a new Manager.
func NewManager(client TelegramAPI, state *DownloadState, hub Broadcaster, config ConfigProvider) *Manager {
	return &Manager{client: client, state: state, hub: hub, config: config}
}

func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[\\/*?:"<>|]`)
	return re.ReplaceAllString(name, "_")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
		defer func() {
			m.mu.Lock()
			m.cancel = nil
			m.mu.Unlock()
			m.state.SetRunning(false, "")
		}()

		if err := m.client.WaitReady(ctx); err != nil {
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

			count, err := m.downloadChannel(ctx, next, downloadDir)
			if err != nil {
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

// ScanChannels starts background scanning of channels for audio messages.
func (m *Manager) ScanChannels(ctx context.Context, channels []string, downloadDir string) error {
	go func() {
		ctx := context.Background()
		if err := m.client.WaitReady(ctx); err != nil {
			m.state.AddLog(fmt.Sprintf("等待连接就绪失败: %v", err))
			return
		}
		for _, ch := range channels {
			m.state.AddLog(fmt.Sprintf("扫描频道: %s", ch))
			msgs, err := m.scanChannel(ctx, ch, downloadDir)
			if err != nil {
				m.state.AddLog(fmt.Sprintf("扫描频道 %s 失败: %v", ch, err))
				continue
			}

			existing, _ := m.state.GetMessages()
			existing = append(existing, msgs...)
			m.state.SetMessages(existing)
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
		defer func() {
			m.mu.Lock()
			m.cancel = nil
			m.mu.Unlock()
			m.state.SetRunning(false, "")
		}()

		if err := m.client.WaitReady(ctx); err != nil {
			m.state.AddLog(fmt.Sprintf("等待连接就绪失败: %v", err))
			return
		}

		success, err := m.downloadMessage(ctx, channel, dir, msgID)
		if err != nil || !success {
			m.state.AddLog("下载失败")
			return
		}
		m.state.MarkDownloaded(channel, msgID)
		m.state.IncrementDownloaded()
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
	m.state.AddLog(fmt.Sprintf("测试频道: %s", channel))
	m.state.AddLog("获取第一条音频...")

	go func() {
		defer func() {
			m.mu.Lock()
			m.cancel = nil
			m.mu.Unlock()
			m.state.SetRunning(false, "")
		}()

		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := m.client.WaitReady(ctx); err != nil {
			m.state.AddLog(fmt.Sprintf("等待连接就绪失败: %v", err))
			return
		}

		_, peer, err := m.resolveChannel(ctx, channel)
		if err != nil {
			m.state.AddLog(fmt.Sprintf("频道不存在: %v", err))
			return
		}

		m.state.AddLog(fmt.Sprintf("正在获取消息历史 (Peer: %T)...", peer))

		messages, err := m.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  peer,
			Limit: 20,
		})
		if err != nil {
			m.state.AddLog(fmt.Sprintf("获取消息失败: %v", err))
			return
		}

		m.state.AddLog(fmt.Sprintf("获取到消息类型: %T", messages))

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

			m.state.AddLog(fmt.Sprintf("找到音频，消息ID: %d", msgMsg.ID))
			success, err := m.downloadMessage(ctx, channel, dir, msgMsg.ID)
			if err != nil {
				m.state.AddLog(fmt.Sprintf("下载错误: %v", err))
			} else if success {
				m.state.MarkDownloaded(channel, msgMsg.ID)
				m.state.IncrementDownloaded()
				m.state.AddLog("快速测试完成")
			} else {
				m.state.AddLog("下载失败")
			}
			return
		}

		m.state.AddLog("最近20条消息没有音频")
	}()

	return nil
}

func (m *Manager) downloadChannel(ctx context.Context, channel, downloadDir string) (int, error) {
	_, peer, err := m.resolveChannel(ctx, channel)
	if err != nil {
		return 0, fmt.Errorf("resolve channel: %w", err)
	}

	dirPath := filepath.Join(downloadDir, channel)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return 0, err
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
			saveName := fmt.Sprintf("%d_%s", msgMsg.ID, sanitizeFilename(docFileName(doc)))
			savePath := filepath.Join(dirPath, saveName)
			if fileExists(savePath) {
				m.state.AddLog(fmt.Sprintf("文件已存在，跳过: %s", filepath.Base(savePath)))
				continue
			}

			m.state.AddLog(fmt.Sprintf("正在下载: %s", filepath.Base(savePath)))
			if err := m.downloadDocument(ctx, doc, savePath); err != nil {
				m.state.AddLog(fmt.Sprintf("下载失败: %v", err))
				continue
			}
			m.state.AddLog("下载完成")
			m.state.IncrementDownloaded()
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

	dirPath := filepath.Join(downloadDir, channel)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return nil, err
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
			fileName := sanitizeFilename(docFileName(doc))
			savePath := filepath.Join(dirPath, fmt.Sprintf("%d_%s", msgMsg.ID, fileName))
			messages = append(messages, ScannedMessage{
				Channel:      channel,
				ChannelTitle: ch.Title,
				MsgID:        msgMsg.ID,
				FileName:     fileName,
				FileSize:     doc.Size,
				Exists:       fileExists(savePath),
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

	dirPath := filepath.Join(dir, channel)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return false, err
	}
	savePath := filepath.Join(dirPath, fmt.Sprintf("%d_%s", msgID, sanitizeFilename(docFileName(doc))))

	if fileExists(savePath) {
		return true, nil
	}

	return true, m.downloadDocument(ctx, doc, savePath)
}

func (m *Manager) downloadDocument(ctx context.Context, doc *tg.Document, savePath string) error {
	location := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}

	file, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return m.client.DownloadToFile(ctx, location, "", file)
}
