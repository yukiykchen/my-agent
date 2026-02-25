/**
 * 文件读取工具
 */

import * as fs from 'fs/promises';
import * as path from 'path';
import { toolRegistry } from '../tool-registry.js';

export async function readFile(args: Record<string, any>): Promise<string> {
  const filePath = path.resolve(args.path);
  const encoding = (args.encoding || 'utf-8') as BufferEncoding;
  
  try {
    const content = await fs.readFile(filePath, encoding);
    const stats = await fs.stat(filePath);
    
    return JSON.stringify({
      success: true,
      path: filePath,
      content,
      size: stats.size,
      modified: stats.mtime.toISOString()
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
  'read_file',
  '读取指定路径的文件内容',
  {
    type: 'object',
    properties: {
      path: {
        type: 'string',
        description: '要读取的文件路径'
      },
      encoding: {
        type: 'string',
        description: '文件编码，默认 utf-8'
      }
    },
    required: ['path']
  },
  readFile
);
