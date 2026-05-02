# Issue #492 验收对照矩阵（P4 HookPoint）

本文档用于关闭 [#492](https://github.com/1024XEngineer/neo-code/issues/492) 的工程验收证据。
基线：`upstream/main`（`ce36b706`，已合并 PR #531）。

## AC 对照矩阵

| AC / 任务项 | 实现位置 | 测试证据 | 结果 |
| --- | --- | --- | --- |
| 新增 HookPoint 枚举（P4-a/P4-b/P4-c） | `internal/runtime/hooks/types.go` | `internal/runtime/hooks/types_test.go` | ✅ |
| 能力矩阵（CanBlock/CanAnnotate/CanUpdateInput/UserAllowed） | `internal/runtime/hooks/types.go` | `internal/runtime/hooks/types_test.go` | ✅ |
| `CanBlock=false` 点位 block 降级为 observe | `internal/runtime/hooks/executor.go` | `internal/runtime/hooks/executor_test.go` | ✅ |
| `UserAllowed=false` 点位禁止 user/repo 挂载 | `internal/config/runtime_hooks.go`, `internal/runtime/repo_hooks.go` | `internal/config/runtime_hooks_test.go`, `internal/runtime/hook_capability_consistency_test.go`, `internal/runtime/repo_hooks_test.go` | ✅ |
| `before_permission_decision` 触发与阻断 | `internal/runtime/permission.go` | `internal/runtime/hooks_integration_test.go` | ✅ |
| `after_tool_failure` 触发 | `internal/runtime/toolexec.go` | `internal/runtime/hooks_integration_test.go` | ✅ |
| `session_start/session_end` 触发 | `internal/runtime/run.go` | `internal/runtime/hooks_integration_test.go` | ✅ |
| `user_prompt_submit` 触发与阻断 | `internal/runtime/run.go` | `internal/runtime/hooks_integration_test.go` | ✅ |
| `pre_compact/post_compact` 触发与阻断 | `internal/runtime/compact.go` | `internal/runtime/hooks_integration_test.go` | ✅ |
| `subagent_start/subagent_stop` 触发与阻断 | `internal/runtime/subagent_run.go` | `internal/runtime/hooks_integration_test.go` | ✅ |
| 无 hook 注册时主链行为不变 | `internal/runtime/*`（执行路径不依赖 hook） | `internal/runtime/hooks_integration_test.go`, `internal/runtime/hooks/executor_test.go` | ✅ |
| 性能退化约束证据（no hooks vs N hooks） | `internal/runtime/hooks/performance_test.go` | `TestExecutorRunOverheadWithinThreshold`, `BenchmarkExecutorRunNoHooks`, `BenchmarkExecutorRunTenHooks` | ✅ |

## 性能阈值说明

`TestExecutorRunOverheadWithinThreshold` 对 `HookPointBeforeToolCall` 做了 `no hooks` 与 `10 internal hooks` 的对比：

- 阈值：`withHookAvg <= noHookAvg * 30 + 300µs`
- 目的：防止新增点位后出现数量级退化，确保主链路可控

> 该阈值用于 CI 回归保护，不追求极限性能，后续可结合真实压测数据继续收紧。

## 建议执行命令

```powershell
go test ./internal/runtime/hooks/...
go test ./internal/runtime -run Hook -count=1
go test ./internal/config -run Hook -count=1
go test ./internal/runtime/...
```
