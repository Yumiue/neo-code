package tools

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"neo-code/internal/security"
)

// SessionPermissionScope 表示 session 级权限记忆的作用范围。
type SessionPermissionScope string

const (
	// SessionPermissionScopeOnce 表示仅当前一次请求放行。
	SessionPermissionScopeOnce SessionPermissionScope = "once"
	// SessionPermissionScopeAlways 表示当前会话内同类请求持续放行。
	SessionPermissionScopeAlways SessionPermissionScope = "always_session"
	// SessionPermissionScopeReject 表示当前会话内同类请求持续拒绝。
	SessionPermissionScopeReject SessionPermissionScope = "reject"
)

type sessionPermissionEntry struct {
	decision  security.Decision
	scope     SessionPermissionScope
	remaining int
}

// sessionPermissionMemory 管理按 session/action 维度的审批记忆。
type sessionPermissionMemory struct {
	mu      sync.Mutex
	entries map[string]map[string]sessionPermissionEntry
}

// newSessionPermissionMemory 创建 session 级权限记忆存储。
func newSessionPermissionMemory() *sessionPermissionMemory {
	return &sessionPermissionMemory{
		entries: make(map[string]map[string]sessionPermissionEntry),
	}
}

// remember 记录一条 session 级权限决策。
func (m *sessionPermissionMemory) remember(sessionID string, action security.Action, scope SessionPermissionScope) error {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return errors.New("tools: session id is empty")
	}
	if err := action.Validate(); err != nil {
		return err
	}

	var entry sessionPermissionEntry
	switch scope {
	case SessionPermissionScopeOnce:
		entry = sessionPermissionEntry{
			decision:  security.DecisionAllow,
			scope:     scope,
			remaining: 1,
		}
	case SessionPermissionScopeAlways:
		entry = sessionPermissionEntry{
			decision:  security.DecisionAllow,
			scope:     scope,
			remaining: -1,
		}
	case SessionPermissionScopeReject:
		entry = sessionPermissionEntry{
			decision:  security.DecisionDeny,
			scope:     scope,
			remaining: -1,
		}
	default:
		return fmt.Errorf("tools: unsupported session permission scope %q", scope)
	}

	actionKey := sessionPermissionActionKey(action)
	m.mu.Lock()
	defer m.mu.Unlock()
	sessionEntries, ok := m.entries[trimmedSessionID]
	if !ok {
		sessionEntries = make(map[string]sessionPermissionEntry)
		m.entries[trimmedSessionID] = sessionEntries
	}
	sessionEntries[actionKey] = entry
	return nil
}

// resolve 查询并按 scope 规则消费 session 级权限记忆。
func (m *sessionPermissionMemory) resolve(sessionID string, action security.Action) (security.Decision, SessionPermissionScope, bool) {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return "", "", false
	}
	actionKey := sessionPermissionActionKey(action)

	m.mu.Lock()
	defer m.mu.Unlock()

	sessionEntries, ok := m.entries[trimmedSessionID]
	if !ok {
		return "", "", false
	}
	entry, ok := sessionEntries[actionKey]
	if !ok {
		return "", "", false
	}

	if entry.scope == SessionPermissionScopeOnce && entry.remaining > 0 {
		entry.remaining--
		if entry.remaining <= 0 {
			delete(sessionEntries, actionKey)
		} else {
			sessionEntries[actionKey] = entry
		}
	}

	if len(sessionEntries) == 0 {
		delete(m.entries, trimmedSessionID)
	}

	return entry.decision, entry.scope, true
}

// sessionPermissionActionKey 基于结构化 action 生成稳定匹配键。
func sessionPermissionActionKey(action security.Action) string {
	return strings.Join([]string{
		string(action.Type),
		sessionPermissionCategory(action),
	}, "|")
}

// sessionPermissionCategory 将安全动作归一为稳定的工具类别。
// 类别用于 once/always/reject 的 session 级记忆，不再按具体 target 区分。
func sessionPermissionCategory(action security.Action) string {
	resource := strings.ToLower(strings.TrimSpace(action.Payload.Resource))
	switch action.Type {
	case security.ActionTypeRead:
		if strings.HasPrefix(resource, "filesystem_") {
			return "filesystem_read"
		}
		if resource == "webfetch" {
			return "webfetch"
		}
	case security.ActionTypeWrite:
		if strings.HasPrefix(resource, "filesystem_") {
			return "filesystem_write"
		}
	case security.ActionTypeBash:
		return "bash"
	case security.ActionTypeMCP:
		target := strings.ToLower(strings.TrimSpace(action.Payload.Target))
		if target != "" {
			return "mcp:" + target
		}
		return "mcp"
	}

	toolName := strings.ToLower(strings.TrimSpace(action.Payload.ToolName))
	if toolName != "" {
		return toolName
	}
	return resource
}
