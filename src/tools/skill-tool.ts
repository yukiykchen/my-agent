/**
 * Skill 调用工具
 */

import { toolRegistry } from '../tool-registry.js';
import { skillRegistry } from '../skills/skill-registry.js';

export async function callSkill(args: Record<string, any>): Promise<string> {
  const { name, action, params } = args;
  
  try {
    const skill = skillRegistry.get(name);
    if (!skill) {
      return JSON.stringify({
        success: false,
        error: `未找到 Skill: ${name}`,
        available: skillRegistry.list().map(s => s.name)
      });
    }
    
    const result = await skill.execute(action, params || {});
    
    return JSON.stringify({
      success: true,
      skill: name,
      action,
      result
    });
  } catch (error: any) {
    return JSON.stringify({
      success: false,
      error: error.message
    });
  }
}

// 注册工具
toolRegistry.register(
  'call_skill',
  '调用特定的 Skill 技能来完成任务',
  {
    type: 'object',
    properties: {
      name: {
        type: 'string',
        description: 'Skill 名称'
      },
      action: {
        type: 'string',
        description: '要执行的动作'
      },
      params: {
        type: 'string',
        description: '动作参数（JSON 格式）'
      }
    },
    required: ['name', 'action']
  },
  callSkill
);
