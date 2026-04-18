package discovery

import "strings"

// ResolveEndpoint 将归一化后的相对端点路径拼接到 baseURL，避免上层重复处理路径前后缀。
func ResolveEndpoint(baseURL string, endpointPath string) string {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	trimmedEndpointPath := strings.TrimSpace(endpointPath)
	if trimmedEndpointPath == "" {
		return trimmedBaseURL
	}
	if !strings.HasPrefix(trimmedEndpointPath, "/") {
		trimmedEndpointPath = "/" + trimmedEndpointPath
	}
	return trimmedBaseURL + trimmedEndpointPath
}
