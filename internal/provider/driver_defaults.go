package provider

// DriverProtocolDefaults 描述不同 driver 在运行时固定采用的协议与鉴权默认值。
type DriverProtocolDefaults struct {
	ChatProtocol      string
	DiscoveryProtocol string
	AuthStrategy      string
	ResponseProfile   string
	APIVersion        string
}

// ResolveDriverProtocolDefaults 根据 driver 返回唯一默认协议配置来源。
func ResolveDriverProtocolDefaults(driver string) DriverProtocolDefaults {
	switch NormalizeProviderDriver(driver) {
	case DriverGemini:
		return DriverProtocolDefaults{
			ChatProtocol:      ChatProtocolGeminiNative,
			DiscoveryProtocol: DiscoveryProtocolGeminiModels,
			AuthStrategy:      AuthStrategyXAPIKey,
			ResponseProfile:   DiscoveryResponseProfileGemini,
		}
	case DriverAnthropic:
		return DriverProtocolDefaults{
			ChatProtocol:      ChatProtocolAnthropicMessages,
			DiscoveryProtocol: DiscoveryProtocolAnthropicModels,
			AuthStrategy:      AuthStrategyAnthropic,
			ResponseProfile:   DiscoveryResponseProfileGeneric,
		}
	case DriverOpenAICompat:
		return DriverProtocolDefaults{
			ChatProtocol:      ChatProtocolOpenAIChatCompletions,
			DiscoveryProtocol: DiscoveryProtocolOpenAIModels,
			AuthStrategy:      AuthStrategyBearer,
			ResponseProfile:   DiscoveryResponseProfileOpenAI,
		}
	default:
		return DriverProtocolDefaults{
			ChatProtocol:      ChatProtocolOpenAIChatCompletions,
			DiscoveryProtocol: DiscoveryProtocolCustomHTTPJSON,
			AuthStrategy:      AuthStrategyBearer,
			ResponseProfile:   DiscoveryResponseProfileGeneric,
		}
	}
}

// ResolveDriverDiscoveryConfig 解析 discovery 请求所需配置，并在端点为空时注入默认值。
func ResolveDriverDiscoveryConfig(driver string, endpointPath string) (string, string, string, error) {
	defaults := ResolveDriverProtocolDefaults(driver)
	normalizedEndpointPath, err := NormalizeProviderDiscoveryEndpointPath(endpointPath)
	if err != nil {
		return "", "", "", err
	}
	if normalizedEndpointPath == "" {
		normalizedEndpointPath = defaultDiscoveryEndpointPath(defaults.DiscoveryProtocol)
	}
	return defaults.DiscoveryProtocol, normalizedEndpointPath, defaults.ResponseProfile, nil
}

// ResolveDriverAuthConfig 返回 driver 对应的鉴权策略与附加 API 版本配置。
func ResolveDriverAuthConfig(driver string) (string, string) {
	defaults := ResolveDriverProtocolDefaults(driver)
	return defaults.AuthStrategy, defaults.APIVersion
}
