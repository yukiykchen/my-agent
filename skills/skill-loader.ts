/**
 * Skill 加载器
 * 从文件系统加载 Skill 定义
 */

import * as fs from 'fs/promises';
import * as path from 'path';
import matter from 'gray-matter';
import type { SkillDefinition } from './skill-types.js';

/**
 * Skill 加载器
 */
export class SkillLoader {
  private skillsDir: string;

  constructor(skillsDir?: string) {
    this.skillsDir = skillsDir || path.join(process.cwd(), '.agent', 'skills');
  }

  /**
   * 加载单个 Skill 定义
   */
  async load(name: string): Promise<SkillDefinition | null> {
    const filePath = path.join(this.skillsDir, `${name}.md`);
    
    try {
      const content = await fs.readFile(filePath, 'utf-8');
      const { data, content: body } = matter(content);
      
      return {
        name: data.name || name,
        description: data.description || '',
        version: data.version,
        author: data.author,
        actions: data.actions || []
      };
    } catch {
      return null;
    }
  }

  /**
   * 加载所有 Skill 定义
   */
  async loadAll(): Promise<SkillDefinition[]> {
    const skills: SkillDefinition[] = [];
    
    try {
      const files = await fs.readdir(this.skillsDir);
      
      for (const file of files) {
        if (file.endsWith('.md')) {
          const name = file.replace('.md', '');
          const skill = await this.load(name);
          if (skill) {
            skills.push(skill);
          }
        }
      }
    } catch {
      // 目录不存在
    }
    
    return skills;
  }

  /**
   * 检查 Skill 是否存在
   */
  async exists(name: string): Promise<boolean> {
    const filePath = path.join(this.skillsDir, `${name}.md`);
    try {
      await fs.access(filePath);
      return true;
    } catch {
      return false;
    }
  }
}

export const skillLoader = new SkillLoader();
