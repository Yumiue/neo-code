package bootstrap

import "strings"

// Mode 定义 TUI bootstrap 的装配模式。
type Mode string

const (
	// ModeLive 表示使用真实依赖进行正常装配。
	ModeLive Mode = "live"
	// ModeOffline 表示使用离线装配策略（可由工厂映射为本地实现）。
	ModeOffline Mode = "offline"
	// ModeMock 表示使用 mock 装配策略（通常用于测试）。
	ModeMock Mode = "mock"
)

// NormalizeMode 归一化 mode 输入，未知值默认回退到 live。
func NormalizeMode(mode Mode) Mode {
	switch Mode(strings.ToLower(strings.TrimSpace(string(mode)))) {
	case ModeOffline:
		return ModeOffline
	case ModeMock:
		return ModeMock
	default:
		return ModeLive
	}
}
