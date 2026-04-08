package session

import (
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"strings"
)

const projectsDirName = "projects"

// sessionDirectory 负责根据工作区根目录计算会话分桶目录。
func sessionDirectory(baseDir string, workspaceRoot string) string {
	return filepath.Join(baseDir, projectsDirName, hashWorkspaceRoot(workspaceRoot), sessionsDirName)
}

// hashWorkspaceRoot 负责为规范化后的工作区根目录生成稳定哈希。
func hashWorkspaceRoot(workspaceRoot string) string {
	key := workspacePathKey(workspaceRoot)
	if key == "" {
		key = "unknown"
	}
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:8])
}

// workspacePathKey 负责生成工作区路径的稳定比较键，并与项目级 transcript 哈希规则保持一致。
func workspacePathKey(workspaceRoot string) string {
	normalized := normalizeWorkspaceRoot(workspaceRoot)
	if normalized == "" {
		return ""
	}
	return strings.ToLower(normalized)
}

// normalizeWorkspaceRoot 负责将工作区根目录规范化为绝对清洗路径。
func normalizeWorkspaceRoot(workspaceRoot string) string {
	trimmed := strings.TrimSpace(workspaceRoot)
	if trimmed == "" {
		return ""
	}

	absolute, err := filepath.Abs(trimmed)
	if err == nil {
		trimmed = absolute
	}
	return filepath.Clean(trimmed)
}
