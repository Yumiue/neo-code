package memo

import (
	"context"
	"sync"
	"time"

	agentcontext "neo-code/internal/context"
)

// memoContextSource 将持久化记忆作为 prompt section 注入上下文构建器。
// 它实现 agentcontext.SectionSource 接口，仅加载 MEMO.md 索引内容，
// topic 文件的详细内容通过 memo_recall 工具按需加载。
type memoContextSource struct {
	store      Store
	mu         sync.RWMutex
	cachedText string
	cacheTime  time.Time
	ttl        time.Duration
}

// MemoContextSourceOption 配置 memoContextSource 的可选参数。
type MemoContextSourceOption func(*memoContextSource)

// WithCacheTTL 设置索引缓存的存活时间，默认 5 秒。
func WithCacheTTL(ttl time.Duration) MemoContextSourceOption {
	return func(s *memoContextSource) {
		s.ttl = ttl
	}
}

// NewContextSource 创建注入记忆到上下文的 SectionSource 实现。
func NewContextSource(store Store, opts ...MemoContextSourceOption) agentcontext.SectionSource {
	s := &memoContextSource{
		store: store,
		ttl:   5 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Sections 实现 agentcontext.SectionSource，返回记忆索引作为 prompt section。
func (s *memoContextSource) Sections(ctx context.Context, input agentcontext.BuildInput) ([]agentcontext.PromptSection, error) {
	_ = input // 当前不依赖 BuildInput

	text, err := s.loadCached(ctx)
	if err != nil {
		// 记忆加载失败不应阻断上下文构建，返回空 section
		return nil, nil
	}
	if text == "" {
		return nil, nil
	}

	return []agentcontext.PromptSection{
		agentcontext.NewPromptSection("Memo", text),
	}, nil
}

// loadCached 带缓存地加载 MEMO.md 内容。
func (s *memoContextSource) loadCached(ctx context.Context) (string, error) {
	s.mu.RLock()
	if s.cachedText != "" && time.Since(s.cacheTime) < s.ttl {
		text := s.cachedText
		s.mu.RUnlock()
		return text, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 双重检查
	if s.cachedText != "" && time.Since(s.cacheTime) < s.ttl {
		return s.cachedText, nil
	}

	index, err := s.store.LoadIndex(ctx)
	if err != nil {
		return "", err
	}

	text := RenderIndex(index)
	s.cachedText = text
	s.cacheTime = time.Now()
	return text, nil
}

// InvalidateCache 使缓存失效，用于记忆变更后立即生效。
func (s *memoContextSource) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedText = ""
	s.cacheTime = time.Time{}
}
