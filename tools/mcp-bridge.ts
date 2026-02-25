/**
 * MCP 工具桥接器
 * 将 MCP 服务器提供的工具自动注册到 ToolRegistry
 * 这样 Agent 就能像调用内置工具一样调用 MCP 工具
 */

import { mcpClient, MCPTool } from '../mcp-client.js';
import { toolRegistry } from '../tool-registry.js';
import type { ToolDefinition } from '../providers/types.js';

/** 工具调用日志（用于前端展示） */
export interface ToolCallLog {
  timestamp: number;
  server: string;
  tool: string;
  args: Record<string, any>;
  result: string;
  duration: number;
  success: boolean;
}

// 存储工具调用日志
const toolCallLogs: ToolCallLog[] = [];

/**
 * 将 MCP 工具的 inputSchema 转换为 OpenAI 格式的 ToolDefinition
 */
function mcpToolToDefinition(serverName: string, tool: MCPTool): ToolDefinition {
  return {
    type: 'function',
    function: {
      name: `mcp_${serverName}_${tool.name}`,
      description: `[MCP:${serverName}] ${tool.description}`,
      parameters: {
        type: 'object',
        properties: tool.inputSchema.properties || {},
        required: tool.inputSchema.required
      }
    }
  };
}

/**
 * 为一个 MCP 工具创建执行器
 * 执行器会通过 MCP 协议调用远程工具
 */
function createMCPExecutor(serverName: string, toolName: string) {
  return async (args: Record<string, any>): Promise<string> => {
    const startTime = Date.now();
    let success = true;
    let resultStr = '';

    try {
      const result = await mcpClient.callTool(serverName, toolName, args);
      
      // 将 MCP 结果转换为字符串
      if (result.content && result.content.length > 0) {
        resultStr = result.content
          .map(c => c.text || JSON.stringify(c))
          .join('\n');
      } else {
        resultStr = JSON.stringify(result);
      }

      if (result.isError) {
        success = false;
      }
    } catch (error: any) {
      success = false;
      resultStr = JSON.stringify({ error: error.message });
    }

    // 记录工具调用日志
    toolCallLogs.push({
      timestamp: startTime,
      server: serverName,
      tool: toolName,
      args,
      result: resultStr.slice(0, 1000), // 限制日志长度
      duration: Date.now() - startTime,
      success
    });

    // 只保留最近 100 条日志
    if (toolCallLogs.length > 100) {
      toolCallLogs.splice(0, toolCallLogs.length - 100);
    }

    return resultStr;
  };
}

/**
 * 连接所有 MCP 服务器，并将工具注册到 ToolRegistry
 * 返回注册的工具数量
 */
export async function registerMCPTools(): Promise<number> {
  let totalTools = 0;

  try {
    const results = await mcpClient.connectAll();

    for (const [serverName, tools] of results) {
      for (const tool of tools) {
        const registeredName = `mcp_${serverName}_${tool.name}`;
        
        // 注册到 ToolRegistry
        toolRegistry.register(
          registeredName,
          `[MCP:${serverName}] ${tool.description}`,
          {
            type: 'object',
            properties: tool.inputSchema.properties || {},
            required: tool.inputSchema.required
          },
          createMCPExecutor(serverName, tool.name)
        );

        totalTools++;
      }
    }
  } catch (error: any) {
    console.error('Failed to register MCP tools:', error.message);
  }

  return totalTools;
}

/**
 * 注销所有 MCP 工具
 */
export function unregisterMCPTools(): void {
  const toolNames = toolRegistry.getNames();
  for (const name of toolNames) {
    if (name.startsWith('mcp_')) {
      toolRegistry.unregister(name);
    }
  }
}

/**
 * 获取工具调用日志
 */
export function getToolCallLogs(limit = 50): ToolCallLog[] {
  return toolCallLogs.slice(-limit);
}

/**
 * 清除工具调用日志
 */
export function clearToolCallLogs(): void {
  toolCallLogs.length = 0;
}
