package openaicompat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"neo-code/internal/provider"
	"neo-code/internal/provider/openaicompat/chatcompletions"
	"neo-code/internal/provider/openaicompat/responses"
	providertypes "neo-code/internal/provider/types"
)

const (
	maxStreamingErrorSummaryBytes int64 = 8 * 1024
	streamProbeBytes                    = 512
)

// generateSDKChatCompletions 走 SDK chat/completions 发送请求，复用本地 wire 解析。
func (p *Provider) generateSDKChatCompletions(
	ctx context.Context,
	req providertypes.GenerateRequest,
	events chan<- providertypes.StreamEvent,
) error {
	payload, err := chatcompletions.BuildRequest(ctx, p.cfg, req)
	if err != nil {
		return err
	}
	endpoint, err := resolveChatEndpoint(p.cfg)
	if err != nil {
		return err
	}
	return p.sendSDKStreamRequest(ctx, endpoint, payload, chatcompletions.ConsumeStream, ParseError, events)
}

// generateSDKResponses 走 SDK responses 发送请求，复用本地流事件映射。
func (p *Provider) generateSDKResponses(
	ctx context.Context,
	req providertypes.GenerateRequest,
	events chan<- providertypes.StreamEvent,
) error {
	payload, err := responses.BuildRequest(ctx, p.cfg, req)
	if err != nil {
		return err
	}
	endpoint, err := resolveChatEndpoint(p.cfg)
	if err != nil {
		return err
	}
	return p.sendSDKStreamRequest(ctx, endpoint, payload, responses.ConsumeStream, ParseError, events)
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
			if resp.Body != nil {
				defer func(body io.ReadCloser) {
					if closeErr := body.Close(); closeErr != nil {
						log.Printf("%sclose response body: %v", errorPrefix, closeErr)
					}
				}(resp.Body)
			}
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
	if err := validateStreamingResponse(resp); err != nil {
		return err
	}
	return consumeStream(ctx, resp.Body, events)
}

func (p *Provider) newSDKClient() openai.Client {
	return openai.NewClient(
		option.WithHTTPClient(p.client),
		option.WithAPIKey(strings.TrimSpace(p.cfg.APIKey)),
	)
}

func resolveChatEndpoint(cfg provider.RuntimeConfig) (string, error) {
	chatEndpointPath := resolveChatEndpointPathByMode(cfg.ChatEndpointPath, cfg.ChatAPIMode)
	endpoint, err := provider.ResolveChatEndpointURL(cfg.BaseURL, chatEndpointPath)
	if err != nil {
		return "", fmt.Errorf("%sinvalid chat endpoint configuration: %w", errorPrefix, err)
	}
	return endpoint, nil
}

// resolveChatEndpointPathByMode 在 chat endpoint 为空时，根据 chat_api_mode 自动回填默认端点路径。
func resolveChatEndpointPathByMode(rawPath string, chatAPIMode string) string {
	if strings.TrimSpace(rawPath) != "" {
		return rawPath
	}

	mode, err := provider.NormalizeProviderChatAPIMode(chatAPIMode)
	if err != nil || mode == "" {
		mode = provider.DefaultProviderChatAPIMode()
	}
	if mode == provider.ChatAPIModeResponses {
		return chatEndpointPathResponses
	}
	return chatEndpointPathCompletions
}

// validateStreamingResponse 校验流式响应协议，避免非 SSE 响应被误交给流解析器。
func validateStreamingResponse(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("%sstream response is empty", errorPrefix)
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType, _, _ := mime.ParseMediaType(contentType)
	mediaType = strings.TrimSpace(strings.ToLower(mediaType))

	if mediaType == "text/event-stream" {
		return nil
	}

	reader := bufio.NewReader(resp.Body)
	originalBody := resp.Body
	resp.Body = struct {
		io.Reader
		io.Closer
	}{
		Reader: reader,
		Closer: originalBody,
	}

	if mediaType == "" {
		peek, _ := reader.Peek(streamProbeBytes)
		if !looksLikeHTMLPayload(peek) {
			return nil
		}
	}

	summary := readHTTPErrorSummary(reader, maxStreamingErrorSummaryBytes)
	if looksLikeHTMLPayload([]byte(summary)) {
		summary = strings.TrimSpace(htmlTagPattern.ReplaceAllString(summary, " "))
	}
	message := "upstream did not return an SSE stream"
	if mediaType == "" {
		message += " (missing content-type)"
	} else {
		message += fmt.Sprintf(" (content-type=%s)", mediaType)
	}
	if summary != "" {
		message += "; upstream body: " + summary
	}
	return provider.NewProviderErrorFromStatus(http.StatusBadGateway, message)
}

// looksLikeHTMLPayload 判断响应片段是否明显是 HTML 页面内容。
func looksLikeHTMLPayload(payload []byte) bool {
	trimmed := strings.TrimSpace(strings.ToLower(string(payload)))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}
