package openaicompat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"neo-code/internal/provider/openaicompat/shared"
)

// openAIModelsResponse 表示 /models 端点的响应结构。
type openAIModelsResponse struct {
	Data any `json:"data"`
}

// decodeModelsData 兼容解析 data 字段，支持数组与单对象两种常见返回格式。
func decodeModelsData(data any) ([]map[string]any, error) {
	switch value := data.(type) {
	case nil:
		return nil, nil
	case []any:
		models := make([]map[string]any, 0, len(value))
		for _, item := range value {
			model, ok := item.(map[string]any)
			if !ok {
				continue
			}
			models = append(models, model)
		}
		return models, nil
	case map[string]any:
		return []map[string]any{value}, nil
	default:
		return nil, fmt.Errorf("unsupported models data type %T", data)
	}
}

// fetchModels 从 OpenAI 兼容的 /models 端点获取原始模型列表。
func (p *Provider) fetchModels(ctx context.Context) ([]map[string]any, error) {
	endpoint := shared.Endpoint(p.cfg.BaseURL, "/models")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%sbuild models request: %w", shared.ErrorPrefix, err)
	}
	req.Header.Set("Accept", "application/json")
	shared.SetBearerAuthorization(req.Header, p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%ssend models request: %w", shared.ErrorPrefix, err)
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(resp.Body)
		body := strings.TrimSpace(string(data))
		if body == "" {
			body = resp.Status
		}
		return nil, fmt.Errorf("%smodels endpoint %s", shared.ErrorPrefix, body)
	}

	var payload openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%sdecode models response: %w", shared.ErrorPrefix, err)
	}
	models, err := decodeModelsData(payload.Data)
	if err != nil {
		return nil, fmt.Errorf("%sdecode models response: %w", shared.ErrorPrefix, err)
	}
	return models, nil
}
