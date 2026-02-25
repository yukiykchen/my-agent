/**
 * Moonshot / Kimi Provider
 */

import { OpenAICompatibleProvider } from './openai-compatible.js';
import type { ModelProvider } from './types.js';

export function createMoonshotProvider(apiKey: string): ModelProvider {
  return new OpenAICompatibleProvider(
    'Moonshot',
    apiKey,
    'https://api.moonshot.cn/v1',
    'moonshot-v1-8k'
  );
}

// 可用模型
export const MOONSHOT_MODELS = [
  'moonshot-v1-8k',
  'moonshot-v1-32k',
  'moonshot-v1-128k'
];
