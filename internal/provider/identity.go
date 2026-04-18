package provider

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// ProviderIdentity 标识 discovery、缓存与去重所使用的具体 provider 连接身份。
type ProviderIdentity struct {
	Driver                   string `json:"driver"`
	BaseURL                  string `json:"base_url"`
	ChatProtocol             string `json:"chat_protocol,omitempty"`
	ChatEndpointPath         string `json:"chat_endpoint_path,omitempty"`
	DiscoveryProtocol        string `json:"discovery_protocol,omitempty"`
	AuthStrategy             string `json:"auth_strategy,omitempty"`
	ResponseProfile          string `json:"response_profile,omitempty"`
	APIStyle                 string `json:"api_style,omitempty"`
	DeploymentMode           string `json:"deployment_mode,omitempty"`
	APIVersion               string `json:"api_version,omitempty"`
	DiscoveryEndpointPath    string `json:"discovery_endpoint_path,omitempty"`
	DiscoveryResponseProfile string `json:"discovery_response_profile,omitempty"`
}

// Key 返回稳定的 provider 身份键，供缓存文件命名与去重逻辑复用。
func (i ProviderIdentity) Key() string {
	parts := []string{i.Driver, i.BaseURL}
	if strings.TrimSpace(i.ChatProtocol) != "" {
		parts = append(parts, i.ChatProtocol)
	}
	if strings.TrimSpace(i.ChatEndpointPath) != "" {
		parts = append(parts, i.ChatEndpointPath)
	}
	if strings.TrimSpace(i.DiscoveryProtocol) != "" {
		parts = append(parts, i.DiscoveryProtocol)
	}
	if strings.TrimSpace(i.AuthStrategy) != "" {
		parts = append(parts, i.AuthStrategy)
	}
	if strings.TrimSpace(i.ResponseProfile) != "" {
		parts = append(parts, i.ResponseProfile)
	}
	if strings.TrimSpace(i.APIStyle) != "" {
		parts = append(parts, i.APIStyle)
	}
	if strings.TrimSpace(i.DeploymentMode) != "" {
		parts = append(parts, i.DeploymentMode)
	}
	if strings.TrimSpace(i.APIVersion) != "" {
		parts = append(parts, i.APIVersion)
	}
	if strings.TrimSpace(i.DiscoveryEndpointPath) != "" {
		parts = append(parts, i.DiscoveryEndpointPath)
	}
	if strings.TrimSpace(i.DiscoveryResponseProfile) != "" {
		parts = append(parts, i.DiscoveryResponseProfile)
	}
	return strings.Join(parts, "|")
}

// String 返回可读的 provider 身份字符串，便于错误信息与日志复用。
func (i ProviderIdentity) String() string {
	return i.Key()
}

// NormalizeKey 统一执行大小写折叠与空白裁剪，保证跨层比较稳定。
func NormalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// NormalizeProviderDriver 规范化 driver 名称，避免大小写与空白导致身份漂移。
func NormalizeProviderDriver(driver string) string {
	return NormalizeKey(driver)
}

// NormalizeProviderAPIStyle 规范化 openaicompat 的 api_style，用于稳定生成连接身份。
func NormalizeProviderAPIStyle(apiStyle string) string {
	return NormalizeKey(apiStyle)
}

// NormalizeProviderDeploymentMode 规范化 Gemini deployment_mode，避免大小写与空白导致误判。
func NormalizeProviderDeploymentMode(mode string) string {
	return NormalizeKey(mode)
}

// NormalizeProviderAPIVersion 规范化 Anthropic api_version，用于稳定生成连接身份。
func NormalizeProviderAPIVersion(version string) string {
	return NormalizeKey(version)
}

// NormalizeProviderDiscoveryEndpointPath 规范化模型发现端点路径，拒绝包含主机信息或查询参数的配置。
func NormalizeProviderDiscoveryEndpointPath(endpointPath string) (string, error) {
	value := strings.TrimSpace(endpointPath)
	if value == "" {
		return "", nil
	}
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("provider discovery endpoint path %q is invalid", endpointPath)
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("provider discovery endpoint path %q is invalid: %w", endpointPath, err)
	}
	if parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(value, "//") {
		return "", fmt.Errorf("provider discovery endpoint path %q must be a relative path", endpointPath)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("provider discovery endpoint path %q must not contain query or fragment", endpointPath)
	}

	segments := strings.Split(value, "/")
	for _, segment := range segments {
		if strings.TrimSpace(segment) == ".." {
			return "", fmt.Errorf("provider discovery endpoint path %q must not contain '..'", endpointPath)
		}
	}

	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	cleaned := path.Clean(value)
	if !strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("provider discovery endpoint path %q is invalid", endpointPath)
	}
	if cleaned != "/" {
		cleaned = strings.TrimRight(cleaned, "/")
	}
	return cleaned, nil
}

// NormalizeProviderDiscoveryResponseProfile 规范化模型发现响应解析策略，仅允许受支持的 profile。
func NormalizeProviderDiscoveryResponseProfile(profile string) (string, error) {
	normalized := NormalizeKey(profile)
	if normalized == "" {
		return "", nil
	}

	switch normalized {
	case DiscoveryResponseProfileOpenAI, DiscoveryResponseProfileGemini, DiscoveryResponseProfileGeneric:
		return normalized, nil
	default:
		return "", fmt.Errorf("provider discovery response profile %q is unsupported", profile)
	}
}

// NormalizeProviderDiscoverySettings 根据 driver 归一化 discovery 设置，并在受支持场景补齐默认值。
func NormalizeProviderDiscoverySettings(
	driver string,
	endpointPath string,
	responseProfile string,
) (string, string, error) {
	normalizedDriver := NormalizeProviderDriver(driver)
	candidateEndpointPath := strings.TrimSpace(endpointPath)
	candidateResponseProfile := strings.TrimSpace(responseProfile)

	// 仅为 openaicompat 提供默认发现配置，其他 driver 维持显式输入。
	if normalizedDriver == DriverOpenAICompat {
		if candidateEndpointPath == "" {
			candidateEndpointPath = DiscoveryEndpointPathModels
		}
		if candidateResponseProfile == "" {
			candidateResponseProfile = DiscoveryResponseProfileOpenAI
		}
	}

	normalizedEndpointPath, err := NormalizeProviderDiscoveryEndpointPath(candidateEndpointPath)
	if err != nil {
		return "", "", err
	}
	normalizedResponseProfile, err := NormalizeProviderDiscoveryResponseProfile(candidateResponseProfile)
	if err != nil {
		return "", "", err
	}
	return normalizedEndpointPath, normalizedResponseProfile, nil
}

// NormalizeProviderBaseURL 将 provider 接入地址规整为可比较的稳定形式。
func NormalizeProviderBaseURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("provider base_url is empty")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("provider base_url %q is invalid: %w", raw, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("provider base_url %q must include scheme and host", raw)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil

	if cleaned := path.Clean(strings.TrimSpace(parsed.Path)); cleaned == "." {
		parsed.Path = ""
	} else {
		parsed.Path = strings.TrimRight(cleaned, "/")
	}

	return parsed.String(), nil
}

// NewProviderIdentity 根据 driver 与 base_url 构造基础 provider 身份。
func NewProviderIdentity(driver string, baseURL string) (ProviderIdentity, error) {
	normalizedDriver := NormalizeProviderDriver(driver)
	if normalizedDriver == "" {
		return ProviderIdentity{}, fmt.Errorf("provider driver is empty")
	}

	normalizedBaseURL, err := NormalizeProviderBaseURL(baseURL)
	if err != nil {
		return ProviderIdentity{}, err
	}

	return ProviderIdentity{
		Driver:  normalizedDriver,
		BaseURL: normalizedBaseURL,
	}, nil
}

// NormalizeProviderIdentity 根据 driver 规则规范化连接身份，保留参与缓存去重的专用字段。
func NormalizeProviderIdentity(identity ProviderIdentity) (ProviderIdentity, error) {
	normalizedDriver := NormalizeProviderDriver(identity.Driver)
	if normalizedDriver == "" {
		return ProviderIdentity{}, fmt.Errorf("provider driver is empty")
	}

	switch normalizedDriver {
	case DriverOpenAICompat:
		baseURL, err := NormalizeProviderBaseURL(identity.BaseURL)
		if err != nil {
			return ProviderIdentity{}, err
		}
		normalizedProtocols, err := NormalizeProviderProtocolSettings(
			identity.Driver,
			identity.ChatProtocol,
			identity.ChatEndpointPath,
			identity.DiscoveryProtocol,
			identity.DiscoveryEndpointPath,
			identity.AuthStrategy,
			identity.ResponseProfile,
			identity.APIStyle,
			identity.DiscoveryResponseProfile,
		)
		if err != nil {
			return ProviderIdentity{}, err
		}
		return ProviderIdentity{
			Driver:                   normalizedDriver,
			BaseURL:                  baseURL,
			ChatProtocol:             normalizedProtocols.ChatProtocol,
			ChatEndpointPath:         normalizedProtocols.ChatEndpointPath,
			DiscoveryProtocol:        normalizedProtocols.DiscoveryProtocol,
			AuthStrategy:             normalizedProtocols.AuthStrategy,
			ResponseProfile:          normalizedProtocols.ResponseProfile,
			APIStyle:                 normalizedProtocols.LegacyAPIStyle,
			DiscoveryEndpointPath:    normalizedProtocols.DiscoveryEndpointPath,
			DiscoveryResponseProfile: normalizedProtocols.ResponseProfile,
		}, nil
	case DriverGemini:
		baseURL, err := NormalizeProviderBaseURL(identity.BaseURL)
		if err != nil {
			return ProviderIdentity{}, err
		}
		normalizedProtocols, err := NormalizeProviderProtocolSettings(
			identity.Driver,
			identity.ChatProtocol,
			identity.ChatEndpointPath,
			identity.DiscoveryProtocol,
			identity.DiscoveryEndpointPath,
			identity.AuthStrategy,
			identity.ResponseProfile,
			identity.APIStyle,
			identity.DiscoveryResponseProfile,
		)
		if err != nil {
			return ProviderIdentity{}, err
		}
		return ProviderIdentity{
			Driver:                   normalizedDriver,
			BaseURL:                  baseURL,
			ChatProtocol:             normalizedProtocols.ChatProtocol,
			ChatEndpointPath:         normalizedProtocols.ChatEndpointPath,
			DiscoveryProtocol:        normalizedProtocols.DiscoveryProtocol,
			AuthStrategy:             normalizedProtocols.AuthStrategy,
			ResponseProfile:          normalizedProtocols.ResponseProfile,
			APIStyle:                 normalizedProtocols.LegacyAPIStyle,
			DeploymentMode:           NormalizeProviderDeploymentMode(identity.DeploymentMode),
			DiscoveryEndpointPath:    normalizedProtocols.DiscoveryEndpointPath,
			DiscoveryResponseProfile: normalizedProtocols.ResponseProfile,
		}, nil
	case DriverAnthropic:
		baseURL, err := NormalizeProviderBaseURL(identity.BaseURL)
		if err != nil {
			return ProviderIdentity{}, err
		}
		normalizedProtocols, err := NormalizeProviderProtocolSettings(
			identity.Driver,
			identity.ChatProtocol,
			identity.ChatEndpointPath,
			identity.DiscoveryProtocol,
			identity.DiscoveryEndpointPath,
			identity.AuthStrategy,
			identity.ResponseProfile,
			identity.APIStyle,
			identity.DiscoveryResponseProfile,
		)
		if err != nil {
			return ProviderIdentity{}, err
		}
		return ProviderIdentity{
			Driver:                   normalizedDriver,
			BaseURL:                  baseURL,
			ChatProtocol:             normalizedProtocols.ChatProtocol,
			ChatEndpointPath:         normalizedProtocols.ChatEndpointPath,
			DiscoveryProtocol:        normalizedProtocols.DiscoveryProtocol,
			AuthStrategy:             normalizedProtocols.AuthStrategy,
			ResponseProfile:          normalizedProtocols.ResponseProfile,
			APIStyle:                 normalizedProtocols.LegacyAPIStyle,
			APIVersion:               NormalizeProviderAPIVersion(identity.APIVersion),
			DiscoveryEndpointPath:    normalizedProtocols.DiscoveryEndpointPath,
			DiscoveryResponseProfile: normalizedProtocols.ResponseProfile,
		}, nil
	default:
		baseURL, err := NormalizeProviderBaseURL(identity.BaseURL)
		if err != nil {
			return ProviderIdentity{}, err
		}
		discoveryEndpointPath, discoveryResponseProfile, err := NormalizeProviderDiscoverySettings(
			identity.Driver,
			identity.DiscoveryEndpointPath,
			identity.DiscoveryResponseProfile,
		)
		if err != nil {
			return ProviderIdentity{}, err
		}
		return ProviderIdentity{
			Driver:                   normalizedDriver,
			BaseURL:                  baseURL,
			DiscoveryEndpointPath:    discoveryEndpointPath,
			DiscoveryResponseProfile: discoveryResponseProfile,
		}, nil
	}
}
