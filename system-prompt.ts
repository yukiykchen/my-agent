/**
 * 系统提示词管理
 */

import * as fs from 'fs/promises';
import * as path from 'path';
import matter from 'gray-matter';

// 提示词元数据
export interface PromptMetadata {
  name: string;
  description?: string;
  author?: string;
  version?: string;
  tags?: string[];
}

// 提示词配置
export interface PromptConfig {
  metadata: PromptMetadata;
  content: string;
}

/**
 * 系统提示词管理器
 */
export class SystemPromptManager {
  private promptsDir: string;
  private cache: Map<string, PromptConfig> = new Map();

  constructor(promptsDir?: string) {
    this.promptsDir = promptsDir || path.join(process.cwd(), 'src', 'prompts');
  }

  /**
   * 加载提示词文件
   */
  async load(name: string): Promise<PromptConfig> {
    // 检查缓存
    if (this.cache.has(name)) {
      return this.cache.get(name)!;
    }

    const filePath = path.join(this.promptsDir, `${name}.md`);
    const content = await fs.readFile(filePath, 'utf-8');
    
    // 解析 frontmatter
    const { data, content: body } = matter(content);
    
    const config: PromptConfig = {
      metadata: {
        name: data.name || name,
        description: data.description,
        author: data.author,
        version: data.version,
        tags: data.tags
      },
      content: body.trim()
    };

    this.cache.set(name, config);
    return config;
  }

  /**
   * 列出所有可用的提示词
   */
  async list(): Promise<PromptMetadata[]> {
    try {
      const files = await fs.readdir(this.promptsDir);
      const prompts: PromptMetadata[] = [];

      for (const file of files) {
        if (file.endsWith('.md')) {
          const name = file.replace('.md', '');
          try {
            const config = await this.load(name);
            prompts.push(config.metadata);
          } catch {
            prompts.push({ name });
          }
        }
      }

      return prompts;
    } catch {
      return [];
    }
  }

  /**
   * 获取提示词内容
   */
  async getContent(name: string): Promise<string> {
    const config = await this.load(name);
    return config.content;
  }

  /**
   * 清除缓存
   */
  clearCache(): void {
    this.cache.clear();
  }
}

// 默认系统提示词
export const DEFAULT_SYSTEM_PROMPT = `你是一个强大的AI编程助手。你的任务是帮助用户完成各种编程任务。

## 核心能力
- 理解和分析代码
- 编写高质量的代码
- 调试和修复问题
- 解释技术概念

## 工作原则
1. **准确性**：确保代码正确、可运行
2. **简洁性**：代码清晰易读，避免冗余
3. **安全性**：注意安全最佳实践
4. **效率性**：考虑性能和资源使用

## 交互方式
- 仔细阅读用户的问题
- 如果需要更多信息，主动询问
- 使用工具来完成实际操作
- 清晰地解释你的思考过程`;

export const promptManager = new SystemPromptManager();
