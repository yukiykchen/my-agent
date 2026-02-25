/**
 * 文件编辑工具
 * 支持搜索替换操作
 */

import * as fs from 'fs/promises';
import * as path from 'path';
import { toolRegistry } from '../tool-registry.js';

export async function editFile(args: Record<string, any>): Promise<string> {
  const filePath = path.resolve(args.path);
  
  try {
    // 读取文件
    const content = await fs.readFile(filePath, 'utf-8');
    
    // 检查 old_str 是否存在
    if (!content.includes(args.old_str)) {
      return JSON.stringify({
        success: false,
        error: '未找到要替换的内容'
      });
    }
    
    // 检查 old_str 是否唯一
    const occurrences = content.split(args.old_str).length - 1;
    if (occurrences > 1) {
      return JSON.stringify({
        success: false,
        error: `找到 ${occurrences} 处匹配，请提供更多上下文使其唯一`
      });
    }
    
    // 执行替换
    const newContent = content.replace(args.old_str, args.new_str);
    await fs.writeFile(filePath, newContent, 'utf-8');
    
    return JSON.stringify({
      success: true,
      path: filePath,
      message: '文件已更新'
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
  'edit_file',
  '编辑文件：将 old_str 替换为 new_str。old_str 必须在文件中唯一存在',
  {
    type: 'object',
    properties: {
      path: {
        type: 'string',
        description: '要编辑的文件路径'
      },
      old_str: {
        type: 'string',
        description: '要被替换的原始内容（必须唯一）'
      },
      new_str: {
        type: 'string',
        description: '替换后的新内容'
      }
    },
    required: ['path', 'old_str', 'new_str']
  },
  editFile
);
