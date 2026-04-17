package config

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

var envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var protectedEnvVarNames = map[string]struct{}{
	"PATH":              {},
	"HOME":              {},
	"USERPROFILE":       {},
	"SHELL":             {},
	"PWD":               {},
	"COMSPEC":           {},
	"PATHEXT":           {},
	"WINDIR":            {},
	"SYSTEMROOT":        {},
	"TEMP":              {},
	"TMP":               {},
	"LD_LIBRARY_PATH":   {},
	"DYLD_LIBRARY_PATH": {},
	"XDG_CONFIG_HOME":   {},
}

// ValidateEnvVarName 校验环境变量名格式，避免无效名称进入配置与持久化链路。
func ValidateEnvVarName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("config: env key is empty")
	}
	if !envVarNamePattern.MatchString(trimmed) {
		return fmt.Errorf("config: env key %q is invalid", trimmed)
	}
	return nil
}

// NormalizeEnvVarNameForCompare 统一环境变量名比较键，Windows 上按大小写不敏感处理。
func NormalizeEnvVarNameForCompare(name string) string {
	trimmed := strings.TrimSpace(name)
	if runtime.GOOS == "windows" {
		return strings.ToUpper(trimmed)
	}
	return trimmed
}

// IsProtectedEnvVarName 判断名称是否命中受保护变量，避免覆盖关键系统变量。
func IsProtectedEnvVarName(name string) bool {
	key := strings.ToUpper(strings.TrimSpace(name))
	_, found := protectedEnvVarNames[key]
	return found
}
