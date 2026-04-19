package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"neo-code/internal/provider"
	"neo-code/internal/provider/anthropic/wire"
	httpdiscovery "neo-code/internal/provider/discovery/http"
	providertypes "neo-code/internal/provider/types"
)

// Provider 封装 Anthropic messages 协议的请求发送与流式解析。
type Provider struct {
	cfg    provider.RuntimeConfig
	client *http.Client
}

// buildOptions 描述 Anthropic provider 构建时可选注入项。
type buildOptions struct {
	transport http.RoundTripper
}

// buildOption 为 New 提供函数式配置。
type buildOption func(*buildOptions)

// withTransport 注入自定义 HTTP Transport。
func withTransport(rt http.RoundTripper) buildOption {
	return func(o *buildOptions) {
		o.transport = rt
	}
}

// New 创建 Anthropic provider 实例。
func New(cfg provider.RuntimeConfig, opts ...buildOption) (*Provider, error) {
	if err := validateRuntimeConfig(cfg); err != nil {
		return nil, err
	}

	o := &buildOptions{transport: http.DefaultTransport}
	for _, apply := range opts {
		apply(o)
	}

	return &Provider{
		cfg: cfg,
		client: &http.Client{
			Timeout:   90 * time.Second,
			Transport: o.transport,
		},
	}, nil
}

// DiscoverModels 通过统一 discovery/http 入口发现 Anthropic 可用模型。
func (p *Provider) DiscoverModels(ctx context.Context) ([]providertypes.ModelDescriptor, error) {
	requestCfg, err := httpdiscovery.RequestConfigFromRuntime(p.cfg)
	if err != nil {
		return nil, err
	}
	return httpdiscovery.DiscoverModelDescriptors(ctx, p.client, requestCfg)
}

// Generate 发起 Anthropic /messages 流式请求。
func (p *Provider) Generate(ctx context.Context, req providertypes.GenerateRequest, events chan<- providertypes.StreamEvent) error {
	if err := supportedChatProtocol(p.cfg); err != nil {
		return err
	}

	payload, err := BuildRequest(ctx, p.cfg, req)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%smarshal request: %w", errorPrefix, err)
	}

	endpointPath := strings.TrimSpace(p.cfg.ChatEndpointPath)
	if endpointPath == "" || endpointPath == "/" {
		endpointPath = "/messages"
	}
	endpoint, err := provider.ResolveChatEndpointURL(p.cfg.BaseURL, endpointPath)
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
		return wire.ParseError(resp)
	}

	return wire.ConsumeStream(ctx, resp.Body, events)
}

// applyAuthHeaders 应用 Anthropic 请求所需认证头。
func applyAuthHeaders(header http.Header, cfg provider.RuntimeConfig) {
	authStrategy, apiVersion := provider.ResolveDriverAuthConfig(cfg.Driver)
	provider.ApplyAuthHeaders(header, authStrategy, cfg.APIKey, apiVersion)
}
