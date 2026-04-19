package responses

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"neo-code/internal/provider"
	openaicompatwire "neo-code/internal/provider/openaicompat/wire"
	providertypes "neo-code/internal/provider/types"
)

// Provider 封装 Responses 端点的请求组装、发送与流式响应解析。
type Provider struct {
	cfg    provider.RuntimeConfig
	client *http.Client
}

// New 基于共享运行时配置与 HTTP client 创建 Responses provider。
func New(cfg provider.RuntimeConfig, client *http.Client) (*Provider, error) {
	if err := validateRuntimeConfig(cfg); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("%sclient is nil", errorPrefix)
	}

	return &Provider{
		cfg:    cfg,
		client: client,
	}, nil
}

// Generate 发起 Responses SSE 流式生成请求。
func (p *Provider) Generate(ctx context.Context, req providertypes.GenerateRequest, events chan<- providertypes.StreamEvent) error {
	payload, err := BuildRequest(ctx, p.cfg, req)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%smarshal request: %w", errorPrefix, err)
	}

	endpoint, err := provider.ResolveChatEndpointURL(p.cfg.BaseURL, p.cfg.ChatEndpointPath)
	if err != nil {
		return fmt.Errorf("%sinvalid chat endpoint configuration: %w", errorPrefix, err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%sbuild request: %w", errorPrefix, err)
	}
	applyAuthHeaders(httpReq.Header, p.cfg)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%ssend request: %w", errorPrefix, err)
	}
	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
			log.Printf("%sclose response body: %v", errorPrefix, closeErr)
		}
	}(resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		return openaicompatwire.ParseError(resp)
	}

	return ConsumeStream(ctx, resp.Body, events)
}
