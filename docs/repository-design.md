# Repository 模块设计

`internal/repository` 是仓库级事实层，只负责发现、归一化、裁剪和返回结构化结果。

## 职责

- `Summary`：返回最小仓库摘要，如 `InGitRepo`、`Branch`、`Dirty`、`Ahead`、`Behind`
- `ChangedFiles`：围绕当前变更集返回受限的文件列表、状态和可选短片段
- `Retrieve`：提供 `path`、`glob`、`text`、`symbol` 四种统一的定向检索入口

## 非目标

- 不做 LSP 集成
- 不做向量检索或 embedding retrieval
- 不做预构建索引
- 不做跨文件语义分析平台
- 不直接决定 prompt 注入策略
- 不暴露为模型可调用工具

## 边界

```text
repository
  -> discover / summarize / retrieve

context
  -> decide whether to inject repository facts into prompt

runtime / tui / tools
  -> 本 issue 不直接接入 repository
```

## 结果约束

- `Summary` 与 `ChangedFiles` 统一基于一次 `git status --porcelain=v1 --branch --untracked-files=normal` 快照
- `ChangedFiles` 默认只返回路径和状态；默认上限 `50`，硬上限 `200`
- `ChangedFiles` 片段模式每文件最多 `20` 行，总计最多 `200` 行，并显式返回 `Truncated`
- `Retrieve` 默认上限 `20`，硬上限 `50`
- `Retrieve` 的 `text` / `symbol` 结果按 `path + line_hint` 稳定排序
- 路径解析必须限制在工作区内，并拒绝 path traversal 与 symlink escape

## 语言策略

- `symbol` 首版只对 Go 做轻量定义检索优化
- 其他语言先统一走 `path`、`glob`、`text`
