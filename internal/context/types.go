package context

import (
	"context"

	"neo-code/internal/provider"
	"neo-code/internal/tools"
)

// Builder builds the provider-facing context for a single model round.
type Builder interface {
	Build(ctx context.Context, input BuildInput) (BuildResult, error)
}

// BuildInput contains the runtime state needed to assemble model context.
type BuildInput struct {
	Messages []provider.Message
	Metadata Metadata
	Compact  CompactOptions
}

// BuildResult is the provider-facing context produced for a single round.
type BuildResult struct {
	SystemPrompt string
	Messages     []provider.Message
}

// MicroCompactPolicySource 定义 context 读取工具 micro compact 策略的最小依赖。
type MicroCompactPolicySource interface {
	MicroCompactPolicy(name string) tools.MicroCompactPolicy
}

// CompactOptions controls read-time compact behavior inside the context builder.
type CompactOptions struct {
	DisableMicroCompact bool
}
