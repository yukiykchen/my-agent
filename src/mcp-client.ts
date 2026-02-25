/**
 * MCP 协议客户端
 * 基于 JSON-RPC 2.0 over stdio 与 MCP 服务器通信
 * 
 * MCP (Model Context Protocol) 让 Agent 能以即插即用的方式对接外部工具
 * 相当于 AI 世界的 USB 接口标准
 */

import { spawn, ChildProcess } from 'child_process';
import * as fs from 'fs/promises';
import * as path from 'path';
import { EventEmitter } from 'events';

// ==================== 类型定义 ====================

/** MCP 服务器配置（.mcp.json 中的每一项） */
export interface MCPServerConfig {
  command: string;
  args?: string[];
  env?: Record<string, string>;
}

/** .mcp.json 配置文件格式 */
export interface MCPConfig {
  mcpServers: Record<string, MCPServerConfig>;
}

/** MCP 工具定义（从服务器获取） */
export interface MCPTool {
  name: string;
  description: string;
  inputSchema: {
    type: string;
    properties: Record<string, any>;
    required?: string[];
  };
}

/** JSON-RPC 2.0 请求 */
interface JsonRpcRequest {
  jsonrpc: '2.0';
  id: number;
  method: string;
  params?: Record<string, any>;
}

/** JSON-RPC 2.0 响应 */
interface JsonRpcResponse {
  jsonrpc: '2.0';
  id: number;
  result?: any;
  error?: {
    code: number;
    message: string;
    data?: any;
  };
}

/** MCP 工具调用结果 */
export interface MCPToolResult {
  content: Array<{
    type: string;
    text?: string;
  }>;
  isError?: boolean;
}

/** 服务器连接信息 */
interface ServerConnection {
  process: ChildProcess;
  config: MCPServerConfig;
  tools: MCPTool[];
  ready: boolean;
  pendingRequests: Map<number, {
    resolve: (value: any) => void;
    reject: (reason: any) => void;
    timer: ReturnType<typeof setTimeout>;
  }>;
  buffer: string; // 用于处理不完整的 JSON 数据
}

// ==================== MCP Client ====================

export class MCPClient extends EventEmitter {
  private config: MCPConfig | null = null;
  private connections: Map<string, ServerConnection> = new Map();
  private requestId = 0;

  constructor() {
    super();
  }

  /**
   * 加载 .mcp.json 配置文件
   */
  async loadConfig(configPath?: string): Promise<MCPConfig> {
    const mcpConfigPath = configPath || path.join(process.cwd(), '.mcp.json');
    
    try {
      const content = await fs.readFile(mcpConfigPath, 'utf-8');
      this.config = JSON.parse(content);
      return this.config!;
    } catch (err: any) {
      if (err.code === 'ENOENT') {
        this.config = { mcpServers: {} };
        return this.config;
      }
      throw new Error(`Failed to load MCP config: ${err.message}`);
    }
  }

  /**
   * 启动并连接一个 MCP 服务器
   */
  async connectServer(name: string): Promise<MCPTool[]> {
    if (!this.config) {
      await this.loadConfig();
    }

    const serverConfig = this.config?.mcpServers[name];
    if (!serverConfig) {
      throw new Error(`MCP server "${name}" not found in config`);
    }

    // 如果已连接，直接返回工具列表
    if (this.connections.has(name) && this.connections.get(name)!.ready) {
      return this.connections.get(name)!.tools;
    }

    // 启动子进程
    const proc = spawn(serverConfig.command, serverConfig.args || [], {
      stdio: ['pipe', 'pipe', 'pipe'],
      env: { ...process.env, ...serverConfig.env }
    });

    const connection: ServerConnection = {
      process: proc,
      config: serverConfig,
      tools: [],
      ready: false,
      pendingRequests: new Map(),
      buffer: ''
    };

    this.connections.set(name, connection);

    // 监听 stdout（JSON-RPC 响应）
    proc.stdout!.on('data', (data: Buffer) => {
      this.handleServerData(name, data);
    });

    // 监听 stderr（调试/错误信息）
    proc.stderr!.on('data', (data: Buffer) => {
      const msg = data.toString().trim();
      if (msg) {
        this.emit('server-log', { server: name, message: msg });
      }
    });

    // 进程退出处理
    proc.on('exit', (code) => {
      this.emit('server-exit', { server: name, code });
      this.connections.delete(name);
    });

    proc.on('error', (error) => {
      this.emit('server-error', { server: name, error: error.message });
      this.connections.delete(name);
    });

    // 等待进程启动
    await this.waitForProcess(proc);

    // 初始化 MCP 握手
    await this.initialize(name);

    // 获取工具列表
    const tools = await this.listTools(name);
    connection.tools = tools;
    connection.ready = true;

    this.emit('server-ready', { server: name, tools: tools.length });
    return tools;
  }

  /**
   * 等待子进程就绪
   */
  private waitForProcess(proc: ChildProcess): Promise<void> {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => resolve(), 5000); // 给进程 5 秒启动时间（npx 首次下载可能较慢）
      
      proc.on('error', (err) => {
        clearTimeout(timer);
        reject(new Error(`Failed to start MCP server: ${err.message}`));
      });

      // 如果 stdout 有数据，说明进程已经启动
      proc.stdout!.once('data', () => {
        clearTimeout(timer);
        resolve();
      });
    });
  }

  /**
   * 发送 MCP initialize 握手
   */
  private async initialize(serverName: string): Promise<void> {
    const result = await this.sendRequest(serverName, 'initialize', {
      protocolVersion: '2024-11-05',
      capabilities: {},
      clientInfo: {
        name: 'try-agent',
        version: '1.0.0'
      }
    });

    // 发送 initialized 通知
    this.sendNotification(serverName, 'notifications/initialized', {});
    
    return result;
  }

  /**
   * 获取服务器的工具列表
   */
  private async listTools(serverName: string): Promise<MCPTool[]> {
    const result = await this.sendRequest(serverName, 'tools/list', {});
    return result?.tools || [];
  }

  /**
   * 调用 MCP 工具
   */
  async callTool(serverName: string, toolName: string, args: Record<string, any>): Promise<MCPToolResult> {
    const result = await this.sendRequest(serverName, 'tools/call', {
      name: toolName,
      arguments: args
    });
    return result as MCPToolResult;
  }

  /**
   * 发送 JSON-RPC 请求并等待响应
   */
  private sendRequest(serverName: string, method: string, params: Record<string, any>): Promise<any> {
    const connection = this.connections.get(serverName);
    if (!connection) {
      throw new Error(`MCP server "${serverName}" is not connected`);
    }

    const id = ++this.requestId;
    const request: JsonRpcRequest = {
      jsonrpc: '2.0',
      id,
      method,
      params
    };

    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        connection.pendingRequests.delete(id);
        reject(new Error(`MCP request timeout: ${method} (${serverName})`));
      }, 60000); // 60 秒超时，npx 首次下载可能较慢

      connection.pendingRequests.set(id, { resolve, reject, timer });

      const data = JSON.stringify(request);
      connection.process.stdin!.write(data + '\n');
    });
  }

  /**
   * 发送 JSON-RPC 通知（不需要响应）
   */
  private sendNotification(serverName: string, method: string, params: Record<string, any>): void {
    const connection = this.connections.get(serverName);
    if (!connection) return;

    const notification = {
      jsonrpc: '2.0',
      method,
      params
    };

    connection.process.stdin!.write(JSON.stringify(notification) + '\n');
  }

  /**
   * 处理服务器返回的数据
   */
  private handleServerData(serverName: string, data: Buffer): void {
    const connection = this.connections.get(serverName);
    if (!connection) return;

    // 追加到缓冲区
    connection.buffer += data.toString();

    // 尝试解析完整的 JSON 行
    const lines = connection.buffer.split('\n');
    connection.buffer = lines.pop() || ''; // 保留最后一个不完整的行

    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed) continue;

      try {
        const response = JSON.parse(trimmed) as JsonRpcResponse;
        
        if (response.id !== undefined) {
          // 这是对请求的响应
          const pending = connection.pendingRequests.get(response.id);
          if (pending) {
            clearTimeout(pending.timer);
            connection.pendingRequests.delete(response.id);
            
            if (response.error) {
              pending.reject(new Error(`MCP error: ${response.error.message}`));
            } else {
              pending.resolve(response.result);
            }
          }
        } else {
          // 这是服务器的通知
          this.emit('notification', { server: serverName, data: response });
        }
      } catch {
        // 非 JSON 数据，忽略
      }
    }
  }

  /**
   * 连接所有配置的服务器
   */
  async connectAll(): Promise<Map<string, MCPTool[]>> {
    if (!this.config) {
      await this.loadConfig();
    }

    const results = new Map<string, MCPTool[]>();
    const serverNames = Object.keys(this.config?.mcpServers || {});

    for (const name of serverNames) {
      try {
        const tools = await this.connectServer(name);
        results.set(name, tools);
      } catch (error: any) {
        this.emit('server-error', { server: name, error: error.message });
        console.error(`Failed to connect MCP server "${name}":`, error.message);
      }
    }

    return results;
  }

  /**
   * 断开一个服务器
   */
  disconnectServer(name: string): void {
    const connection = this.connections.get(name);
    if (connection) {
      // 清理所有待处理请求
      for (const [id, pending] of connection.pendingRequests) {
        clearTimeout(pending.timer);
        pending.reject(new Error('Server disconnected'));
      }
      // 关闭 stdin，子进程会收到 'end' 事件自动退出
      try { connection.process.stdin?.end(); } catch {}
      // 直接强杀
      try { connection.process.kill('SIGKILL'); } catch {}
      this.connections.delete(name);
    }
  }

  /**
   * 断开所有服务器
   */
  disconnectAll(): void {
    for (const [name] of this.connections) {
      this.disconnectServer(name);
    }
  }

  /**
   * 获取所有已连接服务器的工具列表
   */
  getAllTools(): Array<{ server: string; tool: MCPTool }> {
    const allTools: Array<{ server: string; tool: MCPTool }> = [];
    for (const [serverName, connection] of this.connections) {
      if (connection.ready) {
        for (const tool of connection.tools) {
          allTools.push({ server: serverName, tool });
        }
      }
    }
    return allTools;
  }

  /**
   * 查找工具所属的服务器
   */
  findToolServer(toolName: string): string | null {
    for (const [serverName, connection] of this.connections) {
      if (connection.ready && connection.tools.some(t => t.name === toolName)) {
        return serverName;
      }
    }
    return null;
  }

  /**
   * 获取服务器列表和状态
   */
  getServerStatus(): Array<{ name: string; ready: boolean; toolCount: number }> {
    if (!this.config) return [];
    
    return Object.keys(this.config.mcpServers).map(name => {
      const conn = this.connections.get(name);
      return {
        name,
        ready: conn?.ready || false,
        toolCount: conn?.tools.length || 0
      };
    });
  }

  /**
   * 检查是否有可用的 MCP 服务器
   */
  hasServers(): boolean {
    return Object.keys(this.config?.mcpServers || {}).length > 0;
  }

  /**
   * 检查服务器是否已连接
   */
  isConnected(name: string): boolean {
    return this.connections.get(name)?.ready || false;
  }
}

// 导出全局单例
export const mcpClient = new MCPClient();
