package provider

import (
	"fmt"
	"strings"
)

// NormalizedProtocolSettings 收敛 provider 协议相关字段，统一承载新旧配置映射后的可执行结果。
type NormalizedProtocolSettings struct {
	ChatProtocol          string
	ChatEndpointPath      string
	DiscoveryProtocol     string
	DiscoveryEndpointPath string
	AuthStrategy          string
	ResponseProfile       string
	LegacyAPIStyle        string
}

// NormalizeProviderChatProtocol 规范化 chat protocol 枚举，未知值保持原样供上层做更严格校验。
func NormalizeProviderChatProtocol(value string) string {
	return NormalizeKey(value)
}

// NormalizeProviderDiscoveryProtocol 规范化 discovery protocol 枚举，未知值保持原样供上层做更严格校验。
func NormalizeProviderDiscoveryProtocol(value string) string {
	return NormalizeKey(value)
}

// NormalizeProviderAuthStrategy 规范化 auth strategy 枚举，未知值保持原样供上层做更严格校验。
func NormalizeProviderAuthStrategy(value string) string {
	return NormalizeKey(value)
}

// NormalizeProviderChatEndpointPath 规范化聊天端点路径，沿用 discovery 路径的安全约束规则。
func NormalizeProviderChatEndpointPath(endpointPath string) (string, error) {
	return NormalizeProviderDiscoveryEndpointPath(endpointPath)
}

// NormalizeProviderProtocolSettings 合并新旧协议字段并执行组合校验，输出统一可执行协议配置。
func NormalizeProviderProtocolSettings(
	driver string,
	chatProtocol string,
	chatEndpointPath string,
	discoveryProtocol string,
	discoveryEndpointPath string,
	authStrategy string,
	responseProfile string,
	legacyAPIStyle string,
	legacyDiscoveryResponseProfile string,
) (NormalizedProtocolSettings, error) {
	normalizedDriver := NormalizeProviderDriver(driver)
	settings := NormalizedProtocolSettings{
		ChatProtocol:      NormalizeProviderChatProtocol(chatProtocol),
		DiscoveryProtocol: NormalizeProviderDiscoveryProtocol(discoveryProtocol),
		AuthStrategy:      NormalizeProviderAuthStrategy(authStrategy),
		ResponseProfile:   NormalizeKey(responseProfile),
		LegacyAPIStyle:    NormalizeProviderAPIStyle(legacyAPIStyle),
	}

	normalizedChatEndpointPath, err := NormalizeProviderChatEndpointPath(chatEndpointPath)
	if err != nil {
		return NormalizedProtocolSettings{}, err
	}
	normalizedDiscoveryEndpointPath, err := NormalizeProviderDiscoveryEndpointPath(discoveryEndpointPath)
	if err != nil {
		return NormalizedProtocolSettings{}, err
	}
	settings.ChatEndpointPath = normalizedChatEndpointPath
	settings.DiscoveryEndpointPath = normalizedDiscoveryEndpointPath

	if settings.LegacyAPIStyle != "" {
		mappedChatProtocol := ""
		switch settings.LegacyAPIStyle {
		case OpenAICompatibleAPIStyleChatCompletions:
			mappedChatProtocol = ChatProtocolOpenAIChatCompletions
		case OpenAICompatibleAPIStyleResponses:
			mappedChatProtocol = ChatProtocolOpenAIResponses
		}
		if mappedChatProtocol != "" &&
			(settings.ChatProtocol == "" ||
				settings.ChatProtocol == ChatProtocolOpenAIChatCompletions ||
				settings.ChatProtocol == ChatProtocolOpenAIResponses) {
			settings.ChatProtocol = mappedChatProtocol
		}
	}

	if settings.ResponseProfile == "" {
		legacyProfile, err := NormalizeProviderDiscoveryResponseProfile(legacyDiscoveryResponseProfile)
		if err != nil {
			return NormalizedProtocolSettings{}, err
		}
		settings.ResponseProfile = legacyProfile
	}

	applyDriverDefaultProtocolSettings(normalizedDriver, &settings)
	if err := validateProtocolEnums(settings); err != nil {
		return NormalizedProtocolSettings{}, err
	}
	if err := validateProtocolCombinations(settings); err != nil {
		return NormalizedProtocolSettings{}, err
	}

	if settings.ChatEndpointPath == "" {
		settings.ChatEndpointPath = defaultChatEndpointPath(settings.ChatProtocol)
	}
	if settings.DiscoveryEndpointPath == "" {
		settings.DiscoveryEndpointPath = defaultDiscoveryEndpointPath(settings.DiscoveryProtocol)
	}

	if settings.LegacyAPIStyle == "" && (normalizedDriver == DriverOpenAICompat || normalizedDriver == DriverGemini) {
		switch settings.ChatProtocol {
		case ChatProtocolOpenAIChatCompletions:
			settings.LegacyAPIStyle = OpenAICompatibleAPIStyleChatCompletions
		case ChatProtocolOpenAIResponses:
			settings.LegacyAPIStyle = OpenAICompatibleAPIStyleResponses
		}
	}

	return settings, nil
}

// applyDriverDefaultProtocolSettings 为不同 driver 注入协议默认值，降低配置输入成本并保证行为稳定。
func applyDriverDefaultProtocolSettings(normalizedDriver string, settings *NormalizedProtocolSettings) {
	if settings == nil {
		return
	}

	switch normalizedDriver {
	case DriverOpenAICompat:
		if settings.ChatProtocol == "" {
			settings.ChatProtocol = ChatProtocolOpenAIChatCompletions
		}
		if settings.DiscoveryProtocol == "" {
			settings.DiscoveryProtocol = DiscoveryProtocolOpenAIModels
		}
		if settings.AuthStrategy == "" {
			settings.AuthStrategy = AuthStrategyBearer
		}
		if settings.ResponseProfile == "" {
			settings.ResponseProfile = DiscoveryResponseProfileOpenAI
		}
	case DriverGemini:
		if settings.ChatProtocol == "" {
			settings.ChatProtocol = ChatProtocolOpenAIChatCompletions
		}
		if settings.DiscoveryProtocol == "" {
			settings.DiscoveryProtocol = DiscoveryProtocolGeminiModels
		}
		if settings.AuthStrategy == "" {
			settings.AuthStrategy = AuthStrategyBearer
		}
		if settings.ResponseProfile == "" {
			settings.ResponseProfile = DiscoveryResponseProfileGemini
		}
	case DriverAnthropic:
		if settings.ChatProtocol == "" {
			settings.ChatProtocol = ChatProtocolAnthropicMessages
		}
		if settings.DiscoveryProtocol == "" {
			settings.DiscoveryProtocol = DiscoveryProtocolAnthropicModels
		}
		if settings.AuthStrategy == "" {
			settings.AuthStrategy = AuthStrategyAnthropic
		}
		if settings.ResponseProfile == "" {
			settings.ResponseProfile = DiscoveryResponseProfileGeneric
		}
	default:
		if settings.ChatProtocol == "" {
			settings.ChatProtocol = ChatProtocolOpenAIChatCompletions
		}
		if settings.DiscoveryProtocol == "" {
			settings.DiscoveryProtocol = DiscoveryProtocolCustomHTTPJSON
		}
		if settings.AuthStrategy == "" {
			settings.AuthStrategy = AuthStrategyBearer
		}
		if settings.ResponseProfile == "" {
			settings.ResponseProfile = DiscoveryResponseProfileGeneric
		}
	}
}

// validateProtocolEnums 校验协议枚举输入，避免拼写错误在运行期才暴露。
func validateProtocolEnums(settings NormalizedProtocolSettings) error {
	switch settings.ChatProtocol {
	case ChatProtocolOpenAIChatCompletions, ChatProtocolOpenAIResponses, ChatProtocolGeminiNative, ChatProtocolAnthropicMessages:
	default:
		return fmt.Errorf("provider chat protocol %q is unsupported", settings.ChatProtocol)
	}

	switch settings.DiscoveryProtocol {
	case DiscoveryProtocolOpenAIModels, DiscoveryProtocolGeminiModels, DiscoveryProtocolAnthropicModels, DiscoveryProtocolCustomHTTPJSON:
	default:
		return fmt.Errorf("provider discovery protocol %q is unsupported", settings.DiscoveryProtocol)
	}

	switch settings.AuthStrategy {
	case AuthStrategyBearer, AuthStrategyXAPIKey, AuthStrategyAnthropic:
	default:
		return fmt.Errorf("provider auth strategy %q is unsupported", settings.AuthStrategy)
	}

	profile, err := NormalizeProviderDiscoveryResponseProfile(settings.ResponseProfile)
	if err != nil {
		return err
	}
	if profile == "" {
		return fmt.Errorf("provider response profile is empty")
	}
	return nil
}

// validateProtocolCombinations 校验协议组合合法性，保证启动阶段 fail-fast。
func validateProtocolCombinations(settings NormalizedProtocolSettings) error {
	if settings.ChatProtocol == ChatProtocolAnthropicMessages && settings.AuthStrategy == AuthStrategyBearer {
		return fmt.Errorf("chat protocol %q does not allow auth strategy %q", settings.ChatProtocol, settings.AuthStrategy)
	}
	if settings.ChatProtocol == ChatProtocolGeminiNative && settings.AuthStrategy == AuthStrategyAnthropic {
		return fmt.Errorf("chat protocol %q does not allow auth strategy %q", settings.ChatProtocol, settings.AuthStrategy)
	}
	return nil
}

// defaultChatEndpointPath 返回不同 chat protocol 的默认端点路径。
func defaultChatEndpointPath(chatProtocol string) string {
	switch strings.TrimSpace(chatProtocol) {
	case ChatProtocolOpenAIResponses:
		return "/responses"
	case ChatProtocolAnthropicMessages:
		return "/messages"
	default:
		return "/chat/completions"
	}
}

// defaultDiscoveryEndpointPath 返回不同 discovery protocol 的默认端点路径。
func defaultDiscoveryEndpointPath(discoveryProtocol string) string {
	switch strings.TrimSpace(discoveryProtocol) {
	case DiscoveryProtocolOpenAIModels, DiscoveryProtocolGeminiModels, DiscoveryProtocolAnthropicModels, DiscoveryProtocolCustomHTTPJSON:
		return DiscoveryEndpointPathModels
	default:
		return ""
	}
}
