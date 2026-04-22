package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ContextBudgetMigrationResult 汇总 config.yaml 预算配置迁移的执行结果。
type ContextBudgetMigrationResult struct {
	Path    string
	Changed bool
	Backup  string
	Reason  string
}

// DefaultConfigPath 返回当前用户环境下的默认主配置文件路径。
func DefaultConfigPath() string {
	return filepath.Join(defaultBaseDir(), configName)
}

// UpgradeConfigSchemaBeforeLoad 在严格解析配置前执行一次磁盘结构升级。
func UpgradeConfigSchemaBeforeLoad(path string) error {
	_, err := MigrateContextBudgetConfigFile(path, false)
	return err
}

// MigrateContextBudgetConfigFile 将 config.yaml 中的 context.auto_compact 迁移到 context.budget。
func MigrateContextBudgetConfigFile(path string, dryRun bool) (ContextBudgetMigrationResult, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	if filepath.Base(path) != configName {
		return ContextBudgetMigrationResult{}, fmt.Errorf("config: migration target must be %s", configName)
	}

	result := ContextBudgetMigrationResult{Path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("config: read migration target %s: %w", path, err)
	}

	migrated, changed, err := MigrateContextBudgetConfigContent(raw)
	if err != nil {
		return result, fmt.Errorf("config: migrate %s: %w", path, err)
	}
	if !changed {
		result.Reason = "未检测到 context.auto_compact"
		return result, nil
	}

	result.Changed = true
	if dryRun {
		return result, nil
	}

	backup := path + ".bak"
	if err := os.WriteFile(backup, raw, 0o644); err != nil {
		return result, fmt.Errorf("config: write migration backup %s: %w", backup, err)
	}
	if err := os.WriteFile(path, migrated, 0o644); err != nil {
		return result, fmt.Errorf("config: write migrated config %s: %w", path, err)
	}
	result.Backup = backup
	return result, nil
}

// MigrateContextBudgetConfigContent 将旧预算 YAML 块替换为当前预算 YAML 块。
func MigrateContextBudgetConfigContent(raw []byte) ([]byte, bool, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return raw, false, nil
	}
	if !bytes.Contains(raw, []byte("auto_compact")) {
		return raw, false, nil
	}

	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, false, err
	}
	contextValue, ok := doc["context"]
	if !ok {
		return raw, false, nil
	}
	contextMap, ok := migrationStringMap(contextValue)
	if !ok {
		return nil, false, errors.New("context must be a mapping")
	}

	autoValue, hasAutoCompact := contextMap["auto_compact"]
	if !hasAutoCompact {
		return raw, false, nil
	}
	if _, hasBudget := contextMap["budget"]; hasBudget {
		return nil, false, errors.New("context.auto_compact and context.budget cannot both exist")
	}

	autoMap, ok := migrationStringMap(autoValue)
	if !ok {
		return nil, false, errors.New("context.auto_compact must be a mapping")
	}
	budgetMap := make(map[string]any)
	migrationMoveField(autoMap, budgetMap, "input_token_threshold", "prompt_budget")
	migrationMoveField(autoMap, budgetMap, "reserve_tokens", "reserve_tokens")
	migrationMoveField(autoMap, budgetMap, "fallback_input_token_threshold", "fallback_prompt_budget")

	delete(contextMap, "auto_compact")
	contextMap["budget"] = budgetMap
	doc["context"] = contextMap

	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// migrationMoveField 在两个 YAML map 之间迁移字段名，不修改字段值。
func migrationMoveField(src map[string]any, dst map[string]any, oldName string, newName string) {
	if value, ok := src[oldName]; ok {
		dst[newName] = value
	}
}

// migrationStringMap 将 YAML map 统一转为 map[string]any。
func migrationStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		result := make(map[string]any, len(typed))
		for key, value := range typed {
			keyString, ok := key.(string)
			if !ok {
				return nil, false
			}
			result[keyString] = value
		}
		return result, true
	default:
		return nil, false
	}
}
