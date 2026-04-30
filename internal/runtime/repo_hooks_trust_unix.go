//go:build !windows

package runtime

import (
	"fmt"
	"os"
)

// validateTrustStorePermissions 检查 trust store 文件权限是否安全（Unix）。
// 要求文件不能被 other 用户写入。
func validateTrustStorePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat trust store: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("trust store is not a regular file")
	}
	if info.Mode().Perm()&0002 != 0 {
		return fmt.Errorf("trust store is world-writable (mode %o)", info.Mode().Perm())
	}
	return nil
}
