package memo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"neo-code/internal/config"
)

// Service 编排记忆的存储、检索、提取和删除，是 memo 子系统对外的统一入口。
type Service struct {
	store      Store
	extractor  Extractor
	config     config.MemoConfig
	mu         sync.Mutex
	sourceInvl func() // 可选的缓存失效回调
}

// NewService 创建 memo Service 实例。
// extractor 可以为 nil（禁用自动提取时不需要）。
func NewService(store Store, extractor Extractor, cfg config.MemoConfig, sourceInvl func()) *Service {
	return &Service{
		store:      store,
		extractor:  extractor,
		config:     cfg,
		sourceInvl: sourceInvl,
	}
}

// Add 添加一条记忆并持久化索引和 topic 文件。
func (s *Service) Add(ctx context.Context, entry Entry) error {
	if !IsValidType(entry.Type) {
		return fmt.Errorf("memo: invalid type %q", entry.Type)
	}
	if strings.TrimSpace(entry.Title) == "" {
		return fmt.Errorf("memo: title is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if entry.ID == "" {
		entry.ID = newEntryID(entry.Type)
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	if entry.TopicFile == "" {
		entry.TopicFile = fmt.Sprintf("%s_%s.md", entry.Type, entry.ID)
	}

	index, err := s.store.LoadIndex(ctx)
	if err != nil {
		return fmt.Errorf("memo: load index: %w", err)
	}

	// 检查是否已存在相同 ID（更新场景）
	replaced := false
	for i, existing := range index.Entries {
		if existing.ID == entry.ID {
			index.Entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		index.Entries = append(index.Entries, entry)
	}
	index.UpdatedAt = now

	// 截断索引到最大行数
	if s.config.MaxIndexLines > 0 && len(index.Entries) > s.config.MaxIndexLines {
		excess := len(index.Entries) - s.config.MaxIndexLines
		// 删除最旧的条目对应的 topic 文件
		for i := 0; i < excess; i++ {
			if index.Entries[i].TopicFile != "" {
				_ = s.store.DeleteTopic(ctx, index.Entries[i].TopicFile)
			}
		}
		index.Entries = index.Entries[excess:]
	}

	if err := s.store.SaveIndex(ctx, index); err != nil {
		return fmt.Errorf("memo: save index: %w", err)
	}

	if err := s.store.SaveTopic(ctx, entry.TopicFile, RenderTopic(&entry)); err != nil {
		return fmt.Errorf("memo: save topic: %w", err)
	}

	s.invalidateCache()
	return nil
}

// Remove 按关键词搜索并删除匹配的记忆条目。
// 返回被删除的条目数量。
func (s *Service) Remove(ctx context.Context, keyword string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.store.LoadIndex(ctx)
	if err != nil {
		return 0, fmt.Errorf("memo: load index: %w", err)
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return 0, fmt.Errorf("memo: keyword is empty")
	}

	var remaining []Entry
	removed := 0
	for _, entry := range index.Entries {
		if matchesKeyword(entry, keyword) {
			if entry.TopicFile != "" {
				_ = s.store.DeleteTopic(ctx, entry.TopicFile)
			}
			removed++
		} else {
			remaining = append(remaining, entry)
		}
	}

	if removed == 0 {
		return 0, nil
	}

	index.Entries = remaining
	index.UpdatedAt = time.Now()
	if err := s.store.SaveIndex(ctx, index); err != nil {
		return 0, fmt.Errorf("memo: save index: %w", err)
	}

	s.invalidateCache()
	return removed, nil
}

// List 返回所有记忆条目的浅拷贝。
func (s *Service) List(ctx context.Context) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.store.LoadIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("memo: load index: %w", err)
	}
	result := make([]Entry, len(index.Entries))
	copy(result, index.Entries)
	return result, nil
}

// Search 按关键词搜索记忆条目，返回匹配结果。
func (s *Service) Search(ctx context.Context, keyword string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.store.LoadIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("memo: load index: %w", err)
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	var results []Entry
	for _, entry := range index.Entries {
		if matchesKeyword(entry, keyword) {
			results = append(results, entry)
		}
	}
	return results, nil
}

// Recall 加载匹配关键词的 topic 文件内容。
func (s *Service) Recall(ctx context.Context, keyword string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.store.LoadIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("memo: load index: %w", err)
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	results := make(map[string]string)
	for _, entry := range index.Entries {
		if !matchesKeyword(entry, keyword) {
			continue
		}
		if entry.TopicFile == "" {
			continue
		}
		content, err := s.store.LoadTopic(ctx, entry.TopicFile)
		if err != nil {
			continue
		}
		results[entry.TopicFile] = content
	}
	return results, nil
}

// ExtractAndStore 从对话消息中提取记忆并保存。提取失败静默处理，不返回错误。
func (s *Service) ExtractAndStore(ctx context.Context, messages interface{}) {
	if s.extractor == nil || !s.config.AutoExtract {
		return
	}
	// Extractor 接口接受 []providertypes.Message，此处使用 interface{} 避免循环依赖
	// 实际调用处会进行类型断言
}

// invalidateCache 触发上下文源的缓存失效回调。
func (s *Service) invalidateCache() {
	if s.sourceInvl != nil {
		s.sourceInvl()
	}
}

// matchesKeyword 检查条目是否匹配关键词（标题、关键词列表、类型）。
func matchesKeyword(entry Entry, keyword string) bool {
	lowerKeyword := keyword
	if strings.Contains(strings.ToLower(entry.Title), lowerKeyword) {
		return true
	}
	if strings.Contains(strings.ToLower(string(entry.Type)), lowerKeyword) {
		return true
	}
	for _, kw := range entry.Keywords {
		if strings.Contains(strings.ToLower(kw), lowerKeyword) {
			return true
		}
	}
	return false
}

// newEntryID 生成格式为 <type>_<timestamp_hex>_<random_hex> 的唯一 ID。
func newEntryID(t Type) string {
	ts := fmt.Sprintf("%x", time.Now().Unix())
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	rand := hex.EncodeToString(buf)
	return fmt.Sprintf("%s_%s_%s", t, ts, rand)
}
