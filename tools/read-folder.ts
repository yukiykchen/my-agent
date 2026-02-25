/**
 * 目录读取工具
 */

import * as fs from 'fs/promises';
import * as path from 'path';
import { toolRegistry } from '../tool-registry.js';

interface FileEntry {
  name: string;
  type: 'file' | 'directory';
  size?: number;
  modified?: string;
}

export async function readFolder(args: Record<string, any>): Promise<string> {
  const folderPath = path.resolve(args.path);
  const maxDepth = args.maxDepth ?? 2;
  const includeHidden = args.includeHidden ?? false;
  
  const ignoreDirs = ['node_modules', '.git', 'dist', '__pycache__', 'target', 'build'];
  
  async function readDir(dir: string, depth: number): Promise<FileEntry[]> {
    const entries: FileEntry[] = [];
    
    try {
      const items = await fs.readdir(dir, { withFileTypes: true });
      
      for (const item of items) {
        // 跳过隐藏文件
        if (!includeHidden && item.name.startsWith('.')) continue;
        
        // 跳过忽略的目录
        if (item.isDirectory() && ignoreDirs.includes(item.name)) continue;
        
        const fullPath = path.join(dir, item.name);
        const relativePath = path.relative(folderPath, fullPath);
        
        if (item.isDirectory()) {
          entries.push({
            name: relativePath + '/',
            type: 'directory'
          });
          
          // 递归读取子目录
          if (args.recursive && depth < maxDepth) {
            const subEntries = await readDir(fullPath, depth + 1);
            entries.push(...subEntries);
          }
        } else {
          const stats = await fs.stat(fullPath).catch(() => null);
          entries.push({
            name: relativePath,
            type: 'file',
            size: stats?.size,
            modified: stats?.mtime.toISOString()
          });
        }
      }
    } catch (error) {
      // 忽略无权限的目录
    }
    
    return entries;
  }
  
  try {
    const entries = await readDir(folderPath, 0);
    
    // 生成树形结构
    const tree = entries
      .sort((a, b) => a.name.localeCompare(b.name))
      .map(e => e.type === 'directory' ? `📁 ${e.name}` : `📄 ${e.name}`)
      .join('\n');
    
    return JSON.stringify({
      success: true,
      path: folderPath,
      count: entries.length,
      tree,
      entries
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
  'read_folder',
  '读取目录结构，返回文件和子目录列表',
  {
    type: 'object',
    properties: {
      path: {
        type: 'string',
        description: '要读取的目录路径'
      },
      recursive: {
        type: 'string',
        description: '是否递归读取子目录，默认 false'
      },
      maxDepth: {
        type: 'string',
        description: '递归的最大深度，默认 2'
      },
      includeHidden: {
        type: 'string',
        description: '是否包含隐藏文件，默认 false'
      }
    },
    required: ['path']
  },
  readFolder
);
