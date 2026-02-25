/**
 * Skill 注册中心
 */

import type { Skill, SkillDefinition, SkillAction } from './skill-types.js';

/**
 * Skill 基类
 */
export abstract class BaseSkill implements Skill {
  name: string;
  description: string;
  actions: string[];
  protected definition: SkillDefinition;

  constructor(definition: SkillDefinition) {
    this.definition = definition;
    this.name = definition.name;
    this.description = definition.description;
    this.actions = definition.actions.map(a => a.name);
  }

  abstract execute(action: string, params: Record<string, any>): Promise<string>;

  getActions(): SkillAction[] {
    return this.definition.actions;
  }
}

/**
 * Skill 注册中心
 */
class SkillRegistryClass {
  private skills: Map<string, Skill> = new Map();

  /**
   * 注册 Skill
   */
  register(skill: Skill): void {
    this.skills.set(skill.name, skill);
  }

  /**
   * 获取 Skill
   */
  get(name: string): Skill | undefined {
    return this.skills.get(name);
  }

  /**
   * 列出所有 Skill
   */
  list(): { name: string; description: string; actions: string[] }[] {
    return Array.from(this.skills.values()).map(s => ({
      name: s.name,
      description: s.description,
      actions: s.actions
    }));
  }

  /**
   * 检查是否存在
   */
  has(name: string): boolean {
    return this.skills.has(name);
  }

  /**
   * 获取数量
   */
  get size(): number {
    return this.skills.size;
  }
}

export const skillRegistry = new SkillRegistryClass();
