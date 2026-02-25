/**
 * 项目上下文注入
 * 自动收集项目信息并注入到提示词中
 */

import * as fs from 'fs/promises';
import * as path from 'path';

// 项目上下文信息
export interface ProjectContext {
  name: string;
  path: string;
  type?: string;
  language?: string;
  framework?: string;
  description?: string;
  structure?: string;
  dependencies?: Record<string, string>;
}

/**
 * 项目上下文管理器
 */
export class ProjectContextManager {
  private projectPath: string;
  private context: ProjectContext | null = null;

  constructor(projectPath?: string) {
    this.projectPath = projectPath || process.cwd();
  }

  /**
   * 分析项目
   */
  async analyze(): Promise<ProjectContext> {
    if (this.context) {
      return this.context;
    }

    const context: ProjectContext = {
      name: path.basename(this.projectPath),
      path: this.projectPath
    };

    // 检测项目类型
    const files = await this.safeReadDir(this.projectPath);
    
    // 检查 package.json (Node.js 项目)
    if (files.includes('package.json')) {
      const pkg = await this.readJson('package.json');
      context.type = 'node';
      context.language = 'typescript/javascript';
      context.name = pkg.name || context.name;
      context.description = pkg.description;
      context.dependencies = { ...pkg.dependencies, ...pkg.devDependencies };
      
      // 检测框架
      if (context.dependencies) {
        if (context.dependencies['react']) context.framework = 'React';
        else if (context.dependencies['vue']) context.framework = 'Vue';
        else if (context.dependencies['@angular/core']) context.framework = 'Angular';
        else if (context.dependencies['next']) context.framework = 'Next.js';
        else if (context.dependencies['express']) context.framework = 'Express';
      }
    }
    
    // 检查 pyproject.toml 或 requirements.txt (Python 项目)
    else if (files.includes('pyproject.toml') || files.includes('requirements.txt')) {
      context.type = 'python';
      context.language = 'python';
    }
    
    // 检查 Cargo.toml (Rust 项目)
    else if (files.includes('Cargo.toml')) {
      context.type = 'rust';
      context.language = 'rust';
    }
    
    // 检查 go.mod (Go 项目)
    else if (files.includes('go.mod')) {
      context.type = 'go';
      context.language = 'go';
    }

    // 生成项目结构
    context.structure = await this.generateStructure();

    this.context = context;
    return context;
  }

  /**
   * 安全读取目录
   */
  private async safeReadDir(dir: string): Promise<string[]> {
    try {
      return await fs.readdir(dir);
    } catch {
      return [];
    }
  }

  /**
   * 读取 JSON 文件
   */
  private async readJson(filename: string): Promise<any> {
    try {
      const content = await fs.readFile(
        path.join(this.projectPath, filename),
        'utf-8'
      );
      return JSON.parse(content);
    } catch {
      return {};
    }
  }

  /**
   * 生成项目结构树
   */
  private async generateStructure(maxDepth: number = 3): Promise<string> {
    const lines: string[] = [];
    
    const walk = async (dir: string, prefix: string, depth: number) => {
      if (depth > maxDepth) return;

      const entries = await this.safeReadDir(dir);
      const filtered = entries.filter(e => 
        !e.startsWith('.') && 
        !['node_modules', 'dist', '__pycache__', 'target', 'build'].includes(e)
      );

      for (let i = 0; i < filtered.length; i++) {
        const entry = filtered[i];
        const isLast = i === filtered.length - 1;
        const connector = isLast ? '└── ' : '├── ';
        const newPrefix = prefix + (isLast ? '    ' : '│   ');

        const fullPath = path.join(dir, entry);
        const stat = await fs.stat(fullPath).catch(() => null);

        if (stat?.isDirectory()) {
          lines.push(`${prefix}${connector}${entry}/`);
          await walk(fullPath, newPrefix, depth + 1);
        } else {
          lines.push(`${prefix}${connector}${entry}`);
        }
      }
    };

    lines.push(path.basename(this.projectPath) + '/');
    await walk(this.projectPath, '', 1);

    return lines.join('\n');
  }

  /**
   * 生成上下文提示词
   */
  async generatePrompt(): Promise<string> {
    const ctx = await this.analyze();
    
    let prompt = `## 项目信息\n`;
    prompt += `- 名称: ${ctx.name}\n`;
    prompt += `- 路径: ${ctx.path}\n`;
    
    if (ctx.language) prompt += `- 语言: ${ctx.language}\n`;
    if (ctx.framework) prompt += `- 框架: ${ctx.framework}\n`;
    if (ctx.description) prompt += `- 描述: ${ctx.description}\n`;
    
    if (ctx.structure) {
      prompt += `\n## 项目结构\n\`\`\`\n${ctx.structure}\n\`\`\`\n`;
    }

    return prompt;
  }

  /**
   * 重置缓存
   */
  reset(): void {
    this.context = null;
  }
}

export const projectContext = new ProjectContextManager();
