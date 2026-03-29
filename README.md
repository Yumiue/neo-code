# NeoCode

NeoCode 是一个基于 Go 和 Bubble Tea 的本地编码 Agent。它在终端中运行 ReAct 闭环，能够对话、调用工具、持久化会话，并以流式方式展示模型输出。

## 当前 Provider 策略

- 内建 provider 定义随代码版本发布。
- `config.yaml` 不再持久化完整 `providers` 列表。
- `config.yaml` 只保存当前选择状态和通用运行配置。
- 运行时的 `providers` 完全来自代码内建定义。
- API Key 只从环境变量读取，不写入 YAML。
- 当前内建 provider 包括 `openai` 和 `gemini`。
- `gemini` 复用 OpenAI-compatible driver，请求地址指向 Gemini 的兼容接口。
- provider 实例自己定义 `base_url`、默认模型、可选模型列表和 `api_key_env`。
- `base_url` 不在 TUI 中展示给用户。
- driver 只负责协议构造与响应解析，不决定 `models`、`base_url` 或 `api_key_env`。

这意味着：

- 新用户启动后会自动拿到当前版本最新的内建 provider。
- 未来代码新增 provider 时，新用户不需要修改 YAML。
- 老配置文件中的 `providers` / `provider_overrides` 会在加载时被清理为新的最小状态格式。

## 配置文件

默认路径：
[`~/.neocode/config.yaml`](~/.neocode/config.yaml)

当前落盘结构示例：

```yaml
selected_provider: openai
current_model: gpt-5.4
workdir: .
shell: powershell
max_loops: 8
tool_timeout_sec: 20
```

其中：

- `selected_provider` 和 `current_model` 是用户当前选择。
- provider 的 `base_url`、`models`、`api_key_env` 和 `driver` 都由开发者在代码中预设。
- `openai` 默认读取 `OPENAI_API_KEY`，`gemini` 默认读取 `GEMINI_API_KEY`。
- 完整 provider 列表不落盘，用户不需要在 YAML 中维护供应商元数据。

## Slash Commands

- `/provider`：打开 provider 选择器。
- `/model`：打开当前 provider 的模型选择器。



## 安装指南 (快速开始)

NeoCode 提供了极其原生的系统级安装体验，真正的开箱即用（无需 Go 环境）。请根据你的操作系统选择最适合的安装方式：

### 🍎 macOS / Linux (推荐 Homebrew)

对于习惯使用 Homebrew 的用户，只需一行命令即可接入官方 Tap 并安装：

```bash
brew install 1024XEngineer/homebrew-neocode/neocode
```

### 🪟 Windows (推荐 Scoop)

对于 Windows 极客玩家，我们提供了官方的 Scoop 分发源：

```PowerShell
scoop bucket add neocode https://github.com/1024XEngineer/scoop-bucket.git
scoop install neocode
```

### 🐧 Ubuntu / Debian (.deb)

前往项目的 [Releases 页面](https://www.google.com/search?q=https://github.com/pionxe/neo-code/releases) 下载最新版本的 `.deb` 安装包，然后在终端执行：

```Bash
sudo dpkg -i neo-code_*_linux_amd64.deb
```

### 🚀 兜底方案：一键安装脚本

如果你的系统没有安装上述包管理器，也可以使用我们提供的自动化脚本一键下载并配置：

**Linux / macOS:**

```Bash
curl -sSL https://raw.githubusercontent.com/1024XEngineer/neo-code/main/scripts/install.sh | bash
```

**Windows (PowerShell):**

```PowerShell
irm https://raw.githubusercontent.com/1024XEngineer/neo-code/main/scripts/install.ps1 | iex
```



## 运行

```bash
go run ./cmd/neocode
```

## 开发

```bash
gofmt -w ./cmd ./internal
go test ./...
```
