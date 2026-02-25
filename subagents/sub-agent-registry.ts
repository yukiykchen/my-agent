/**
 * Sub-Agent 注册中心
 */

import type { SubAgent, SubAgentDefinition } from './sub-agent-types.js';
import { ModelClient } from '../model-client.js';
import { toolRegistry } from '../tool-registry.js';

/**
 * Sub-Agent 基类
 */
export class BaseSubAgent implements SubAgent {
  name: string;
  description: string;
  protected systemPrompt: string;
  protected tools: string[];
  protected maxIterations: number;
  protected client: ModelClient;

  constructor(definition: SubAgentDefinition, client?: ModelClient) {
    this.name = definition.name;
    this.description = definition.description;
    this.systemPrompt = definition.systemPrompt;
    this.tools = definition.tools || [];
    this.maxIterations = definition.maxIterations || 10;
    this.client = client || new ModelClient();
  }

  async execute(task: string, context?: string): Promise<string> {
    const messages: any[] = [
      { role: 'system', content: this.systemPrompt },
      { role: 'user', content: context ? `${task}\n\n上下文：${context}` : task }
    ];

    // 获取可用工具
    const availableTools = this.tools.length > 0
      ? toolRegistry.getDefinitions().filter(t => this.tools.includes(t.function.name))
      : [];

    let iterations = 0;

    while (iterations < this.maxIterations) {
      iterations++;

      const response = await this.client.chat(
        messages,
        availableTools.length > 0 ? availableTools : undefined
      );

      // 处理工具调用
      if (response.tool_calls && response.tool_calls.length > 0) {
        messages.push({
          role: 'assistant',
          content: response.content,
          tool_calls: response.tool_calls
        });

        for (const toolCall of response.tool_calls) {
          const args = JSON.parse(toolCall.function.arguments);
          const result = await toolRegistry.execute(toolCall.function.name, args);
          
          messages.push({
            role: 'tool',
            tool_call_id: toolCall.id,
            content: result
          });
        }
      } else {
        // 没有工具调用，返回结果
        return response.content || '任务完成';
      }
    }

    return '达到最大迭代次数';
  }
}

/**
 * Sub-Agent 注册中心
 */
class SubAgentRegistry {
  private agents: Map<string, SubAgent> = new Map();

  /**
   * 注册 Sub-Agent
   */
  register(agent: SubAgent): void {
    this.agents.set(agent.name, agent);
  }

  /**
   * 从定义创建并注册
   */
  registerFromDefinition(definition: SubAgentDefinition): void {
    const agent = new BaseSubAgent(definition);
    this.register(agent);
  }

  /**
   * 获取 Sub-Agent
   */
  get(name: string): SubAgent | undefined {
    return this.agents.get(name);
  }

  /**
   * 列出所有 Sub-Agent
   */
  list(): { name: string; description: string }[] {
    return Array.from(this.agents.values()).map(a => ({
      name: a.name,
      description: a.description
    }));
  }

  /**
   * 检查是否存在
   */
  has(name: string): boolean {
    return this.agents.has(name);
  }
}

export const subAgentRegistry = new SubAgentRegistry();
