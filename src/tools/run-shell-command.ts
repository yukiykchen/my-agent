/**
 * Shell 命令执行工具
 */

import { exec, spawn } from 'child_process';
import { promisify } from 'util';
import { toolRegistry } from '../tool-registry.js';

const execAsync = promisify(exec);

// 危险命令黑名单
const DANGEROUS_COMMANDS = [
  'rm -rf /',
  'rm -rf ~',
  'mkfs',
  'dd if=',
  ':(){:|:&};:',
  '> /dev/sda',
  'chmod -R 777 /',
  'chown -R'
];

export async function runShellCommand(args: Record<string, any>): Promise<string> {
  const { command, cwd, timeout = 30000 } = args;
  
  // 安全检查
  const lowerCmd = command.toLowerCase();
  for (const dangerous of DANGEROUS_COMMANDS) {
    if (lowerCmd.includes(dangerous.toLowerCase())) {
      return JSON.stringify({
        success: false,
        error: `危险命令被阻止: ${command}`
      });
    }
  }
  
  try {
    const { stdout, stderr } = await execAsync(command, {
      cwd: cwd || process.cwd(),
      timeout,
      maxBuffer: 1024 * 1024 * 10 // 10MB
    });
    
    return JSON.stringify({
      success: true,
      command,
      stdout: stdout.trim(),
      stderr: stderr.trim()
    });
  } catch (error: any) {
    return JSON.stringify({
      success: false,
      command,
      error: error.message,
      stdout: error.stdout?.trim(),
      stderr: error.stderr?.trim(),
      exitCode: error.code
    });
  }
}

// 注册工具
toolRegistry.register(
  'run_shell_command',
  '执行 Shell 命令并返回输出结果',
  {
    type: 'object',
    properties: {
      command: {
        type: 'string',
        description: '要执行的 Shell 命令'
      },
      cwd: {
        type: 'string',
        description: '命令执行的工作目录'
      },
      timeout: {
        type: 'string',
        description: '超时时间（毫秒），默认 30000'
      }
    },
    required: ['command']
  },
  runShellCommand
);
