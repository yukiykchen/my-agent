/**
 * DeepSeek Provider
 */

import { OpenAICompatibleProvider } from './openai-compatible.js';
import type { ModelProvider } from './types.js';

export function createDeepSeekProvider(apiKey: string): ModelProvider {
  return new OpenAICompatibleProvider(
    'DeepSeek',
    apiKey,
    'https://api.deepseek.com/v1',
    'deepseek-chat'
  );
}

// 可用模型
export const DEEPSEEK_MODELS = [
  'deepseek-chat',
  'deepseek-coder'
];
