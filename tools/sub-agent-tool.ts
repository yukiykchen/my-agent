/**
 * Sub-Agent 调用工具
 */

import { toolRegistry } from '../tool-registry.js';
import { subAgentRegistry } from '../subagents/sub-agent-registry.js';

export async function callSubAgent(args: Record<string, any>): Promise<string> {
  const { name, task, context } = args;
  
  try {
    const subAgent = subAgentRegistry.get(name);
    if (!subAgent) {
      return JSON.stringify({
        success: false,
        error: `未找到 Sub-Agent: ${name}`,
        available: subAgentRegistry.list().map(a => a.name)
      });
    }
    
    const result = await subAgent.execute(task, context);
    
    return JSON.stringify({
      success: true,
      agent: name,
      result
    });
  } catch (error: any) {
    return JSON.stringify({
      success: false,
      error: error.message
    });
  }
}

// 注册工具
toolRegistry.register(
  'call_sub_agent',
  '调用专门的 Sub-Agent 来处理特定任务',
  {
    type: 'object',
    properties: {
      name: {
        type: 'string',
        description: 'Sub-Agent 名称'
      },
      task: {
        type: 'string',
        description: '要执行的任务描述'
      },
      context: {
        type: 'string',
        description: '额外的上下文信息'
      }
    },
    required: ['name', 'task']
  },
  callSubAgent
);
