//go:build windows

package runtime

import (
	"fmt"
	"os"
)

// validateTrustStorePermissions 检查 trust store 文件是否为常规文件（Windows）。
func validateTrustStorePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat trust store: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("trust store path is a directory")
	}
	return nil
}
