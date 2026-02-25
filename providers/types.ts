/**
 * Provider 接口定义
 * 所有模型供应商需要实现这些接口
 */

// 消息角色
export type MessageRole = 'system' | 'user' | 'assistant' | 'tool';

// 消息结构
export interface Message {
  role: MessageRole;
  content: string;
  name?: string;
  tool_calls?: ToolCall[];
  tool_call_id?: string;
}

// 工具调用
export interface ToolCall {
  id: string;
  type: 'function';
  function: {
    name: string;
    arguments: string;
  };
}

// 工具定义
export interface ToolDefinition {
  type: 'function';
  function: {
    name: string;
    description: string;
    parameters: {
      type: 'object';
      properties: Record<string, {
        type: string;
        description: string;
        enum?: string[];
      }>;
      required?: string[];
    };
  };
}

// 模型响应
export interface ModelResponse {
  content: string | null;
  tool_calls?: ToolCall[];
  finish_reason: 'stop' | 'tool_calls' | 'length' | 'error';
  usage?: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
  };
}

// 模型配置
export interface ModelConfig {
  model: string;
  temperature?: number;
  max_tokens?: number;
  top_p?: number;
}

// Provider 接口
export interface ModelProvider {
  name: string;
  
  /**
   * 发送聊天请求
   */
  chat(
    messages: Message[],
    tools?: ToolDefinition[],
    config?: ModelConfig
  ): Promise<ModelResponse>;
  
  /**
   * 流式聊天请求
   */
  chatStream?(
    messages: Message[],
    tools?: ToolDefinition[],
    config?: ModelConfig
  ): AsyncIterable<string>;
  
  /**
   * 获取可用模型列表
   */
  listModels?(): Promise<string[]>;
}

// Provider 工厂函数类型
export type ProviderFactory = (apiKey: string, baseUrl?: string) => ModelProvider;
