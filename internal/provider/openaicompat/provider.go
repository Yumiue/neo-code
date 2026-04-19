package openaicompat

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"neo-code/internal/provider"
	httpdiscovery "neo-code/internal/provider/discovery/http"
	providertypes "neo-code/internal/provider/types"
)

// Provider 封装 OpenAI 兼容协议的运行时配置与 HTTP 客户端。
type Provider struct {
	cfg           provider.RuntimeConfig
	client        *http.Client
	executionMode string
}

// buildOptions 控制 provider 构建时的可选注入项。
type buildOptions struct {
	transport     http.RoundTripper
	executionMode string
}

// buildOption 是 New 的函数式配置项。
type buildOption func(*buildOptions)

// withTransport 注入自定义 HTTP Transport。
func withTransport(rt http.RoundTripper) buildOption {
	return func(o *buildOptions) {
		o.transport = rt
	}
}

// withExecutionMode 显式覆盖 openaicompat Generate 的执行模式（auto/http/sdk）。
func withExecutionMode(mode string) buildOption {
	return func(o *buildOptions) {
		o.executionMode = mode
	}
}

// New 创建 OpenAI 兼容 provider 实例。
func New(cfg provider.RuntimeConfig, opts ...buildOption) (*Provider, error) {
	if err := validateRuntimeConfig(cfg); err != nil {
		return nil, err
	}

	o := &buildOptions{
		transport:     http.DefaultTransport,
		executionMode: executionModeAuto,
	}
	for _, apply := range opts {
		apply(o)
	}
	mode, err := normalizeExecutionMode(o.executionMode)
	if err != nil {
		return nil, err
	}

	return &Provider{
		cfg:           cfg,
		executionMode: mode,
		client: &http.Client{
			Timeout:   90 * time.Second,
			Transport: o.transport,
		},
	}, nil
}

// DiscoverModels 通过统一 discovery/http 入口发现可用模型。
func (p *Provider) DiscoverModels(ctx context.Context) ([]providertypes.ModelDescriptor, error) {
	requestCfg, err := httpdiscovery.RequestConfigFromRuntime(p.cfg)
	if err != nil {
		return nil, err
	}
	return httpdiscovery.DiscoverModelDescriptors(ctx, p.client, requestCfg)
}

// Generate 发起流式生成请求。
func (p *Provider) Generate(ctx context.Context, req providertypes.GenerateRequest, events chan<- providertypes.StreamEvent) error {
	chatProtocol, err := supportedChatProtocol(p.cfg)
	if err != nil {
		return err
	}

	executionMode := resolveExecutionMode(p.cfg, chatProtocol, p.executionMode)
	switch executionMode {
	case executionModeSDK:
		return p.generateViaSDK(ctx, req, events, chatProtocol)
	case executionModeHTTP:
		return p.generateViaHTTP(ctx, req, events, chatProtocol)
	default:
		return provider.NewDiscoveryConfigError(
			fmt.Sprintf("openaicompat provider: unsupported execution mode %q", executionMode),
		)
	}
}

// supportedChatProtocol 校验当前驱动可用的聊天协议。
func supportedChatProtocol(cfg provider.RuntimeConfig) (string, error) {
	driverProtocol := provider.ResolveDriverProtocolDefaults(cfg.Driver).ChatProtocol
	if driverProtocol != provider.ChatProtocolOpenAIChatCompletions &&
		driverProtocol != provider.ChatProtocolOpenAIResponses {
		return "", provider.NewDiscoveryConfigError(
			fmt.Sprintf("openaicompat provider: driver %q resolved unsupported chat protocol %q", cfg.Driver, driverProtocol),
		)
	}

	normalized := provider.NormalizeProviderChatProtocol(cfg.ChatProtocol)
	if normalized == "" {
		normalized = inferChatProtocolFromEndpointPath(cfg.ChatEndpointPath)
	}
	if normalized == "" {
		normalized = driverProtocol
	}
	switch normalized {
	case provider.ChatProtocolOpenAIChatCompletions, provider.ChatProtocolOpenAIResponses:
		return normalized, nil
	default:
		return "", provider.NewDiscoveryConfigError(
			fmt.Sprintf("openaicompat provider: driver %q resolved unsupported chat protocol %q", cfg.Driver, normalized),
		)
	}
}

// inferChatProtocolFromEndpointPath 根据聊天端点路径推断 OpenAI-compatible 的协议类型。
func inferChatProtocolFromEndpointPath(endpointPath string) string {
	normalizedPath, err := provider.NormalizeProviderChatEndpointPath(endpointPath)
	if err != nil {
		return ""
	}
	trimmedPath := strings.Trim(strings.ToLower(strings.TrimSpace(normalizedPath)), "/")
	switch {
	case trimmedPath == "responses" || strings.HasSuffix(trimmedPath, "/responses"):
		return provider.ChatProtocolOpenAIResponses
	case trimmedPath == "chat/completions" || strings.HasSuffix(trimmedPath, "/chat/completions"):
		return provider.ChatProtocolOpenAIChatCompletions
	default:
		return ""
	}
}
