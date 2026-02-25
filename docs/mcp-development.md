# MCP 开发文档

## 什么是 MCP？

**MCP（Model Context Protocol，模型上下文协议）** 是一套标准化的协议，让 AI Agent 能以「即插即用」的方式对接外部工具和数据源——相当于 AI 世界的 USB 接口标准。

### 为什么需要 MCP？

没有 MCP 之前，每接入一个新工具，你都要：
1. 在代码里定义工具的 schema（名称、描述、参数）
2. 编写解析逻辑
3. 编写执行逻辑
4. 注册到工具系统

工具一多，维护成本爆炸。

有了 MCP，你只需要：
1. 在 `.mcp.json` 里加一行配置
2. 重启 Agent

Agent 会自动发现并注册新工具，零代码接入。

---

## 核心概念

### 架构

```
┌─────────────────────────────────────────────┐
│                   Agent                      │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐  │
│  │  Model   │  │   Tool   │  │    MCP     │  │
│  │  Client  │  │ Registry │  │   Client   │  │
│  └─────────┘  └──────────┘  └─────┬──────┘  │
│                                    │         │
└────────────────────────────────────┼─────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    │                │                 │
              ┌─────┴─────┐  ┌──────┴──────┐  ┌──────┴──────┐
              │ MCP Server│  │ MCP Server  │  │ MCP Server  │
              │ filesystem│  │   fetch     │  │  你的工具   │
              └───────────┘  └─────────────┘  └─────────────┘
```

### 通信方式

MCP 使用 **JSON-RPC 2.0 over stdio** 通信：
- Agent 通过子进程启动 MCP Server
- 通过 stdin 发送请求，通过 stdout 接收响应
- 协议简单，语言无关

### 生命周期

```
1. Agent 启动
2. 读取 .mcp.json 配置
3. 逐个启动 MCP Server 子进程
4. 发送 initialize 握手
5. 调用 tools/list 获取工具列表
6. 将工具注册到 ToolRegistry（添加 mcp_ 前缀）
7. Agent 开始工作，LLM 可以调用这些工具
8. Agent 退出时，断开所有 MCP Server
```

---

## 快速开始

### 1. 配置 MCP 服务器

编辑项目根目录的 `.mcp.json`：

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/dir"]
    },
    "fetch": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-fetch"]
    }
  }
}
```

每个服务器需要指定：
- `command` - 启动命令
- `args` - 命令参数（可选）
- `env` - 环境变量（可选）

### 2. 启动 Agent

```bash
npm run dev
```

启动时你会看到：

```
  ℹ  发现 2 个 MCP 服务器，正在连接...
  ✅ MCP 工具注册完成，共 11 个工具
     - [filesystem] read_file: Read the complete contents of a file...
     - [filesystem] write_file: Create a new file or overwrite...
     - [fetch] fetch: Fetches a URL from the internet...
```

### 3. 使用工具

在聊天中直接告诉 AI 你需要什么，它会自动选择合适的工具：

> "帮我看看项目根目录有哪些文件"

AI 会自动调用 `mcp_filesystem_list_directory` 工具。

---

## 已内置的 MCP 服务器

### @modelcontextprotocol/server-filesystem

文件系统操作工具集。

**工具列表：**
| 工具名 | 功能 |
|--------|------|
| `read_file` | 读取文件内容 |
| `read_multiple_files` | 批量读取文件 |
| `write_file` | 写入文件 |
| `edit_file` | 编辑文件（搜索替换） |
| `create_directory` | 创建目录 |
| `list_directory` | 列出目录内容 |
| `directory_tree` | 目录树 |
| `move_file` | 移动/重命名文件 |
| `search_files` | 搜索文件 |
| `get_file_info` | 获取文件信息 |
| `list_allowed_directories` | 列出允许访问的目录 |

**配置示例：**
```json
{
  "filesystem": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir1", "/path/to/dir2"]
  }
}
```

最后的参数是允许访问的目录列表（安全沙箱）。

### @modelcontextprotocol/server-fetch

HTTP 请求工具，让 AI 能够访问网页和 API。

**工具列表：**
| 工具名 | 功能 |
|--------|------|
| `fetch` | 请求 URL 并返回内容（自动将 HTML 转为 Markdown） |

**配置示例：**
```json
{
  "fetch": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-fetch"]
  }
}
```

---

## 开发自定义 MCP 服务器

如果社区没有你需要的工具，可以自己开发 MCP Server。

### 最小示例

创建 `my-mcp-server/index.js`：

```javascript
#!/usr/bin/env node

/**
 * 一个最简单的 MCP Server 示例
 * 提供一个 get_weather 工具
 */

const readline = require('readline');

// JSON-RPC 请求处理
const rl = readline.createInterface({ input: process.stdin });

let initialized = false;

rl.on('line', (line) => {
  try {
    const request = JSON.parse(line);
    handleRequest(request);
  } catch (e) {
    // 忽略非 JSON 输入
  }
});

function sendResponse(id, result) {
  const response = { jsonrpc: '2.0', id, result };
  process.stdout.write(JSON.stringify(response) + '\n');
}

function sendError(id, code, message) {
  const response = { jsonrpc: '2.0', id, error: { code, message } };
  process.stdout.write(JSON.stringify(response) + '\n');
}

function handleRequest(request) {
  const { id, method, params } = request;

  switch (method) {
    // ===== 握手 =====
    case 'initialize':
      sendResponse(id, {
        protocolVersion: '2024-11-05',
        capabilities: { tools: {} },
        serverInfo: { name: 'weather-server', version: '1.0.0' }
      });
      initialized = true;
      break;

    case 'notifications/initialized':
      // 客户端确认初始化完成（通知，不需要回复）
      break;

    // ===== 工具列表 =====
    case 'tools/list':
      sendResponse(id, {
        tools: [
          {
            name: 'get_weather',
            description: '获取指定城市的天气信息',
            inputSchema: {
              type: 'object',
              properties: {
                city: {
                  type: 'string',
                  description: '城市名称，如 "深圳"、"北京"'
                }
              },
              required: ['city']
            }
          }
        ]
      });
      break;

    // ===== 工具调用 =====
    case 'tools/call':
      handleToolCall(id, params);
      break;

    default:
      sendError(id, -32601, `Method not found: ${method}`);
  }
}

function handleToolCall(id, params) {
  const { name, arguments: args } = params;

  switch (name) {
    case 'get_weather':
      // 模拟天气数据（实际开发中替换为真实 API 调用）
      const weather = {
        city: args.city,
        temperature: Math.round(Math.random() * 30 + 5),
        condition: ['晴', '多云', '小雨', '阴'][Math.floor(Math.random() * 4)],
        humidity: Math.round(Math.random() * 60 + 30) + '%'
      };
      
      sendResponse(id, {
        content: [
          {
            type: 'text',
            text: `${weather.city}天气：${weather.condition}，温度 ${weather.temperature}°C，湿度 ${weather.humidity}`
          }
        ]
      });
      break;

    default:
      sendResponse(id, {
        content: [{ type: 'text', text: `Unknown tool: ${name}` }],
        isError: true
      });
  }
}
```

### 注册到 Agent

```json
{
  "mcpServers": {
    "weather": {
      "command": "node",
      "args": ["./my-mcp-server/index.js"]
    }
  }
}
```

重启 Agent，就能看到 `get_weather` 工具被自动注册！

---

## MCP 协议详解

### JSON-RPC 2.0 消息格式

**请求：**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_weather",
    "arguments": { "city": "深圳" }
  }
}
```

**成功响应：**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      { "type": "text", "text": "深圳天气：晴，28°C" }
    ]
  }
}
```

**错误响应：**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params: city is required"
  }
}
```

### 协议流程

```
Client (Agent)                    Server (MCP Tool)
    │                                    │
    │──── initialize ──────────────────>│
    │<─── {capabilities, serverInfo} ───│
    │                                    │
    │──── notifications/initialized ──>│
    │                                    │
    │──── tools/list ──────────────────>│
    │<─── {tools: [...]} ──────────────│
    │                                    │
    │──── tools/call ──────────────────>│
    │<─── {content: [...]} ────────────│
    │                                    │
```

### 关键方法

| 方法 | 方向 | 说明 |
|------|------|------|
| `initialize` | Client → Server | 握手，交换能力信息 |
| `notifications/initialized` | Client → Server | 确认初始化完成（通知，无响应） |
| `tools/list` | Client → Server | 获取工具列表 |
| `tools/call` | Client → Server | 调用工具 |

---

## 在本项目中的实现

### 文件结构

```
src/
├── mcp-client.ts          # MCP 协议客户端（JSON-RPC 2.0 over stdio）
├── tools/
│   └── mcp-bridge.ts      # MCP 工具桥接（将 MCP 工具注册到 ToolRegistry）
└── index.ts               # 启动时自动初始化 MCP
```

### 工具命名规则

MCP 工具注册到 ToolRegistry 时，会加上前缀：

```
mcp_{serverName}_{toolName}
```

例如：
- `mcp_filesystem_read_file` - filesystem 服务器的 read_file 工具
- `mcp_fetch_fetch` - fetch 服务器的 fetch 工具
- `mcp_weather_get_weather` - 自定义 weather 服务器的 get_weather 工具

这样可以避免与内置工具的命名冲突。

### API 接口

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/mcp/status` | GET | 获取 MCP 服务器状态和工具列表 |
| `/api/tools` | GET | 获取所有已注册工具（内置 + MCP） |
| `/api/mcp/logs` | GET | 获取工具调用日志 |

---

## 更多社区 MCP 服务器

MCP 生态正在快速增长，以下是一些常用的社区服务器：

| 服务器 | NPM 包 | 功能 |
|--------|--------|------|
| Filesystem | `@modelcontextprotocol/server-filesystem` | 文件系统操作 |
| Fetch | `@modelcontextprotocol/server-fetch` | HTTP 请求 |
| Memory | `@modelcontextprotocol/server-memory` | 知识图谱记忆 |
| Brave Search | `@modelcontextprotocol/server-brave-search` | Brave 搜索引擎 |
| GitHub | `@modelcontextprotocol/server-github` | GitHub API |
| PostgreSQL | `@modelcontextprotocol/server-postgres` | PostgreSQL 数据库 |
| SQLite | `@modelcontextprotocol/server-sqlite` | SQLite 数据库 |
| Puppeteer | `@modelcontextprotocol/server-puppeteer` | 浏览器自动化 |

可以在 [MCP Servers 仓库](https://github.com/modelcontextprotocol/servers) 查看完整列表。

---

## 常见问题

### Q: MCP Server 启动失败怎么办？

检查：
1. `command` 是否正确（`npx` / `node` / `python`）
2. NPM 包是否存在（运行 `npx -y @modelcontextprotocol/server-xxx` 测试）
3. 查看控制台的 stderr 输出

### Q: 工具调用超时？

默认超时 30 秒。如果工具执行时间较长（如大文件操作），可以在 `mcp-client.ts` 的 `sendRequest` 方法中调整超时时间。

### Q: 如何调试 MCP 通信？

MCP Client 会发出事件，可以监听：

```typescript
import { mcpClient } from './mcp-client.js';

mcpClient.on('server-log', ({ server, message }) => {
  console.log(`[${server}] ${message}`);
});

mcpClient.on('server-error', ({ server, error }) => {
  console.error(`[${server}] Error: ${error}`);
});
```

### Q: 能用 Python 写 MCP Server 吗？

可以！MCP 是语言无关的协议，任何能读写 stdio 的语言都行：

```json
{
  "my-python-tool": {
    "command": "python3",
    "args": ["./my-tool/server.py"]
  }
}
```
