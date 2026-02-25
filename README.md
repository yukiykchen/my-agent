# Try Agent - 从零手搓 AI Agent 框架

一个模块化的 TypeScript AI Agent 框架，支持多模型、工具调用、子智能体和技能系统。

## 特性

-  **多模型支持** - Moonshot/Kimi、DeepSeek、智谱AI、Gemini、OpenAI 兼容接口
-  **工具系统** - 文件读写、Shell命令执行、目录操作等
-  **ReAct 循环** - 思考-行动-观察的智能决策循环
-  **子智能体** - 可扩展的 Sub-Agent 系统
-  **技能系统** - 可插拔的 Skill 模块
-  **多种人设** - 内置多种提示词模板
-  **对话压缩** - 自动压缩长对话历史

## 快速开始

### 安装依赖

```bash
npm install
```

### 配置环境变量

复制 `.env` 文件并填入你的 API Key：

```bash
# 至少配置一个模型的 API Key
MOONSHOT_API_KEY= xxx
```

### 运行

```bash
# 开发模式（热重载）
npm run dev

# 或者先编译再运行
npm run build
npm start
```

## 项目结构

```
src/
├── index.ts                # 入口文件（CLI 交互）
├── chat.ts                 # Agent 核心逻辑（ReAct 循环）
├── chat-compress-service.ts # 对话历史压缩
├── model-client.ts         # 模型客户端（多模型适配）
├── mcp-client.ts           # MCP 协议客户端
├── project-context.ts      # 项目上下文注入
├── system-prompt.ts        # 系统提示词管理
├── tool-registry.ts        # 工具注册中心
├── providers/              # 模型供应商实现
│   ├── types.ts            # 接口定义
│   ├── openai-compatible.ts # OpenAI 兼容适配器
│   ├── moonshot.ts         # Moonshot/Kimi
│   ├── deepseek.ts         # DeepSeek
│   ├── zhipu.ts            # 智谱 AI
│   └── gemini.ts           # Google Gemini
├── tools/                  # 工具实现
│   ├── read-file.ts        # 文件读取
│   ├── write-file.ts       # 文件写入
│   ├── edit-file.ts        # 文件编辑
│   ├── read-folder.ts      # 目录读取
│   └── run-shell-command.ts # Shell 命令
├── subagents/              # 子智能体
│   ├── sub-agent-types.ts
│   ├── sub-agent-registry.ts
│   └── codebase-investigator.ts
├── skills/                 # 技能系统
│   ├── skill-types.ts
│   ├── skill-registry.ts
│   └── skill-loader.ts
├── prompts/                # 提示词模板
│   ├── gemini-cli.md
│   ├── coding-mentor.md
│   ├── strict-engineer.md
│   └── ...
└── utils/                  # 工具函数
    └── frontmatter.ts
```

## 使用示例

### 作为 CLI 使用

```bash
npm run dev
```

启动后选择模型和人设，然后开始对话。

### 作为库使用

```typescript
import { Agent } from './chat.js';
import { ModelClient } from './model-client.js';

const client = new ModelClient({ provider: 'moonshot' });
const agent = new Agent(client, {
  promptTemplate: 'gemini-cli',
  verbose: true
});

await agent.initialize();
const response = await agent.chat('帮我创建一个 Hello World 程序');
console.log(response);
```

## 扩展

### 添加新工具

在 `src/tools/` 目录下创建新文件：

```typescript
import { toolRegistry } from '../tool-registry.js';

toolRegistry.register(
  'my_tool',
  '工具描述',
  {
    type: 'object',
    properties: {
      param1: { type: 'string', description: '参数描述' }
    },
    required: ['param1']
  },
  async (args) => {
    // 工具逻辑
    return JSON.stringify({ success: true });
  }
);
```

### 添加新模型供应商

实现 `ModelProvider` 接口或使用 `OpenAICompatibleProvider`。

## 命令

- `/help` - 显示帮助
- `/reset` - 重置对话
- `/prompts` - 列出提示词模板
- `/exit` - 退出

## License

MIT
