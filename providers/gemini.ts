/**
 * Google Gemini Provider
 * Gemini 使用不同的 API 格式，需要特殊处理
 */

import type {
  ModelProvider,
  Message,
  ToolDefinition,
  ModelConfig,
  ModelResponse,
  ToolCall
} from './types.js';

export class GeminiProvider implements ModelProvider {
  name = 'Gemini';
  private apiKey: string;
  private baseUrl = 'https://generativelanguage.googleapis.com/v1beta';

  constructor(apiKey: string) {
    this.apiKey = apiKey;
  }

  // 转换消息格式为 Gemini 格式
  private convertMessages(messages: Message[]): any[] {
    const contents: any[] = [];
    
    for (const msg of messages) {
      if (msg.role === 'system') {
        // Gemini 使用 systemInstruction，这里先跳过
        continue;
      }
      
      contents.push({
        role: msg.role === 'assistant' ? 'model' : 'user',
        parts: [{ text: msg.content }]
      });
    }
    
    return contents;
  }

  // 转换工具定义为 Gemini 格式
  private convertTools(tools: ToolDefinition[]): any[] {
    return [{
      functionDeclarations: tools.map(tool => ({
        name: tool.function.name,
        description: tool.function.description,
        parameters: tool.function.parameters
      }))
    }];
  }

  // 获取系统提示词
  private getSystemInstruction(messages: Message[]): string | undefined {
    const systemMsg = messages.find(m => m.role === 'system');
    return systemMsg?.content;
  }

  async chat(
    messages: Message[],
    tools?: ToolDefinition[],
    config?: ModelConfig
  ): Promise<ModelResponse> {
    const model = config?.model || 'gemini-1.5-flash';
    const url = `${this.baseUrl}/models/${model}:generateContent?key=${this.apiKey}`;

    const body: any = {
      contents: this.convertMessages(messages),
      generationConfig: {
        temperature: config?.temperature ?? 0.7,
        maxOutputTokens: config?.max_tokens ?? 4096
      }
    };

    const systemInstruction = this.getSystemInstruction(messages);
    if (systemInstruction) {
      body.systemInstruction = { parts: [{ text: systemInstruction }] };
    }

    if (tools && tools.length > 0) {
      body.tools = this.convertTools(tools);
    }

    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({})) as any;
      throw new Error(`Gemini API error: ${error.error?.message || response.statusText}`);
    }

    const data = await response.json() as any;
    const candidate = data.candidates?.[0];
    
    if (!candidate) {
      throw new Error('No response from Gemini');
    }

    const content = candidate.content;
    let textContent = '';
    const toolCalls: ToolCall[] = [];

    for (const part of content.parts || []) {
      if (part.text) {
        textContent += part.text;
      }
      if (part.functionCall) {
        toolCalls.push({
          id: `call_${Date.now()}_${Math.random().toString(36).slice(2)}`,
          type: 'function',
          function: {
            name: part.functionCall.name,
            arguments: JSON.stringify(part.functionCall.args)
          }
        });
      }
    }

    return {
      content: textContent || null,
      tool_calls: toolCalls.length > 0 ? toolCalls : undefined,
      finish_reason: toolCalls.length > 0 ? 'tool_calls' : 'stop',
      usage: data?.usageMetadata ? {
        prompt_tokens: data.usageMetadata.promptTokenCount ?? 0,
        completion_tokens: data.usageMetadata.candidatesTokenCount ?? 0,
        total_tokens: data.usageMetadata.totalTokenCount ?? 0
      } : undefined
    };
  }

  async listModels(): Promise<string[]> {
    return [
      'gemini-1.5-flash',
      'gemini-1.5-pro',
      'gemini-1.0-pro'
    ];
  }
}

export function createGeminiProvider(apiKey: string): ModelProvider {
  return new GeminiProvider(apiKey);
}

export const GEMINI_MODELS = [
  'gemini-1.5-flash',
  'gemini-1.5-pro',
  'gemini-1.0-pro'
];
