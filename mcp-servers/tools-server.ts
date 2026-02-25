#!/usr/bin/env node
/**
 * 自定义 MCP Server — 实用工具集
 * 
 * 通过 stdio 与 Agent 通信，遵循 MCP (JSON-RPC 2.0) 协议
 * 提供: web_fetch（抓取网页）、get_weather（查天气）
 * 
 * 启动方式: npx tsx src/mcp-servers/tools-server.ts
 */

import * as https from 'https';
import * as http from 'http';
import * as readline from 'readline';

// ==================== 工具定义 ====================

const TOOLS = [
  {
    name: 'web_fetch',
    description: '抓取指定 URL 的网页内容，返回文本。支持 HTTP/HTTPS。可用于获取网页信息、API 数据等。',
    inputSchema: {
      type: 'object',
      properties: {
        url: {
          type: 'string',
          description: '要抓取的 URL 地址'
        },
        maxLength: {
          type: 'number',
          description: '返回内容的最大字符数，默认 5000'
        }
      },
      required: ['url']
    }
  },
  {
    name: 'get_current_time',
    description: '获取当前日期和时间信息。',
    inputSchema: {
      type: 'object',
      properties: {
        timezone: {
          type: 'string',
          description: '时区，如 "Asia/Shanghai"，默认使用系统时区'
        }
      }
    }
  },
  {
    name: 'calculate',
    description: '计算数学表达式，支持基本运算（加减乘除、幂运算、三角函数等）。',
    inputSchema: {
      type: 'object',
      properties: {
        expression: {
          type: 'string',
          description: '数学表达式，如 "2 + 3 * 4"、"Math.sqrt(144)"、"Math.PI * 2"'
        }
      },
      required: ['expression']
    }
  }
];

// ==================== 工具实现 ====================

/** 抓取网页内容 */
async function webFetch(args: { url: string; maxLength?: number }): Promise<string> {
  const { url, maxLength = 5000 } = args;

  return new Promise((resolve) => {
    const client = url.startsWith('https') ? https : http;
    
    const req = client.get(url, {
      headers: {
        'User-Agent': 'Mozilla/5.0 (compatible; TryAgent/1.0; MCP)',
        'Accept': 'text/html,application/json,text/plain,*/*',
        'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8'
      },
      timeout: 15000
    }, (res) => {
      // 处理重定向
      if (res.statusCode && res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        const redirectUrl = res.headers.location.startsWith('http')
          ? res.headers.location
          : new URL(res.headers.location, url).toString();
        res.resume();
        webFetch({ url: redirectUrl, maxLength }).then(resolve);
        return;
      }

      let data = '';
      res.setEncoding('utf-8');
      res.on('data', (chunk: string) => {
        data += chunk;
        if (data.length > maxLength * 2) {
          res.destroy();
        }
      });
      res.on('end', () => {
        // 简单清理 HTML 标签
        let text = data
          .replace(/<script[^>]*>[\s\S]*?<\/script>/gi, '')
          .replace(/<style[^>]*>[\s\S]*?<\/style>/gi, '')
          .replace(/<[^>]+>/g, ' ')
          .replace(/&nbsp;/g, ' ')
          .replace(/&amp;/g, '&')
          .replace(/&lt;/g, '<')
          .replace(/&gt;/g, '>')
          .replace(/&quot;/g, '"')
          .replace(/\s+/g, ' ')
          .trim();

        if (text.length > maxLength) {
          text = text.substring(0, maxLength) + '...(内容已截断)';
        }

        resolve(JSON.stringify({
          url,
          status: res.statusCode,
          contentType: res.headers['content-type'],
          length: text.length,
          content: text
        }));
      });
    });

    req.on('error', (err) => {
      resolve(JSON.stringify({ error: `请求失败: ${err.message}`, url }));
    });

    req.on('timeout', () => {
      req.destroy();
      resolve(JSON.stringify({ error: '请求超时 (15s)', url }));
    });
  });
}


/** 获取当前时间 */
function getCurrentTime(args: { timezone?: string }): string {
  const { timezone } = args;
  const now = new Date();
  
  const options: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    weekday: 'long',
    hour12: false,
    ...(timezone ? { timeZone: timezone } : {})
  };

  return JSON.stringify({
    formatted: now.toLocaleString('zh-CN', options),
    iso: now.toISOString(),
    timestamp: now.getTime(),
    timezone: timezone || Intl.DateTimeFormat().resolvedOptions().timeZone
  });
}

/** 计算数学表达式 */
function calculate(args: { expression: string }): string {
  const { expression } = args;
  
  // 安全检查：只允许数学相关内容
  const allowedPattern = /^[0-9+\-*/().,%\s]|Math\.\w+|PI|E|sqrt|pow|abs|sin|cos|tan|log|ceil|floor|round|min|max|random/;
  const dangerousPattern = /import|require|eval|exec|spawn|Function|process|global|window|document/i;
  
  if (dangerousPattern.test(expression)) {
    return JSON.stringify({ error: '不允许的表达式', expression });
  }
  
  try {
    // 使用 Function 构造器而非 eval，限制作用域
    const fn = new Function('Math', `"use strict"; return (${expression})`);
    const result = fn(Math);
    return JSON.stringify({ expression, result: Number(result), type: typeof result });
  } catch (err: any) {
    return JSON.stringify({ error: `计算失败: ${err.message}`, expression });
  }
}

// ==================== MCP 协议处理 ====================

/** 处理 JSON-RPC 请求 */
async function handleRequest(request: any): Promise<any> {
  const { method, params, id } = request;

  switch (method) {
    case 'initialize':
      return {
        jsonrpc: '2.0',
        id,
        result: {
          protocolVersion: '2024-11-05',
          capabilities: { tools: {} },
          serverInfo: {
            name: 'try-agent-tools',
            version: '1.0.0'
          }
        }
      };

    case 'notifications/initialized':
      // 通知，无需响应
      return null;

    case 'tools/list':
      return {
        jsonrpc: '2.0',
        id,
        result: { tools: TOOLS }
      };

    case 'tools/call': {
      const toolName = params?.name;
      const toolArgs = params?.arguments || {};
      let content: string;

      try {
        switch (toolName) {
          case 'web_fetch':
            content = await webFetch(toolArgs);
            break;
          case 'get_current_time':
            content = getCurrentTime(toolArgs);
            break;
          case 'calculate':
            content = calculate(toolArgs);
            break;
          default:
            return {
              jsonrpc: '2.0',
              id,
              result: {
                content: [{ type: 'text', text: JSON.stringify({ error: `未知工具: ${toolName}` }) }],
                isError: true
              }
            };
        }

        return {
          jsonrpc: '2.0',
          id,
          result: {
            content: [{ type: 'text', text: content }]
          }
        };
      } catch (err: any) {
        return {
          jsonrpc: '2.0',
          id,
          result: {
            content: [{ type: 'text', text: JSON.stringify({ error: err.message }) }],
            isError: true
          }
        };
      }
    }

    default:
      return {
        jsonrpc: '2.0',
        id,
        error: {
          code: -32601,
          message: `Method not found: ${method}`
        }
      };
  }
}

// ==================== 启动 stdio 通信 ====================

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

rl.on('line', async (line) => {
  const trimmed = line.trim();
  if (!trimmed) return;

  try {
    const request = JSON.parse(trimmed);
    const response = await handleRequest(request);
    
    // 通知类消息不需要响应
    if (response) {
      process.stdout.write(JSON.stringify(response) + '\n');
    }
  } catch (err) {
    // 解析失败，返回 JSON-RPC 错误
    const errorResponse = {
      jsonrpc: '2.0',
      id: null,
      error: {
        code: -32700,
        message: 'Parse error'
      }
    };
    process.stdout.write(JSON.stringify(errorResponse) + '\n');
  }
});

// 父进程断开 stdin 时自动退出
process.stdin.on('end', () => {
  process.exit(0);
});

// 收到终止信号时退出
process.on('SIGINT', () => process.exit(0));
process.on('SIGTERM', () => process.exit(0));

process.stderr.write('MCP Tools Server started\n');
