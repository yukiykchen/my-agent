/**
 * 智谱 AI Provider
 */

import { OpenAICompatibleProvider } from './openai-compatible.js';
import type { ModelProvider } from './types.js';

export function createZhipuProvider(apiKey: string): ModelProvider {
  return new OpenAICompatibleProvider(
    'Zhipu',
    apiKey,
    'https://open.bigmodel.cn/api/paas/v4',
    'glm-4'
  );
}

// 可用模型
export const ZHIPU_MODELS = [
  'glm-4',
  'glm-4-air',
  'glm-4-flash',
  'glm-3-turbo'
];
