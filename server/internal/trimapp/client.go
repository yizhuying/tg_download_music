// Package trimapp calls fnOS open APIs through the gateway Unix socket.
// See https://developer.fnnas.com/api/calling/.
package trimapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// gatewaySocket is the only supported transport for the open API backend.
const gatewaySocket = "/var/run/trim_open_gateway_apiscope.socket"

// Capability identifiers used by this app.
const (
	ReqGetSharedAccessibleFolders = "trim.file.getSharedAccessibleFolders"
	ReqConvertPath                = "trim.file.convertPath"
)

// Client posts requests to the fnOS open API gateway on behalf of the app.
type Client struct {
	appName string
	http    *http.Client
}

// New creates a gateway client bound to the package appname.
func New(appName string) *Client {
	return &Client{
		appName: appName,
		http: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", gatewaySocket)
				},
			},
		},
	}
}

type gatewayRequest struct {
	ReqID   string `json:"reqId"`
	Req     string `json:"req"`
	AppName string `json:"appName"`
	Data    any    `json:"data,omitempty"`
}

type gatewayResponse struct {
	ReqID string          `json:"reqId"`
	Code  int             `json:"code"`
	Msg   string          `json:"msg"`
	Data  json.RawMessage `json:"data"`
}

func (c *Client) call(ctx context.Context, req string, data any) (json.RawMessage, error) {
	// fnOS rotates the token on app re-register/reinstall, so it must be
	// re-read on every call and never cached or persisted.
	token := os.Getenv("TRIM_API_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("%s: TRIM_API_TOKEN not set", req)
	}

	body, err := json.Marshal(gatewayRequest{
		ReqID:   fmt.Sprintf("tg-%d", time.Now().UnixNano()),
		Req:     req,
		AppName: c.appName,
		Data:    data,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://trimapp/api/v1/trimapp", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gwResp gatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&gwResp); err != nil {
		return nil, err
	}
	if gwResp.Code != 0 {
		return nil, fmt.Errorf("%s: code=%d msg=%s", req, gwResp.Code, gwResp.Msg)
	}
	return gwResp.Data, nil
}

// GetSharedAccessibleFolders returns the directories the administrator
// granted to the app (scope trim.file.sharedAccess).
func (c *Client) GetSharedAccessibleFolders(ctx context.Context) ([]string, error) {
	data, err := c.call(ctx, ReqGetSharedAccessibleFolders, struct{}{})
	if err != nil {
		return nil, err
	}
	var result struct {
		Paths []string `json:"paths"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Paths, nil
}

type convertResult struct {
	Path         string `json:"path"`
	SemanticPath string `json:"semanticPath"`
}

// parseConvertResult accepts both gateway shapes: a bare result array (what
// fnOS actually returns on device) and the documented {status, result} object.
func parseConvertResult(data []byte) ([]convertResult, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%s: empty data", ReqConvertPath)
	}
	if trimmed[0] == '[' {
		var results []convertResult
		if err := json.Unmarshal(trimmed, &results); err != nil {
			return nil, fmt.Errorf("%s: decode array: %w", ReqConvertPath, err)
		}
		return results, nil
	}
	var wrapped struct {
		Status int             `json:"status"`
		Result []convertResult `json:"result"`
	}
	if err := json.Unmarshal(trimmed, &wrapped); err != nil {
		return nil, fmt.Errorf("%s: decode object: %w (data: %s)", ReqConvertPath, err, string(trimmed))
	}
	if wrapped.Status != 0 {
		return nil, fmt.Errorf("%s: status=%d", ReqConvertPath, wrapped.Status)
	}
	return wrapped.Result, nil
}

// ConvertPaths maps internal paths (/vol1/...) to user-facing semantic
// paths (scope trim.file.path), keyed by the original path.
func (c *Client) ConvertPaths(ctx context.Context, paths []string, language string) (map[string]string, error) {
	data, err := c.call(ctx, ReqConvertPath, struct {
		Path     []string `json:"path"`
		Language string   `json:"language"`
	}{Path: paths, Language: language})
	if err != nil {
		return nil, err
	}
	results, err := parseConvertResult(data)
	if err != nil {
		return nil, err
	}
	converted := make(map[string]string, len(results))
	for _, r := range results {
		if r.SemanticPath != "" {
			converted[r.Path] = r.SemanticPath
		}
	}
	return converted, nil
}
