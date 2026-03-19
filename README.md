# neo-code

一个基于 Go 的命令行对话 Demo，当前使用 `config.yaml` 作为唯一业务配置文件。

现在支持：

1. REPL 对话与流式输出
2. `/switch` 切换聊天模型
3. 本地 JSON 长期记忆检索与写回
4. 短期上下文保留
5. 人设文件注入

## 配置方式

只需要修改根目录下的 `config.yaml`。

示例：

```yaml
app:
  name: "NeoCode"
  version: "1.0.0"

ai:
  provider: "modelscope"
  api_key: "你的聊天模型 Key"
  model: "Qwen/Qwen3-Coder-480B-A35B-Instruct"

embedding:
  provider: "modelscope"
  api_key: "你的向量模型 Key"
  model: "BAAI/bge-large-zh-v1.5"

memory:
  file_path: "./data/memory.json"
  top_k: 5
  min_score: 0.75
  max_items: 1000

history:
  short_term_turns: 6

persona:
  file_path: "./persona.txt"
```

说明：

- `ai.api_key`：聊天模型调用所需 Key
- `embedding.api_key`：记忆检索和记忆保存所需向量 Key
- `memory.file_path`：长期记忆存储文件
- `history.short_term_turns`：保留最近多少轮上下文
- `persona.file_path`：启动时加载的人设文件

## 运行

```bash
go run .
```

## 可用命令

- `/models`：查看支持的模型
- `/switch <model>`：切换当前聊天模型
- `/memory`：查看本地记忆状态
- `/clear-memory`：清空长期记忆
- `/clear-context`：清空当前短期上下文
- `/help`：查看帮助
- `/exit`：退出程序

## 相关文件

- `config.yaml`：主配置文件
- `config/models.yaml`：模型与接口地址映射
- `data/memory.json`：长期记忆存储文件
- `persona.txt`：人设内容

如果没有配置 `embedding.api_key`，普通对话可能还能发起，但记忆检索和记忆保存会不可用。

