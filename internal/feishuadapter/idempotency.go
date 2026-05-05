package feishuadapter

import (
	"strings"
	"sync"
	"time"
)

type idempotencyStore struct {
	ttl   time.Duration
	mu    sync.Mutex
	items map[string]time.Time
}

// newIdempotencyStore 创建带 TTL 的内存去重存储。
func newIdempotencyStore(ttl time.Duration) *idempotencyStore {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &idempotencyStore{
		ttl:   ttl,
		items: make(map[string]time.Time),
	}
}

// Seen 会在首次看到 key 时返回 false，TTL 窗口内重复看到返回 true。
func (s *idempotencyStore) Seen(key string, now time.Time) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	expireAt, exists := s.items[trimmed]
	if exists && expireAt.After(now) {
		return true
	}
	s.items[trimmed] = now.Add(s.ttl)
	return false
}

// cleanupLocked 清理已经过期的去重键。
func (s *idempotencyStore) cleanupLocked(now time.Time) {
	if len(s.items) == 0 {
		return
	}
	for key, expireAt := range s.items {
		if expireAt.After(now) {
			continue
		}
		delete(s.items, key)
	}
}
