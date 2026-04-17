package cli

import (
	"strings"
	"sync"
)

var (
	updateNoticeMu      sync.Mutex
	pendingUpdateNotice string
)

// setUpdateNotice 保存待输出的更新提示，后写入会覆盖先前值。
func setUpdateNotice(notice string) {
	normalized := strings.TrimSpace(notice)
	if normalized == "" {
		return
	}

	updateNoticeMu.Lock()
	pendingUpdateNotice = normalized
	updateNoticeMu.Unlock()
}

// ConsumeUpdateNotice 读取并清空待输出的更新提示，确保只消费一次。
func ConsumeUpdateNotice() string {
	updateNoticeMu.Lock()
	defer updateNoticeMu.Unlock()

	notice := pendingUpdateNotice
	pendingUpdateNotice = ""
	return notice
}
