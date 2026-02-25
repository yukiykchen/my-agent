/**
 * OpenAI 兼容接口适配器
 * 支持所有兼容 OpenAI API 格式的模型供应商
 */

import type {
  ModelProvider,
  Message,
  ToolDefinition,
  ModelConfig,
  ModelResponse
} from './types.js';

export class OpenAICompatibleProvider implements ModelProvider {
  name: string;
  private apiKey: string;
  private baseUrl: string;
  private defaultModel: string;

  constructor(
    name: string,
    apiKey: string,
    baseUrl: string,
    defaultModel: string
  ) {
    this.name = name;
    this.apiKey = apiKey;
    this.baseUrl = baseUrl.replace(/\/$/, ''); // 移除末尾斜杠
    this.defaultModel = defaultModel;
  }

  async chat(
    messages: Message[],
    tools?: ToolDefinition[],
    config?: ModelConfig
  ): Promise<ModelResponse> {
    const response = await fetch(`${this.baseUrl}/chat/completions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${this.apiKey}`
      },
      body: JSON.stringify({
        model: config?.model || this.defaultModel,
        messages,
        tools: tools && tools.length > 0 ? tools : undefined,
        temperature: config?.temperature ?? 0.7,
        max_tokens: config?.max_tokens ?? 4096,
        top_p: config?.top_p
      })
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({})) as any;
      throw new Error(
        `${this.name} API error: ${error.error?.message || response.statusText}`
      );
    }

    const data = await response.json() as any;
    const choice = data.choices[0];

    return {
      content: choice.message.content,
      tool_calls: choice.message.tool_calls,
      finish_reason: choice.finish_reason === 'tool_calls' ? 'tool_calls' : 'stop',
      usage: data.usage
    };
  }

  async *chatStream(
    messages: Message[],
    tools?: ToolDefinition[],
    config?: ModelConfig
  ): AsyncIterable<string> {
    const response = await fetch(`${this.baseUrl}/chat/completions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${this.apiKey}`
      },
      body: JSON.stringify({
        model: config?.model || this.defaultModel,
        messages,
        tools: tools && tools.length > 0 ? tools : undefined,
        temperature: config?.temperature ?? 0.7,
        max_tokens: config?.max_tokens ?? 4096,
        stream: true
      })
    });

    if (!response.ok) {
      throw new Error(`${this.name} API error: ${response.statusText}`);
    }

    const reader = response.body?.getReader();
    if (!reader) throw new Error('No response body');

    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = line.slice(6);
          if (data === '[DONE]') return;

          try {
            const parsed = JSON.parse(data);
            const content = parsed.choices[0]?.delta?.content;
            if (content) yield content;
          } catch {
            // 忽略解析错误
          }
        }
      }
    }
  }

  async listModels(): Promise<string[]> {
    const response = await fetch(`${this.baseUrl}/models`, {
      headers: {
        'Authorization': `Bearer ${this.apiKey}`
      }
    });

    if (!response.ok) {
      throw new Error(`Failed to list models: ${response.statusText}`);
    }

    const data = await response.json() as any;
    return data.data.map((model: { id: string }) => model.id);
  }
}
