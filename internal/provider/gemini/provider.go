package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"neo-code/internal/provider"
	httpdiscovery "neo-code/internal/provider/discovery/http"
	"neo-code/internal/provider/gemini/wire"
	providertypes "neo-code/internal/provider/types"
)

// Provider 封装 Gemini native 协议的请求发送与流式响应解析。
type Provider struct {
	cfg    provider.RuntimeConfig
	client *http.Client
}

// buildOptions 描述 Gemini provider 构建时可选注入项。
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

// New 创建 Gemini native provider 实例。
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

// DiscoverModels 通过统一 discovery/http 入口发现 Gemini 可用模型。
func (p *Provider) DiscoverModels(ctx context.Context) ([]providertypes.ModelDescriptor, error) {
	requestCfg, err := httpdiscovery.RequestConfigFromRuntime(p.cfg)
	if err != nil {
		return nil, err
	}
	return httpdiscovery.DiscoverModelDescriptors(ctx, p.client, requestCfg)
}

// Generate 发起 Gemini streamGenerateContent 流式请求。
func (p *Provider) Generate(ctx context.Context, req providertypes.GenerateRequest, events chan<- providertypes.StreamEvent) error {
	if err := supportedChatProtocol(p.cfg); err != nil {
		return err
	}

	payload, model, err := BuildRequest(ctx, p.cfg, req)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%smarshal request: %w", errorPrefix, err)
	}

	endpoint, err := resolveGeminiStreamEndpoint(p.cfg.BaseURL, p.cfg.ChatEndpointPath, model)
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

// resolveGeminiStreamEndpoint 生成 Gemini streamGenerateContent 最终请求地址。
func resolveGeminiStreamEndpoint(baseURL string, endpointPath string, model string) (string, error) {
	normalizedModel := normalizeGeminiModelName(model)
	if normalizedModel == "" {
		return "", errors.New("model is empty")
	}

	normalizedPath, err := provider.ResolveChatEndpointPath(endpointPath)
	if err != nil {
		return "", err
	}
	if normalizedPath == "" {
		normalizedPath = "/models"
	}
	if !strings.HasSuffix(normalizedPath, "/models") {
		normalizedPath = strings.TrimRight(normalizedPath, "/") + "/models"
	}

	endpointBase, err := provider.ResolveChatEndpointURL(baseURL, normalizedPath)
	if err != nil {
		return "", err
	}
	endpointBase = strings.TrimRight(endpointBase, "/")
	return endpointBase + "/" + url.PathEscape(normalizedModel) + ":streamGenerateContent?alt=sse", nil
}

// normalizeGeminiModelName 统一清洗 Gemini 模型名，兼容 discover 返回的 "models/{id}" 形式。
func normalizeGeminiModelName(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "models/"))
}

// applyAuthHeaders 应用 Gemini 请求所需认证头。
func applyAuthHeaders(header http.Header, cfg provider.RuntimeConfig) {
	authStrategy, apiVersion := provider.ResolveDriverAuthConfig(cfg.Driver)
	provider.ApplyAuthHeaders(header, authStrategy, cfg.APIKey, apiVersion)
}
