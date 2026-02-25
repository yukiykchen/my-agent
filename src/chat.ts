/**
 * Agent 核心逻辑 - ReAct 循环
 */

import chalk from 'chalk';
import ora from 'ora';
import type { Message, ToolCall } from './providers/types.js';
import { ModelClient } from './model-client.js';
import { toolRegistry } from './tool-registry.js';
import { ChatCompressService } from './chat-compress-service.js';
import { projectContext } from './project-context.js';
import { promptManager, DEFAULT_SYSTEM_PROMPT } from './system-prompt.js';

// 导入工具以触发注册
import './tools/index.js';
import './subagents/index.js';

// Agent 配置
export interface AgentConfig {
  maxIterations?: number;
  systemPrompt?: string;
  promptTemplate?: string;
  verbose?: boolean;
}

// Agent 状态
export interface AgentState {
  messages: Message[];
  iterations: number;
  isRunning: boolean;
}

/**
 * Agent 类
 */
export class Agent {
  private client: ModelClient;
  private compressService: ChatCompressService;
  private config: AgentConfig;
  private state: AgentState;

  constructor(client?: ModelClient, config?: AgentConfig) {
    this.client = client || new ModelClient();
    this.config = {
      maxIterations: config?.maxIterations || 20,
      verbose: config?.verbose ?? true,
      ...config
    };
    this.compressService = new ChatCompressService(this.client);
    this.state = {
      messages: [],
      iterations: 0,
      isRunning: false
    };
  }

  /**
   * 初始化 Agent
   */
  async initialize(): Promise<void> {
    // 加载系统提示词
    let systemPrompt = this.config.systemPrompt || DEFAULT_SYSTEM_PROMPT;
    
    if (this.config.promptTemplate) {
      try {
        systemPrompt = await promptManager.getContent(this.config.promptTemplate);
      } catch {
        // 使用默认提示词
      }
    }

    // 添加项目上下文
    const projectPrompt = await projectContext.generatePrompt();
    systemPrompt += '\n\n' + projectPrompt;

    // 添加可用工具信息
    const toolNames = toolRegistry.getNames();
    if (toolNames.length > 0) {
      systemPrompt += `\n\n## 可用工具\n${toolNames.join(', ')}`;
    }

    this.state.messages = [
      { role: 'system', content: systemPrompt }
    ];
  }

  /**
   * 处理用户输入
   */
  async chat(userMessage: string): Promise<string> {
    if (this.state.messages.length === 0) {
      await this.initialize();
    }

    // 添加用户消息
    this.state.messages.push({
      role: 'user',
      content: userMessage
    });

    // 检查是否需要压缩历史
    if (this.compressService.needsCompression(this.state.messages)) {
      this.state.messages = await this.compressService.compress(this.state.messages);
    }

    this.state.isRunning = true;
    this.state.iterations = 0;

    const tools = toolRegistry.getDefinitions();
    let finalResponse = '';

    // ReAct 循环
    while (this.state.isRunning && this.state.iterations < this.config.maxIterations!) {
      this.state.iterations++;

      const spinner = this.config.verbose 
        ? ora('思考中...').start() 
        : null;

      try {
        const response = await this.client.chat(
          this.state.messages,
          tools.length > 0 ? tools : undefined
        );

        spinner?.stop();

        // 处理工具调用
        if (response.tool_calls && response.tool_calls.length > 0) {
          await this.handleToolCalls(response.tool_calls, response.content);
        } else {
          // 没有工具调用，返回最终响应
          finalResponse = response.content || '';
          this.state.messages.push({
            role: 'assistant',
            content: finalResponse
          });
          this.state.isRunning = false;
        }
      } catch (error: any) {
        spinner?.stop();
        console.error(chalk.red('错误:'), error.message);
        finalResponse = `发生错误: ${error.message}`;
        this.state.isRunning = false;
      }
    }

    if (this.state.iterations >= this.config.maxIterations!) {
      finalResponse = '达到最大迭代次数，任务可能未完成。';
    }

    return finalResponse;
  }

  /**
   * 处理工具调用
   */
  private async handleToolCalls(toolCalls: ToolCall[], content: string | null): Promise<void> {
    // 添加助手消息
    this.state.messages.push({
      role: 'assistant',
      content: content || '',
      tool_calls: toolCalls
    });

    // 执行每个工具调用
    for (const toolCall of toolCalls) {
      const { name, arguments: argsStr } = toolCall.function;
      
      if (this.config.verbose) {
        console.log(chalk.cyan(`\n🔧 调用工具: ${name}`));
      }

      let result: string;
      try {
        const args = JSON.parse(argsStr);
        
        if (this.config.verbose) {
          console.log(chalk.gray(`   参数: ${JSON.stringify(args, null, 2)}`));
        }

        result = await toolRegistry.execute(name, args);
        
        if (this.config.verbose) {
          const preview = result.length > 200 
            ? result.slice(0, 200) + '...' 
            : result;
          console.log(chalk.green(`   结果: ${preview}`));
        }
      } catch (error: any) {
        result = JSON.stringify({ success: false, error: error.message });
        if (this.config.verbose) {
          console.log(chalk.red(`   错误: ${error.message}`));
        }
      }

      // 添加工具结果
      this.state.messages.push({
        role: 'tool',
        tool_call_id: toolCall.id,
        content: result
      });
    }
  }

  /**
   * 流式聊天
   */
  async *chatStream(userMessage: string): AsyncIterable<string> {
    if (this.state.messages.length === 0) {
      await this.initialize();
    }

    this.state.messages.push({
      role: 'user',
      content: userMessage
    });

    for await (const chunk of this.client.chatStream(this.state.messages)) {
      yield chunk;
    }
  }

  /**
   * 重置对话
   */
  reset(): void {
    this.state = {
      messages: [],
      iterations: 0,
      isRunning: false
    };
  }

  /**
   * 获取对话历史
   */
  getHistory(): Message[] {
    return [...this.state.messages];
  }

  /**
   * 获取状态
   */
  getState(): AgentState {
    return { ...this.state };
  }
}

// 导出单例
let defaultAgent: Agent | null = null;

export function getAgent(config?: AgentConfig): Agent {
  if (!defaultAgent || config) {
    defaultAgent = new Agent(undefined, config);
  }
  return defaultAgent;
}
