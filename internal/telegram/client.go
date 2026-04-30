package telegram

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

// Client wraps gotd's MTProto client.
type Client struct {
	client *telegram.Client
	cfg    AppConfig
	logger *zap.Logger
}

// AppConfig holds Telegram application configuration.
type AppConfig struct {
	APIID       int
	APIHash     string
	SessionDir  string
	SessionName string
	Proxy       ProxyConfig
}

// ProxyConfig holds proxy connection settings.
type ProxyConfig struct {
	Scheme   string
	Hostname string
	Port     int
	Username string
	Password string
}

// NewClient creates an unstarted gotd client with file-based session storage.
func NewClient(ctx context.Context, cfg AppConfig, logger *zap.Logger) (*Client, error) {
	if err := os.MkdirAll(cfg.SessionDir, 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	sessPath := filepath.Join(cfg.SessionDir, cfg.SessionName+".json")

	// File-based session storage
	sessStorage := &session.FileStorage{Path: sessPath}

	opts := telegram.Options{
		SessionStorage: sessStorage,
		Logger:         logger,
	}

	// gotd reads proxy from ALL_PROXY / NO_PROXY env vars.
	// If config specifies a proxy, set the environment variable so OptionsFromEnvironment picks it up.
	if cfg.Proxy.Hostname != "" && cfg.Proxy.Port != 0 {
		addr := fmt.Sprintf("%s:%d", cfg.Proxy.Hostname, cfg.Proxy.Port)
		scheme := cfg.Proxy.Scheme
		if scheme == "" {
			scheme = "socks5"
		}
		if cfg.Proxy.Username != "" {
			addr = fmt.Sprintf("%s://%s:%s@%s", scheme, cfg.Proxy.Username, cfg.Proxy.Password, addr)
		} else {
			addr = fmt.Sprintf("%s://%s", scheme, addr)
		}
		_ = os.Setenv("ALL_PROXY", addr)

		enrichedOpts, err := telegram.OptionsFromEnvironment(opts)
		if err != nil {
			return nil, fmt.Errorf("options from env (proxy): %w", err)
		}
		opts = enrichedOpts
	}

	tgClient := telegram.NewClient(cfg.APIID, cfg.APIHash, opts)

	return &Client{
		client: tgClient,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Run starts the client and executes the provided function within the
// authenticated session context.
func (c *Client) Run(ctx context.Context, f func(ctx context.Context) error) error {
	return c.client.Run(ctx, f)
}

// API returns the raw tg.Client for making Telegram API calls.
func (c *Client) API() *tg.Client {
	return c.client.API()
}

// IsAuthorized checks whether the stored session is valid and authenticated.
// Returns false if no session exists or authentication is required.
func (c *Client) IsAuthorized(ctx context.Context) (bool, error) {
	self, err := c.client.Self(ctx)
	if err != nil {
		return false, nil
	}
	return self != nil, nil
}

// Disconnect gracefully disconnects the client.
// Note: gotd handles cleanup on context cancellation; this is a no-op placeholder.
func (c *Client) Disconnect(ctx context.Context) error {
	return nil
}

// DownloadToFile downloads a file from Telegram using gotd's downloader,
// writing to the provided io.WriterAt. Progress is reported via the callback.
func (c *Client) DownloadToFile(ctx context.Context, location tg.InputFileLocationClass, _ string, w io.WriterAt) error {
	dl := downloader.NewDownloader().WithAllowCDN(true)
	_, err := dl.Download(c.client.API(), location).WithThreads(4).Parallel(ctx, w)
	return err
}
