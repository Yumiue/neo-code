//go:build !windows

package config

import (
	"errors"
	"strings"
)

// PersistUserEnvVar persists a key/value pair into user-level environment storage.
// 非 Windows 平台当前不做系统级持久化，但会统一执行输入校验。
func PersistUserEnvVar(key string, value string) error {
	if err := ValidateEnvVarName(strings.TrimSpace(key)); err != nil {
		return err
	}
	if strings.ContainsAny(value, "\r\n") {
		return errors.New("config: env value contains newline")
	}
	return nil
}

// DeleteUserEnvVar 删除用户级环境变量；非 Windows 平台当前无需额外处理。
func DeleteUserEnvVar(key string) error {
	if err := ValidateEnvVarName(strings.TrimSpace(key)); err != nil {
		return err
	}
	return nil
}

// LookupUserEnvVar 查询用户级环境变量；非 Windows 平台当前无单独存储，统一返回不存在。
func LookupUserEnvVar(key string) (string, bool, error) {
	if err := ValidateEnvVarName(strings.TrimSpace(key)); err != nil {
		return "", false, err
	}
	return "", false, nil
}

// SupportsUserEnvPersistence 返回当前平台是否支持用户级环境变量持久化。
func SupportsUserEnvPersistence() bool {
	return false
}
