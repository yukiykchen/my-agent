/**
 * Web 服务器入口
 */

import express from 'express';
import cors from 'cors';
import { config } from 'dotenv';
import { Agent } from './chat.js';
import { ModelClient, ProviderType } from './model-client.js';
import { mcpClient } from './mcp-client.js';
import { registerMCPTools, getToolCallLogs, clearToolCallLogs } from './tools/mcp-bridge.js';
import { toolRegistry } from './tool-registry.js';

config();

const app = express();
const PORT = process.env.PORT || 3000;

// 默认使用的模型供应商（预留接口，可以在这里切换）
const DEFAULT_PROVIDER: ProviderType = 'moonshot';

app.use(cors());
app.use(express.json());
app.use(express.static('public'));

// 存储每个会话的 Agent 实例
const agents = new Map<string, Agent>();

// ==================== MCP 启动 ====================

/** 启动 MCP 服务器并注册工具 */
async function initMCP(): Promise<void> {
  try {
    await mcpClient.loadConfig();
    
    if (!mcpClient.hasServers()) {
      console.log('  ℹ  未发现 MCP 服务器配置');
      return;
    }

    const serverStatus = mcpClient.getServerStatus();
    console.log(`  ℹ  发现 ${serverStatus.length} 个 MCP 服务器，正在连接...`);

    const toolCount = await registerMCPTools();
    console.log(`  ✅ MCP 工具注册完成，共 ${toolCount} 个工具`);
    
    // 打印已注册的工具
    const allTools = mcpClient.getAllTools();
    for (const { server, tool } of allTools) {
      console.log(`     - [${server}] ${tool.name}: ${tool.description.slice(0, 60)}`);
    }
  } catch (error: any) {
    console.error('  ⚠️  MCP 初始化失败:', error.message);
  }
}

// ==================== API 路由 ====================

/**
 * 获取可用的供应商列表
 */
app.get('/api/providers', (req, res) => {
  const providers = [
    { id: 'moonshot', name: 'Kimi', available: !!process.env.MOONSHOT_API_KEY },
  ];
  res.json(providers);
});

/**
 * 获取可用的提示词模板
 */
app.get('/api/prompts', async (req, res) => {
  try {
    const templates = [
      { id: 'gemini-cli', name: '🖥️ Gemini CLI 风格', description: '专业的编程助手' },
      { id: 'coding-mentor', name: '👨‍🏫 编程导师', description: '耐心的教学风格' },
      { id: 'strict-engineer', name: '👔 严格工程师', description: '注重代码质量' },
      { id: 'personal-assistant', name: '😊 个人助手', description: '友好的助手' },
      { id: 'sarcastic-friend', name: '😏 毒舌朋友', description: '幽默的风格' },
      { id: 'anime-girl', name: '🌸 二次元少女', description: '可爱的助手' }
    ];
    res.json(templates);
  } catch (error) {
    res.json([]);
  }
});

/**
 * 创建新会话
 */
app.post('/api/session', async (req, res) => {
  const { promptTemplate, provider } = req.body;
  const sessionId = `session_${Date.now()}_${Math.random().toString(36).slice(2)}`;

  const selectedProvider = (provider || DEFAULT_PROVIDER) as ProviderType;

  try {
    const client = new ModelClient({ provider: selectedProvider });
    const agent = new Agent(client, {
      promptTemplate: promptTemplate || 'gemini-cli',
      verbose: true
    });
    await agent.initialize();
    
    agents.set(sessionId, agent);
    
    res.json({ 
      success: true, 
      sessionId,
      provider: client.getProviderName(),
      toolCount: toolRegistry.size
    });
  } catch (error: any) {
    res.status(400).json({ 
      success: false, 
      error: error.message 
    });
  }
});

/**
 * 发送消息
 * 返回值中增加了工具调用信息
 */
app.post('/api/chat', async (req, res) => {
  const { sessionId, message } = req.body;
  
  const agent = agents.get(sessionId);
  if (!agent) {
    return res.status(404).json({ 
      success: false, 
      error: '会话不存在，请先创建会话' 
    });
  }

  // 记录调用前的日志数量
  const logsBefore = getToolCallLogs().length;

  try {
    const response = await agent.chat(message);
    
    // 获取本次对话中新增的工具调用日志
    const allLogs = getToolCallLogs();
    const newLogs = allLogs.slice(logsBefore);

    res.json({ 
      success: true, 
      response,
      toolCalls: newLogs
    });
  } catch (error: any) {
    res.status(500).json({ 
      success: false, 
      error: error.message 
    });
  }
});

/**
 * 重置会话
 */
app.post('/api/reset', async (req, res) => {
  const { sessionId } = req.body;
  
  const agent = agents.get(sessionId);
  if (agent) {
    agent.reset();
    await agent.initialize();
  }
  
  res.json({ success: true });
});

/**
 * 删除会话
 */
app.delete('/api/session/:sessionId', (req, res) => {
  const { sessionId } = req.params;
  agents.delete(sessionId);
  res.json({ success: true });
});

// ==================== MCP 管理 API ====================

/**
 * 获取 MCP 服务器状态
 */
app.get('/api/mcp/status', (req, res) => {
  const servers = mcpClient.getServerStatus();
  const allTools = mcpClient.getAllTools();
  
  res.json({
    servers,
    tools: allTools.map(({ server, tool }) => ({
      server,
      name: tool.name,
      description: tool.description,
      registeredAs: `mcp_${server}_${tool.name}`
    }))
  });
});

/**
 * 获取所有已注册的工具（包括内置和 MCP）
 */
app.get('/api/tools', (req, res) => {
  const tools = toolRegistry.getDefinitions().map(def => ({
    name: def.function.name,
    description: def.function.description,
    type: def.function.name.startsWith('mcp_') ? 'mcp' : 'builtin',
    parameters: def.function.parameters
  }));
  
  res.json(tools);
});

/**
 * 获取工具调用日志
 */
app.get('/api/mcp/logs', (req, res) => {
  const limit = parseInt(req.query.limit as string) || 50;
  res.json(getToolCallLogs(limit));
});

// ==================== 启动服务器 ====================

async function start() {
  // 初始化 MCP
  await initMCP();

  const toolNames = toolRegistry.getNames();

  app.listen(PORT, () => {
    console.log(`
╔═══════════════════════════════════════╗
║       🤖 AI Agent Framework 🤖        ║
║         Web Server Started            ║
╚═══════════════════════════════════════╝

服务器运行在: http://localhost:${PORT}
当前模型:    Kimi (Moonshot)
已注册工具:  ${toolNames.length} 个
  - 内置: ${toolNames.filter(n => !n.startsWith('mcp_')).length} 个
  - MCP:  ${toolNames.filter(n => n.startsWith('mcp_')).length} 个
    `);
  });
}

// 优雅退出
function cleanup() {
  console.log('\n正在断开 MCP 服务器...');
  mcpClient.disconnectAll();
  // 给子进程 1 秒时间退出，然后强制结束
  setTimeout(() => process.exit(0), 1000);
}

process.on('SIGINT', cleanup);
process.on('SIGTERM', cleanup);

start().catch(console.error);
