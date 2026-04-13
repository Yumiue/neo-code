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
	store                  Store
	extractor              Extractor
	config                 config.MemoConfig
	mu                     sync.Mutex
	sourceInvl             func()
	autoExtractIndexMu     sync.Mutex
	autoExtractIndexReady  bool
	autoExtractKeysByTopic map[string]string
	autoExtractKeyRefs     map[string]int
}

// NewService 创建 memo Service 实例；extractor 可以为 nil。
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
	entry, err := normalizeEntryForPersist(entry)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveEntryLocked(ctx, entry)
}

// addAutoExtractIfAbsent 在同一把锁内完成自动提取条目的查重与写入。
func (s *Service) addAutoExtractIfAbsent(ctx context.Context, entry Entry) (bool, error) {
	entry, err := normalizeEntryForPersist(entry)
	if err != nil {
		return false, err
	}
	if err := s.ensureAutoExtractIndex(ctx); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hasExactAutoExtractLocked(entry) {
		return false, nil
	}

	if err := s.saveEntryLocked(ctx, entry); err != nil {
		return false, err
	}
	return true, nil
}

// saveEntryLocked 在持有 Service 锁的前提下持久化单条记忆及索引。
func (s *Service) saveEntryLocked(ctx context.Context, entry Entry) error {
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
	working := cloneIndex(index)

	replaced := false
	var previous Entry
	for i, existing := range working.Entries {
		if existing.ID == entry.ID {
			previous = existing
			working.Entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		working.Entries = append(working.Entries, entry)
	}
	working.UpdatedAt = now

	var topicsToDelete []string
	if s.config.MaxIndexLines > 0 && len(working.Entries) > s.config.MaxIndexLines {
		excess := len(working.Entries) - s.config.MaxIndexLines
		for i := 0; i < excess; i++ {
			topicFile := strings.TrimSpace(working.Entries[i].TopicFile)
			if topicFile != "" && topicFile != entry.TopicFile {
				topicsToDelete = append(topicsToDelete, topicFile)
			}
		}
		working.Entries = working.Entries[excess:]
	}

	if err := s.store.SaveTopic(ctx, entry.TopicFile, RenderTopic(&entry)); err != nil {
		return fmt.Errorf("memo: save topic: %w", err)
	}
	if err := s.store.SaveIndex(ctx, working); err != nil {
		if !replaced {
			_ = s.store.DeleteTopic(ctx, entry.TopicFile)
		}
		return fmt.Errorf("memo: save index: %w", err)
	}
	for _, topicFile := range topicsToDelete {
		_ = s.store.DeleteTopic(ctx, topicFile)
	}
	if s.autoExtractIndexReady {
		if replaced {
			s.removeAutoExtractTopicLocked(previous.TopicFile)
		}
		for _, topicFile := range topicsToDelete {
			s.removeAutoExtractTopicLocked(topicFile)
		}
		s.trackAutoExtractEntryLocked(entry)
	}

	s.invalidateCache()
	return nil
}

// ensureAutoExtractIndex 在锁外预加载自动提取的精确去重索引，避免重复在主锁内扫 topic 文件。
func (s *Service) ensureAutoExtractIndex(ctx context.Context) error {
	s.mu.Lock()
	if s.autoExtractIndexReady {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	s.autoExtractIndexMu.Lock()
	defer s.autoExtractIndexMu.Unlock()

	s.mu.Lock()
	if s.autoExtractIndexReady {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	index, err := s.store.LoadIndex(ctx)
	if err != nil {
		return fmt.Errorf("memo: load index: %w", err)
	}

	keysByTopic := make(map[string]string)
	keyRefs := make(map[string]int)
	for _, entry := range index.Entries {
		topicFile := strings.TrimSpace(entry.TopicFile)
		if topicFile == "" {
			continue
		}

		topicContent, err := s.store.LoadTopic(ctx, topicFile)
		if err != nil {
			continue
		}

		source, content := parseTopicSourceAndContent(topicContent)
		if source != SourceAutoExtract {
			continue
		}

		entry.Source = source
		entry.Content = content
		key := autoExtractDedupKey(entry)
		if key == "" {
			continue
		}

		keysByTopic[topicFile] = key
		keyRefs[key]++
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.autoExtractIndexReady {
		return nil
	}
	s.autoExtractKeysByTopic = keysByTopic
	s.autoExtractKeyRefs = keyRefs
	s.autoExtractIndexReady = true
	return nil
}

// hasExactAutoExtractLocked 检查是否已存在完全相同的自动提取记忆。
func (s *Service) hasExactAutoExtractLocked(target Entry) bool {
	targetKey := autoExtractDedupKey(target)
	if targetKey == "" {
		return false
	}
	return s.autoExtractKeyRefs[targetKey] > 0
}

// trackAutoExtractEntryLocked 在自动提取索引已就绪时维护单条条目的去重键。
func (s *Service) trackAutoExtractEntryLocked(entry Entry) {
	if !s.autoExtractIndexReady {
		return
	}

	topicFile := strings.TrimSpace(entry.TopicFile)
	if topicFile == "" {
		return
	}
	s.removeAutoExtractTopicLocked(topicFile)

	if entry.Source != SourceAutoExtract {
		return
	}

	key := autoExtractDedupKey(entry)
	if key == "" {
		return
	}

	s.autoExtractKeysByTopic[topicFile] = key
	s.autoExtractKeyRefs[key]++
}

// removeAutoExtractTopicLocked 从精确去重索引中移除指定 topic 的记录。
func (s *Service) removeAutoExtractTopicLocked(topicFile string) {
	if !s.autoExtractIndexReady {
		return
	}

	topicFile = strings.TrimSpace(topicFile)
	if topicFile == "" {
		return
	}

	key, ok := s.autoExtractKeysByTopic[topicFile]
	if !ok {
		return
	}
	delete(s.autoExtractKeysByTopic, topicFile)

	if refs := s.autoExtractKeyRefs[key]; refs > 1 {
		s.autoExtractKeyRefs[key] = refs - 1
		return
	}
	delete(s.autoExtractKeyRefs, key)
}

// loadIndexLocked 在持有锁的状态下加载索引。
func (s *Service) loadIndexLocked(ctx context.Context) (*Index, error) {
	index, err := s.store.LoadIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("memo: load index: %w", err)
	}
	return index, nil
}

// Remove 按关键词搜索并删除匹配的记忆条目，返回删除数量。
func (s *Service) Remove(ctx context.Context, keyword string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndexLocked(ctx)
	if err != nil {
		return 0, err
	}
	working := cloneIndex(index)

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return 0, fmt.Errorf("memo: keyword is empty")
	}

	var remaining []Entry
	removed := 0
	topicsToDelete := make([]string, 0, len(working.Entries))
	for _, entry := range working.Entries {
		if matchesKeyword(entry, keyword) {
			if topicFile := strings.TrimSpace(entry.TopicFile); topicFile != "" {
				topicsToDelete = append(topicsToDelete, topicFile)
			}
			removed++
		} else {
			remaining = append(remaining, entry)
		}
	}

	if removed == 0 {
		return 0, nil
	}

	working.Entries = remaining
	working.UpdatedAt = time.Now()
	if err := s.store.SaveIndex(ctx, working); err != nil {
		return 0, fmt.Errorf("memo: save index: %w", err)
	}
	for _, topicFile := range topicsToDelete {
		_ = s.store.DeleteTopic(ctx, topicFile)
	}
	if s.autoExtractIndexReady {
		for _, topicFile := range topicsToDelete {
			s.removeAutoExtractTopicLocked(topicFile)
		}
	}

	s.invalidateCache()
	return removed, nil
}

// List 返回索引中的所有记忆条目浅拷贝。
func (s *Service) List(ctx context.Context) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndexLocked(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, len(index.Entries))
	copy(result, index.Entries)
	return result, nil
}

// Search 按关键词搜索记忆条目。
func (s *Service) Search(ctx context.Context, keyword string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndexLocked(ctx)
	if err != nil {
		return nil, err
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

	index, err := s.loadIndexLocked(ctx)
	if err != nil {
		return nil, err
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

// invalidateCache 触发上下文源缓存失效回调。
func (s *Service) invalidateCache() {
	if s.sourceInvl != nil {
		s.sourceInvl()
	}
}

// matchesKeyword 检查条目是否匹配关键词。
func matchesKeyword(entry Entry, keyword string) bool {
	if strings.Contains(strings.ToLower(entry.Title), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(string(entry.Type)), keyword) {
		return true
	}
	for _, kw := range entry.Keywords {
		if strings.Contains(strings.ToLower(kw), keyword) {
			return true
		}
	}
	return false
}

// normalizeEntryForPersist 统一校验并标准化写入前的记忆条目。
func normalizeEntryForPersist(entry Entry) (Entry, error) {
	if !IsValidType(entry.Type) {
		return Entry{}, fmt.Errorf("memo: invalid type %q", entry.Type)
	}
	entry.Title = NormalizeTitle(entry.Title)
	if entry.Title == "" {
		return Entry{}, fmt.Errorf("memo: title is empty")
	}
	return entry, nil
}

// newEntryID 生成格式为 <type>_<timestamp_hex>_<random_hex> 的唯一 ID。
func newEntryID(t Type) string {
	ts := fmt.Sprintf("%x", time.Now().Unix())
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	randHex := hex.EncodeToString(buf)
	return fmt.Sprintf("%s_%s_%s", t, ts, randHex)
}

// cloneIndex 复制索引结构，避免持久化失败时污染原始数据引用。
func cloneIndex(index *Index) *Index {
	if index == nil {
		return &Index{}
	}
	cloned := &Index{
		Entries:   make([]Entry, len(index.Entries)),
		UpdatedAt: index.UpdatedAt,
	}
	copy(cloned.Entries, index.Entries)
	return cloned
}
