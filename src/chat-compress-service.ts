/**
 * 对话历史压缩服务
 * 当对话过长时，压缩历史消息以节省 token
 */

import type { Message } from './providers/types.js';
import { ModelClient } from './model-client.js';

// 压缩配置
export interface CompressConfig {
  maxTokens: number;          // 触发压缩的 token 阈值
  targetTokens: number;       // 压缩后的目标 token 数
  keepRecentMessages: number; // 保留最近的消息数量
}

const DEFAULT_CONFIG: CompressConfig = {
  maxTokens: 24000,
  targetTokens: 8000,
  keepRecentMessages: 4
};

/**
 * 对话压缩服务
 */
export class ChatCompressService {
  private config: CompressConfig;
  private client: ModelClient;

  constructor(client: ModelClient, config?: Partial<CompressConfig>) {
    this.client = client;
    this.config = { ...DEFAULT_CONFIG, ...config };
  }

  /**
   * 估算消息的 token 数量（简单估算）
   */
  private estimateTokens(messages: Message[]): number {
    let totalChars = 0;
    for (const msg of messages) {
      totalChars += msg.content.length;
      if (msg.tool_calls) {
        totalChars += JSON.stringify(msg.tool_calls).length;
      }
    }
    // 粗略估算：中文约 2 字符/token，英文约 4 字符/token
    return Math.ceil(totalChars / 3);
  }

  /**
   * 检查是否需要压缩
   */
  needsCompression(messages: Message[]): boolean {
    return this.estimateTokens(messages) > this.config.maxTokens;
  }

  /**
   * 压缩对话历史
   */
  async compress(messages: Message[]): Promise<Message[]> {
    if (!this.needsCompression(messages)) {
      return messages;
    }

    // 分离系统消息和对话消息
    const systemMessage = messages.find(m => m.role === 'system');
    const conversationMessages = messages.filter(m => m.role !== 'system');

    // 保留最近的消息
    const recentMessages = conversationMessages.slice(-this.config.keepRecentMessages);
    const oldMessages = conversationMessages.slice(0, -this.config.keepRecentMessages);

    if (oldMessages.length === 0) {
      return messages;
    }

    // 生成摘要
    const summary = await this.generateSummary(oldMessages);

    // 构建新的消息列表
    const result: Message[] = [];
    
    if (systemMessage) {
      result.push(systemMessage);
    }

    // 添加摘要作为系统消息的补充
    result.push({
      role: 'system',
      content: `[对话历史摘要]\n${summary}`
    });

    // 添加最近的消息
    result.push(...recentMessages);

    return result;
  }

  /**
   * 生成对话摘要
   */
  private async generateSummary(messages: Message[]): Promise<string> {
    const conversationText = messages
      .map(m => `${m.role}: ${m.content}`)
      .join('\n\n');

    const response = await this.client.chat([
      {
        role: 'system',
        content: '你是一个对话摘要助手。请将以下对话内容压缩成简洁的摘要，保留关键信息和上下文。摘要应该包含：主要讨论的话题、做出的决定、完成的任务、待解决的问题。'
      },
      {
        role: 'user',
        content: `请总结以下对话：\n\n${conversationText}`
      }
    ]);

    return response.content || '无法生成摘要';
  }
}
