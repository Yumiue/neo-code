package anthropic

import (
	"context"
	"testing"

	"neo-code/internal/provider"
)

func TestDriverBuildRejectsUnsupportedAnthropicMessages(t *testing.T) {
	t.Parallel()

	driver := Driver()
	_, err := driver.Build(context.Background(), provider.RuntimeConfig{
		Driver:       DriverName,
		BaseURL:      "https://api.anthropic.com/v1",
		APIKey:       "test-key",
		ChatProtocol: provider.ChatProtocolAnthropicMessages,
		AuthStrategy: provider.AuthStrategyAnthropic,
	})
	if err == nil {
		t.Fatal("expected unsupported anthropic messages error")
	}
}
