/**
 * Sub-Agent 类型定义
 */

import type { Message, ToolDefinition } from '../providers/types.js';

// Sub-Agent 定义
export interface SubAgentDefinition {
  name: string;
  description: string;
  systemPrompt: string;
  tools?: string[];  // 可用的工具名称列表
  maxIterations?: number;
}

// Sub-Agent 接口
export interface SubAgent {
  name: string;
  description: string;
  
  /**
   * 执行任务
   */
  execute(task: string, context?: string): Promise<string>;
}

// Sub-Agent 执行结果
export interface SubAgentResult {
  success: boolean;
  output: string;
  iterations: number;
  toolCalls?: {
    tool: string;
    args: Record<string, any>;
    result: string;
  }[];
}
