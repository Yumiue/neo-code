package openaicompat

import (
	"context"
	"fmt"

	"neo-code/internal/provider"
	"neo-code/internal/provider/openaicompat/chatcompletions"
	"neo-code/internal/provider/openaicompat/responses"
	providertypes "neo-code/internal/provider/types"
)

// generateViaHTTP 走本地 HTTP 实现，保持对第三方 OpenAI-compatible 网关的最大兼容面。
func (p *Provider) generateViaHTTP(
	ctx context.Context,
	req providertypes.GenerateRequest,
	events chan<- providertypes.StreamEvent,
	chatProtocol string,
) error {
	switch chatProtocol {
	case provider.ChatProtocolOpenAIChatCompletions:
		impl, buildErr := chatcompletions.New(p.cfg, p.client)
		if buildErr != nil {
			return buildErr
		}
		return impl.Generate(ctx, req, events)
	case provider.ChatProtocolOpenAIResponses:
		impl, buildErr := responses.New(p.cfg, p.client)
		if buildErr != nil {
			return buildErr
		}
		return impl.Generate(ctx, req, events)
	default:
		return provider.NewDiscoveryConfigError(
			fmt.Sprintf("openaicompat provider: driver %q resolved unsupported chat protocol %q", p.cfg.Driver, chatProtocol),
		)
	}
}
