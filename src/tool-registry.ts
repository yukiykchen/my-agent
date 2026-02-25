/**
 * 工具注册中心
 * 管理所有可用的工具
 */

import type { ToolDefinition } from './providers/types.js';

// 工具执行函数类型
export type ToolExecutor = (args: Record<string, any>) => Promise<string>;

// 工具注册信息
export interface RegisteredTool {
  definition: ToolDefinition;
  executor: ToolExecutor;
}

/**
 * 工具注册中心类
 */
export class ToolRegistry {
  private tools: Map<string, RegisteredTool> = new Map();

  /**
   * 注册工具
   */
  register(
    name: string,
    description: string,
    parameters: ToolDefinition['function']['parameters'],
    executor: ToolExecutor
  ): void {
    const definition: ToolDefinition = {
      type: 'function',
      function: {
        name,
        description,
        parameters
      }
    };

    this.tools.set(name, { definition, executor });
  }

  /**
   * 注销工具
   */
  unregister(name: string): boolean {
    return this.tools.delete(name);
  }

  /**
   * 获取工具
   */
  get(name: string): RegisteredTool | undefined {
    return this.tools.get(name);
  }

  /**
   * 执行工具
   */
  async execute(name: string, args: Record<string, any>): Promise<string> {
    const tool = this.tools.get(name);
    if (!tool) {
      throw new Error(`Tool not found: ${name}`);
    }
    return tool.executor(args);
  }

  /**
   * 获取所有工具定义
   */
  getDefinitions(): ToolDefinition[] {
    return Array.from(this.tools.values()).map(t => t.definition);
  }

  /**
   * 获取所有工具名称
   */
  getNames(): string[] {
    return Array.from(this.tools.keys());
  }

  /**
   * 检查工具是否存在
   */
  has(name: string): boolean {
    return this.tools.has(name);
  }

  /**
   * 获取工具数量
   */
  get size(): number {
    return this.tools.size;
  }
}

// 导出全局实例
export const toolRegistry = new ToolRegistry();
