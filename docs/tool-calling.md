# Tool Calling 开发文档

## 什么是 Tool Calling？

Tool Calling（工具调用）是让 LLM 从「只能聊天」进化到「能做事」的关键能力。

### 普通 LLM

```
用户：深圳今天天气怎么样？
LLM：我是一个语言模型，无法获取实时天气信息...
```

### 有 Tool Calling 的 Agent

```
用户：深圳今天天气怎么样？
LLM：（决策）我应该调用 get_weather 工具
Agent：执行 get_weather({ city: "深圳" })
工具返回：晴，28°C
LLM：深圳今天天气晴朗，气温 28°C，适合出门！
```

**核心思路：LLM 负责"动脑"决策，代码负责"动手"执行。**

---

## 工作流程

```
                    ┌──────────────┐
                    │   用户输入    │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   LLM 思考   │◄─────────────────┐
                    └──────┬───────┘                   │
                           │                           │
                    ┌──────▼───────┐                   │
                    │  有工具调用？  │                   │
                    └──┬────────┬──┘                   │
                   Yes │        │ No                   │
                       │        │                      │
                ┌──────▼──┐  ┌──▼──────────┐          │
                │ 执行工具 │  │ 返回最终回答 │          │
                └──────┬──┘  └─────────────┘          │
                       │                               │
                ┌──────▼───────┐                       │
                │ 结果写回消息  │───────────────────────┘
                └──────────────┘
```

这就是 **ReAct 循环**（Reasoning + Acting）：
1. LLM **推理**（Reasoning）：分析问题，决定是否需要工具
2. Agent **执行**（Acting）：调用工具，获取结果
3. LLM **观察**（Observing）：看到工具结果，继续推理
4. 重复，直到 LLM 给出最终答案

---

## 在本项目中注册工具

### 方式一：内置工具（代码注册）

创建工具文件 `src/tools/my-tool.ts`：

```typescript
import { toolRegistry } from '../tool-registry.js';

/**
 * 工具执行函数
 * 接收参数，返回字符串结果
 */
async function myTool(args: Record<string, any>): Promise<string> {
  const { param1, param2 } = args;
  
  // 执行你的逻辑
  const result = `处理完成: ${param1}, ${param2}`;
  
  return result;
}

// 注册工具（模块加载时自动执行）
toolRegistry.register(
  'my_tool',                    // 工具名称
  '这个工具用来做什么',           // 描述（LLM 靠这个决定是否调用）
  {                              // 参数定义（JSON Schema 格式）
    type: 'object',
    properties: {
      param1: {
        type: 'string',
        description: '参数1的描述'
      },
      param2: {
        type: 'number', 
        description: '参数2的描述'
      }
    },
    required: ['param1']
  },
  myTool                        // 执行函数
);
```

然后在 `src/tools/index.ts` 中导入：

```typescript
import './my-tool.js';
```

### 方式二：MCP 工具（配置注册）

在 `.mcp.json` 中添加 MCP 服务器配置，Agent 启动时会自动发现和注册工具。详见 [MCP 开发文档](./mcp-development.md)。

---

## 工具定义规范

工具定义遵循 OpenAI 的 Function Calling 格式：

```typescript
interface ToolDefinition {
  type: 'function';
  function: {
    name: string;        // 工具名称（snake_case）
    description: string; // 工具描述（要写清楚什么时候该用）
    parameters: {        // 参数定义（JSON Schema）
      type: 'object';
      properties: Record<string, {
        type: string;        // 'string' | 'number' | 'boolean' | 'array' | 'object'
        description: string; // 参数描述
        enum?: string[];     // 可选：枚举值
      }>;
      required?: string[];   // 必填参数
    };
  };
}
```

### 描述的重要性

**描述是 LLM 决定是否调用工具的唯一依据！** 一个好的描述要包含：
- 工具做什么
- 什么时候应该用它
- 什么时候不应该用它

```typescript
// ❌ 差的描述
'读取文件'

// ✅ 好的描述
'读取指定路径的文件内容。返回文件的完整文本内容。支持文本文件，不支持二进制文件。路径可以是绝对路径或相对路径。'
```

---

## LLM 返回的工具调用格式

当 LLM 决定调用工具时，它会返回一个 `tool_calls` 数组：

```json
{
  "content": "让我帮你查看一下这个文件",
  "tool_calls": [
    {
      "id": "call_abc123",
      "type": "function",
      "function": {
        "name": "read_file",
        "arguments": "{\"path\": \"./package.json\"}"
      }
    }
  ],
  "finish_reason": "tool_calls"
}
```

注意：`arguments` 是 JSON **字符串**，需要 `JSON.parse()` 解析。

---

## 工具结果写回消息

工具执行完后，结果以 `tool` 角色消息的形式写回对话历史：

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "文件内容..."
}
```

然后 LLM 看到工具结果，继续推理，决定下一步是调用更多工具还是给出最终回答。

---

## 已注册的内置工具

| 工具名 | 文件 | 功能 |
|--------|------|------|
| `read_file` | `tools/read-file.ts` | 读取文件内容 |
| `write_file` | `tools/write-file.ts` | 写入文件 |
| `edit_file` | `tools/edit-file.ts` | 编辑文件（搜索替换） |
| `read_folder` | `tools/read-folder.ts` | 读取目录结构 |
| `run_shell_command` | `tools/run-shell-command.ts` | 执行 Shell 命令 |
| `call_sub_agent` | `tools/sub-agent-tool.ts` | 调用子 Agent |
| `call_skill` | `tools/skill-tool.ts` | 调用 Skill |

---

## 最佳实践

1. **工具粒度适中**：一个工具做一件事，不要太大也不要太小
2. **参数描述清晰**：LLM 靠描述理解参数含义
3. **返回结构化结果**：方便 LLM 解析和理解
4. **做好错误处理**：返回有意义的错误信息
5. **设置合理超时**：避免工具执行无限阻塞
6. **注意安全**：危险操作需要二次确认（如 `run_shell_command` 的命令黑名单）
