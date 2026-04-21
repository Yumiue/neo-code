package openaicompat

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"neo-code/internal/provider"
	"neo-code/internal/provider/openaicompat/chatcompletions"
	"neo-code/internal/provider/openaicompat/responses"
	providertypes "neo-code/internal/provider/types"
)

// generateSDKChatCompletions 走 SDK chat/completions 发送请求
func (p *Provider) generateSDKChatCompletions(
	ctx context.Context,
	req providertypes.GenerateRequest,
	events chan<- providertypes.StreamEvent,
) error {
	payload, err := chatcompletions.BuildRequest(ctx, p.cfg, req)
	if err != nil {
		return err
	}

	client := p.newSDKClient()
	params := convertToChatCompletionParams(payload)

	stream := client.Chat.Completions.NewStreaming(ctx, params)
	return chatcompletions.EmitFromSDKStream(ctx, stream, events)
}

func convertToChatCompletionParams(req chatcompletions.Request) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(req.Model),
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, convertToSDKMessage(msg))
	}
	params.Messages = messages

	if len(req.Tools) > 0 {
		tools := make([]openai.ChatCompletionToolUnionParam, 0, len(req.Tools))
		for _, spec := range req.Tools {
			tools = append(tools, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
				Name:        spec.Function.Name,
				Description: openai.String(spec.Function.Description),
				Parameters:  openai.FunctionParameters(spec.Function.Parameters),
			}))
		}
		params.Tools = tools
		if req.ToolChoice != "" {
			params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
				OfAuto: openai.String(string(openai.ChatCompletionToolChoiceOptionAutoAuto)),
			}
		}
	}

	return params
}

func convertToSDKMessage(msg chatcompletions.Message) openai.ChatCompletionMessageParamUnion {
	switch msg.Role {
	case "system":
		return openai.SystemMessage(msg.Content.(string))
	case "user":
		if content, ok := msg.Content.(string); ok {
			return openai.UserMessage(content)
		}
		// Handle multi-part content if needed
		return openai.UserMessage(fmt.Sprintf("%v", msg.Content))
	case "assistant":
		return openai.AssistantMessage(msg.Content.(string))
	default:
		return openai.UserMessage(fmt.Sprintf("%v", msg.Content))
	}
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

	client := p.newSDKClient()
	var resp *http.Response
	err = client.Post(
		ctx,
		strings.TrimSpace(endpoint),
		payload,
		nil,
		option.WithResponseInto(&resp),
		option.WithHeader("Accept", "text/event-stream"),
	)
	if err != nil {
		return fmt.Errorf("%ssend request: %w", errorPrefix, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return ParseError(resp)
	}

	return responses.EmitFromStream(ctx, resp.Body, events)
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
