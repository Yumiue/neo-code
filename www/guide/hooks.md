---
title: Hooks 使用指南
description: 通过安全 builtin hooks 在 runtime 生命周期点配置用户/项目规则。
---

# Hooks 使用指南

Hook 是 NeoCode runtime 生命周期里的受控扩展点。你可以在固定 HookPoint 上配置规则，让运行过程附加注释、告警或守卫信号。

Hook 不是让配置文件执行任意脚本，而是在 NeoCode 已定义的生命周期点位上，执行受控的 builtin handler。

当前阶段是 P6-lite：

- 仅开放 builtin hooks。
- `command` / `http` / `prompt` / `agent` 等 external kinds 暂不开放。
- user/repo 配置主路径为 `sync`，`async` / `async_rewake` 主要用于 internal hooks。

## Hook 来源与顺序

Hook 有三类来源：

- `internal`：NeoCode 内部系统 hook，用于安全、验收、事件与 runtime 控制。
- `user`：用户全局 hook，配置在 `~/.neocode/config.yaml`，影响该用户所有工作区。
- `repo`：项目级 hook，配置在 `<workspace>/.neocode/hooks.yaml`，用于表达项目规则。

执行顺序固定为：

```text
internal -> user -> repo
```

边界说明：

- user/repo 不会高于 internal。
- repo hook 默认不执行，只有 workspace 被 trust 后才执行。
- user/repo 都受 HookPoint capability matrix 限制。
- 在 `before_completion_decision` 阶段，user/repo 只提供 annotation / guard signal，不直接决定 `accepted/failed/incomplete`；最终裁决由 runtime 内部 acceptance 决策链路完成。

## 配置路径

全局 user hooks：

```text
~/.neocode/config.yaml
```

配置位置：

```yaml
runtime:
  hooks:
    enabled: true
    user_hooks_enabled: true
    items:
      ...
```

项目 repo hooks：

```text
<workspace>/.neocode/hooks.yaml
```

文件结构：

```yaml
hooks:
  items:
    ...
```

## 支持的 Builtin Handlers

当前 P6-lite 仅支持这 3 个 handler：

- `add_context_note`
- `warn_on_tool_call`
- `require_file_exists`

### `add_context_note`

用途：给当前 run 注入规则说明（例如“优先最小改动”）。

常见挂点：`user_prompt_submit`

```yaml
- id: user-context-note
  enabled: true
  point: user_prompt_submit
  scope: user
  kind: builtin
  mode: sync
  handler: add_context_note
  params:
    note: "优先最小改动，避免无说明的大范围重构。"
```

补充：`params.message` 也可作为同义输入。

### `warn_on_tool_call`

用途：当模型调用指定工具时写入 warning/annotation（不改变主链行为）。

常见挂点：`before_tool_call`

```yaml
- id: user-warn-bash
  enabled: true
  point: before_tool_call
  scope: user
  kind: builtin
  mode: sync
  handler: warn_on_tool_call
  params:
    tool_names: ["bash"]
    message: "执行 bash 前请确认命令不会破坏工作区。"
```

补充：`params.tool_name`（单个）和 `params.tool_names`（多个）都支持。

### `require_file_exists`

用途：在完成前检查某个文件必须存在（如 `README.md`）。

常见挂点：`before_completion_decision`

```yaml
- id: require-readme-before-final
  enabled: true
  point: before_completion_decision
  scope: repo
  kind: builtin
  mode: sync
  handler: require_file_exists
  params:
    path: "README.md"
    message: "请先补齐 README.md。"
```

说明：`require_file_exists` 在 final 阶段提供 guard/annotation 信号，不直接输出最终 accepted/failed 结论。

## Trusted Workspace（repo hooks 安全门）

repo hooks 的安全模型是“先发现，后判定”：

- 默认只发现 `<workspace>/.neocode/hooks.yaml`，不直接执行。
- 仅当 workspace 在 trust store 中被标记为 trusted 时执行。
- trust store 路径是：

```text
~/.neocode/trusted-workspaces.json
```

- trust 文件缺失、空文件、损坏 JSON、结构不合法时，都按 untrusted 处理。
- 当前 trust 由本地 trust store 文件驱动，没有独立 trust 管理命令。

最小示例（路径必须是绝对路径）：

```json
{
  "version": 1,
  "workspaces": [
    "/absolute/path/to/workspace"
  ]
}
```

## Capability Matrix 与安全边界

不是所有 HookPoint 都允许 user/repo 挂载。当前 `UserAllowed=false` 的点位：

- `before_permission_decision`
- `pre_compact`
- `subagent_start`

可用于 user/repo 的常见点位包括：

- `user_prompt_submit`
- `before_tool_call`
- `after_tool_result`
- `after_tool_failure`
- `before_completion_decision`
- `session_start` / `session_end`
- `post_compact`
- `subagent_stop`

其他边界：

- external kinds 暂不开放。
- user/repo hook 上下文会做白名单裁剪，不暴露 API key、capability token、service 指针等敏感对象。
- user/repo `mode` 当前仅支持 `sync`。

## 当前不支持的 Hook Kind

P6-lite 不支持：

- `command`
- `http`
- `prompt`
- `agent`

配置这些 kind 时会被拒绝，例如：

```text
external hook kind "command" is not supported in P6-lite; only builtin hooks are enabled
```

暂缓原因：external hooks 涉及命令执行、网络请求、凭据泄露、prompt 注入和 agent 循环等风险，仍需后续 sandbox、allowlist、budget 与审计设计。

## 如何确认 Hook 已生效

可通过 runtime 事件日志确认：

- `hook_started`
- `hook_finished`
- `hook_failed`
- `hook_blocked`
- `repo_hooks_discovered`
- `repo_hooks_loaded`
- `repo_hooks_skipped_untrusted`
- `repo_hooks_trust_store_invalid`

关键字段包括：

- `source`（`user` / `repo` / `internal`）
- `point`
- `hook_id`
- `message`

示例：

```text
hook_finished source=user point=user_prompt_submit hook_id=user-context-note message="优先最小改动..."
```

在 TUI 中可通过日志视图查看（`Ctrl+L` 打开日志视图）。

## Demo A：全局用户规则

1. 在 `~/.neocode/config.yaml` 的 `runtime.hooks.items` 配置 `add_context_note`。
2. 让 Agent 执行一次常规改动任务。
3. 在日志中确认 `user_prompt_submit` 的 hook 事件。
4. 当前 run 会收到“优先最小改动”注释。

## Demo B：项目完成前检查

1. 在 `<workspace>/.neocode/hooks.yaml` 配置 `require_file_exists`（挂到 `before_completion_decision`）。
2. workspace 未 trust 时，repo hook 会被跳过，并出现 `repo_hooks_skipped_untrusted`。
3. 将 workspace 绝对路径写入 `~/.neocode/trusted-workspaces.json` 后再次运行。
4. 在 final 阶段可看到对应 hook 事件与 guard 信号，再由 runtime acceptance 链路统一裁决。
