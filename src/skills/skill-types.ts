/**
 * Skill 类型定义
 */

// Skill 定义
export interface SkillDefinition {
  name: string;
  description: string;
  version?: string;
  author?: string;
  actions: SkillAction[];
}

// Skill 动作
export interface SkillAction {
  name: string;
  description: string;
  parameters?: {
    name: string;
    type: string;
    description: string;
    required?: boolean;
  }[];
}

// Skill 接口
export interface Skill {
  name: string;
  description: string;
  actions: string[];
  
  /**
   * 执行动作
   */
  execute(action: string, params: Record<string, any>): Promise<string>;
  
  /**
   * 获取动作列表
   */
  getActions(): SkillAction[];
}

// Skill 执行结果
export interface SkillResult {
  success: boolean;
  output: any;
  error?: string;
}
