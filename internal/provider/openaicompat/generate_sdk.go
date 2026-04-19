package openaicompat

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"neo-code/internal/provider"
	"neo-code/internal/provider/openaicompat/chatcompletions"
	"neo-code/internal/provider/openaicompat/responses"
	openaicompatwire "neo-code/internal/provider/openaicompat/wire"
	providertypes "neo-code/internal/provider/types"
)

// generateViaSDK 走 openai-go 发送请求，复用本地 wire 解析，保持事件语义不变。
func (p *Provider) generateViaSDK(
	ctx context.Context,
	req providertypes.GenerateRequest,
	events chan<- providertypes.StreamEvent,
	chatProtocol string,
) error {
	switch chatProtocol {
	case provider.ChatProtocolOpenAIChatCompletions:
		payload, err := chatcompletions.BuildRequest(ctx, p.cfg, req)
		if err != nil {
			return err
		}
		endpoint, err := provider.ResolveChatEndpointURL(p.cfg.BaseURL, p.cfg.ChatEndpointPath)
		if err != nil {
			return fmt.Errorf("%sinvalid chat endpoint configuration: %w", errorPrefix, err)
		}
		return p.sendSDKStreamRequest(ctx, endpoint, payload, chatcompletionsStreamConsumer, openaicompatwire.ParseError, events)
	case provider.ChatProtocolOpenAIResponses:
		payload, err := responses.BuildRequest(ctx, p.cfg, req)
		if err != nil {
			return err
		}
		endpoint, err := provider.ResolveChatEndpointURL(p.cfg.BaseURL, p.cfg.ChatEndpointPath)
		if err != nil {
			return fmt.Errorf("%sinvalid chat endpoint configuration: %w", errorPrefix, err)
		}
		return p.sendSDKStreamRequest(ctx, endpoint, payload, responses.ConsumeStream, openaicompatwire.ParseError, events)
	default:
		return provider.NewDiscoveryConfigError(
			fmt.Sprintf("openaicompat provider: driver %q resolved unsupported chat protocol %q", p.cfg.Driver, chatProtocol),
		)
	}
}

func (p *Provider) sendSDKStreamRequest(
	ctx context.Context,
	endpoint string,
	payload any,
	consumeStream func(context.Context, io.Reader, chan<- providertypes.StreamEvent) error,
	parseError func(*http.Response) error,
	events chan<- providertypes.StreamEvent,
) error {
	client := p.newSDKClient()
	var resp *http.Response

	err := client.Post(
		ctx,
		strings.TrimSpace(endpoint),
		payload,
		nil,
		option.WithResponseInto(&resp),
		option.WithHeader("Accept", "text/event-stream"),
	)
	if err != nil {
		if resp != nil && resp.StatusCode >= http.StatusBadRequest {
			return parseError(resp)
		}
		return fmt.Errorf("%ssend request: %w", errorPrefix, err)
	}
	if resp == nil {
		return fmt.Errorf("%ssend request: empty response", errorPrefix)
	}
	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
			log.Printf("%sclose response body: %v", errorPrefix, closeErr)
		}
	}(resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		return parseError(resp)
	}
	return consumeStream(ctx, resp.Body, events)
}

func (p *Provider) newSDKClient() openai.Client {
	return openai.NewClient(
		option.WithHTTPClient(p.client),
		option.WithAPIKey(strings.TrimSpace(p.cfg.APIKey)),
		option.WithMaxRetries(0),
	)
}

func chatcompletionsStreamConsumer(ctx context.Context, body io.Reader, events chan<- providertypes.StreamEvent) error {
	return openaicompatwire.ConsumeStream(ctx, body, events)
}
