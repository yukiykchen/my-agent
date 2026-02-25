/**
 * 模型客户端 - 统一的模型调用接口
 * 支持多模型供应商切换
 */

import { config } from 'dotenv';
import type {
  ModelProvider,
  Message,
  ToolDefinition,
  ModelConfig,
  ModelResponse
} from './providers/types.js';
import { createMoonshotProvider } from './providers/moonshot.js';
import { createDeepSeekProvider } from './providers/deepseek.js';
import { createZhipuProvider } from './providers/zhipu.js';
import { createGeminiProvider } from './providers/gemini.js';
import { OpenAICompatibleProvider } from './providers/openai-compatible.js';

config();

// 支持的供应商类型
export type ProviderType = 'moonshot' | 'deepseek' | 'zhipu' | 'gemini' | 'openai';

// 模型客户端配置
export interface ModelClientConfig {
  provider?: ProviderType;
  model?: string;
  temperature?: number;
  maxTokens?: number;
}

/**
 * 模型客户端类
 */
export class ModelClient {
  private provider: ModelProvider;
  private config: ModelConfig;

  constructor(clientConfig?: ModelClientConfig) {
    const providerType = clientConfig?.provider || 
      (process.env.DEFAULT_PROVIDER as ProviderType) || 
      'moonshot';
    
    this.provider = this.createProvider(providerType);
    this.config = {
      model: clientConfig?.model || this.getDefaultModel(providerType),
      temperature: clientConfig?.temperature ?? 0.7,
      max_tokens: clientConfig?.maxTokens ?? 4096
    };
  }

  /**
   * 根据类型创建 Provider
   */
  private createProvider(type: ProviderType): ModelProvider {
    switch (type) {
      case 'moonshot': {
        const apiKey = process.env.MOONSHOT_API_KEY;
        if (!apiKey) throw new Error('MOONSHOT_API_KEY not set');
        return createMoonshotProvider(apiKey);
      }
      case 'deepseek': {
        const apiKey = process.env.DEEPSEEK_API_KEY;
        if (!apiKey) throw new Error('DEEPSEEK_API_KEY not set');
        return createDeepSeekProvider(apiKey);
      }
      case 'zhipu': {
        const apiKey = process.env.ZHIPU_API_KEY;
        if (!apiKey) throw new Error('ZHIPU_API_KEY not set');
        return createZhipuProvider(apiKey);
      }
      case 'gemini': {
        const apiKey = process.env.GEMINI_API_KEY;
        if (!apiKey) throw new Error('GEMINI_API_KEY not set');
        return createGeminiProvider(apiKey);
      }
      case 'openai': {
        const apiKey = process.env.OPENAI_API_KEY;
        const baseUrl = process.env.OPENAI_BASE_URL || 'https://api.openai.com/v1';
        if (!apiKey) throw new Error('OPENAI_API_KEY not set');
        return new OpenAICompatibleProvider('OpenAI', apiKey, baseUrl, 'gpt-4o');
      }
      default:
        throw new Error(`Unknown provider: ${type}`);
    }
  }

  /**
   * 获取默认模型
   */
  private getDefaultModel(type: ProviderType): string {
    const defaults: Record<ProviderType, string> = {
      moonshot: 'moonshot-v1-8k',
      deepseek: 'deepseek-chat',
      zhipu: 'glm-4',
      gemini: 'gemini-1.5-flash',
      openai: 'gpt-4o'
    };
    return defaults[type];
  }

  /**
   * 发送聊天请求
   */
  async chat(
    messages: Message[],
    tools?: ToolDefinition[]
  ): Promise<ModelResponse> {
    return this.provider.chat(messages, tools, this.config);
  }

  /**
   * 流式聊天请求
   */
  async *chatStream(
    messages: Message[],
    tools?: ToolDefinition[]
  ): AsyncIterable<string> {
    if (this.provider.chatStream) {
      yield* this.provider.chatStream(messages, tools, this.config);
    } else {
      // 降级到普通请求
      const response = await this.chat(messages, tools);
      if (response.content) {
        yield response.content;
      }
    }
  }

  /**
   * 切换供应商
   */
  switchProvider(type: ProviderType): void {
    this.provider = this.createProvider(type);
    this.config.model = this.getDefaultModel(type);
  }

  /**
   * 设置模型
   */
  setModel(model: string): void {
    this.config.model = model;
  }

  /**
   * 获取当前供应商名称
   */
  getProviderName(): string {
    return this.provider.name;
  }
}

// 导出单例
let defaultClient: ModelClient | null = null;

export function getModelClient(config?: ModelClientConfig): ModelClient {
  if (!defaultClient || config) {
    defaultClient = new ModelClient(config);
  }
  return defaultClient;
}
