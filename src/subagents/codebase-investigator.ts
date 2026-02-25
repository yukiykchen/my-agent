/**
 * 代码库调查员 Sub-Agent
 * 专门用于分析和理解代码库结构
 */

import { BaseSubAgent } from './sub-agent-registry.js';
import type { SubAgentDefinition } from './sub-agent-types.js';
import { subAgentRegistry } from './sub-agent-registry.js';

const CODEBASE_INVESTIGATOR_DEFINITION: SubAgentDefinition = {
  name: 'codebase_investigator',
  description: '分析代码库结构，查找特定功能的实现位置，理解代码架构',
  systemPrompt: `你是一个代码库调查员，专门负责分析和理解代码库。

你的职责：
1. 分析项目结构和架构
2. 查找特定功能或模块的实现位置
3. 理解代码之间的依赖关系
4. 识别设计模式和最佳实践
5. 发现潜在的问题和改进点

工作方式：
- 首先通过 read_folder 了解项目结构
- 使用 read_file 阅读关键文件
- 使用 run_shell_command 执行 grep/find 等命令搜索代码
- 系统性地分析，不要遗漏重要细节

输出格式：
- 清晰地描述你的发现
- 提供具体的文件路径和代码位置
- 给出结构化的分析报告`,
  tools: ['read_file', 'read_folder', 'run_shell_command'],
  maxIterations: 15
};

export class CodebaseInvestigator extends BaseSubAgent {
  constructor() {
    super(CODEBASE_INVESTIGATOR_DEFINITION);
  }
}

// 注册到全局
subAgentRegistry.register(new CodebaseInvestigator());

export { CODEBASE_INVESTIGATOR_DEFINITION };
