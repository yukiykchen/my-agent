/**
 * 文件写入工具
 */

import * as fs from 'fs/promises';
import * as path from 'path';
import { toolRegistry } from '../tool-registry.js';

export async function writeFile(args: Record<string, any>): Promise<string> {
  const filePath = path.resolve(args.path);
  const encoding = (args.encoding || 'utf-8') as BufferEncoding;
  
  try {
    // 创建父目录
    if (args.createDirs !== false) {
      await fs.mkdir(path.dirname(filePath), { recursive: true });
    }
    
    await fs.writeFile(filePath, args.content, encoding);
    const stats = await fs.stat(filePath);
    
    return JSON.stringify({
      success: true,
      path: filePath,
      size: stats.size,
      message: `文件已写入: ${filePath}`
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
  'write_file',
  '将内容写入指定路径的文件（会覆盖已有文件）',
  {
    type: 'object',
    properties: {
      path: {
        type: 'string',
        description: '要写入的文件路径'
      },
      content: {
        type: 'string',
        description: '要写入的内容'
      },
      encoding: {
        type: 'string',
        description: '文件编码，默认 utf-8'
      },
      createDirs: {
        type: 'string',
        description: '是否自动创建父目录，默认 true'
      }
    },
    required: ['path', 'content']
  },
  writeFile
);
