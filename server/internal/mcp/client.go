package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ==================== Types ====================

// ServerConfig MCP 服务器配置
type ServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Config .mcp.json 配置文件格式
type Config struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

// Tool MCP 工具定义
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolResult 工具调用结果
type ToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError,omitempty"`
}

// serverConnection 服务器连接
type serverConnection struct {
	process    *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Scanner
	tools      []Tool
	ready      bool
	pending    map[interface{}]chan json.RawMessage
	mu         sync.Mutex
	buffer     string
	requestID  int
}

// Client MCP 客户端
type Client struct {
	config      *Config
	configDir   string // 配置文件所在目录
	connections map[string]*serverConnection
	mu          sync.RWMutex
}

// NewClient 创建 MCP 客户端
func NewClient() *Client {
	return &Client{
		connections: make(map[string]*serverConnection),
	}
}

// LoadConfig 加载配置文件
func (c *Client) LoadConfig(path string) error {
	if path == "" {
		path = ".mcp.json"
	}
	
	// 获取配置文件的绝对路径和目录
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	c.configDir = filepath.Dir(absPath)
	
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.config = &Config{MCPServers: make(map[string]ServerConfig)}
			return nil
		}
		return err
	}
	
	return json.Unmarshal(data, &c.config)
}

// HasServers 是否有配置的服务器
func (c *Client) HasServers() bool {
	return c.config != nil && len(c.config.MCPServers) > 0
}

// GetServerStatus 获取服务器状态
func (c *Client) GetServerStatus() []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var status []map[string]interface{}
	for name, conn := range c.connections {
		status = append(status, map[string]interface{}{
			"name":  name,
			"ready": conn.ready,
			"tools": len(conn.tools),
		})
	}
	return status
}

// ConnectServer 连接到 MCP 服务器
func (c *Client) ConnectServer(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.config == nil {
		return fmt.Errorf("config not loaded")
	}
	
	cfg, ok := c.config.MCPServers[name]
	if !ok {
		return fmt.Errorf("server %s not found in config", name)
	}
	
	// 已连接
	if conn, exists := c.connections[name]; exists && conn.ready {
		return nil
	}
	
	// 解析命令路径（如果是相对路径，则相对于配置文件目录）
	cmdPath := cfg.Command
	if !filepath.IsAbs(cmdPath) {
		cmdPath = filepath.Join(c.configDir, cmdPath)
	}
	
	// 检查命令是否存在
	if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
		return fmt.Errorf("MCP server command not found: %s (resolved from %s)", cmdPath, cfg.Command)
	}
	
	// 启动子进程
	cmd := exec.Command(cmdPath, cfg.Args...)
	
	fmt.Printf("  🔧 启动 MCP 服务器: %s\n", cmdPath)
	
	// 设置环境变量
	env := os.Environ()
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create stdin pipe: %w", err)
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	
	cmd.Stderr = os.Stderr
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}
	
	conn := &serverConnection{
		process: cmd,
		stdin:   stdin,
		stdout:  bufio.NewScanner(stdout),
		tools:   []Tool{},
		pending: make(map[interface{}]chan json.RawMessage),
	}
	c.connections[name] = conn
	
	// 启动读取协程
	go c.readLoop(name, conn)
	
	// 等待进程就绪（给一点时间让进程启动）
	time.Sleep(100 * time.Millisecond)
	
	// 初始化握手
	if err := c.initialize(name, conn); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	fmt.Printf("  ✅ MCP 握手成功: %s\n", name)
	
	// 获取工具列表
	tools, err := c.listTools(name, conn)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	fmt.Printf("  📋 获取到 %d 个工具: %s\n", len(tools), name)
	
	conn.tools = tools
	conn.ready = true
	
	return nil
}

// ConnectAll 连接所有服务器
func (c *Client) ConnectAll() error {
	if c.config == nil {
		return nil
	}
	
	for name := range c.config.MCPServers {
		if err := c.ConnectServer(name); err != nil {
			fmt.Printf("⚠️  连接 MCP 服务器 %s 失败: %v\n", name, err)
		} else {
			conn := c.connections[name]
			fmt.Printf("✅ MCP 服务器 %s 已连接，工具数: %d\n", name, len(conn.tools))
		}
	}
	return nil
}

// GetTools 获取所有工具
func (c *Client) GetTools() []ToolInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var tools []ToolInfo
	for serverName, conn := range c.connections {
		if !conn.ready {
			continue
		}
		for _, t := range conn.tools {
			tools = append(tools, ToolInfo{
				Server:      serverName,
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}
	return tools
}

// ToolInfo 工具信息
type ToolInfo struct {
	Server      string
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// CallTool 调用工具
func (c *Client) CallTool(serverName, toolName string, args map[string]interface{}) (*ToolResult, error) {
	c.mu.RLock()
	conn, ok := c.connections[serverName]
	c.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("server %s not connected", serverName)
	}
	
	result, err := c.sendRequest(conn, "tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return nil, err
	}
	
	var toolResult ToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	
	return &toolResult, nil
}

// DisconnectAll 断开所有连接
func (c *Client) DisconnectAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for name, conn := range c.connections {
		if conn.process != nil && conn.process.Process != nil {
			conn.process.Process.Kill()
			fmt.Printf("🔌 已断开 MCP 服务器: %s\n", name)
		}
	}
	c.connections = make(map[string]*serverConnection)
}

// ==================== Internal Methods ====================

func (c *Client) readLoop(serverName string, conn *serverConnection) {
	for conn.stdout.Scan() {
		line := conn.stdout.Bytes()
		
		var resp struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result,omitempty"`
			Error  interface{}     `json:"error,omitempty"`
		}
		
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		
		conn.mu.Lock()
		if ch, ok := conn.pending[parseID(resp.ID)]; ok {
			ch <- resp.Result
			delete(conn.pending, parseID(resp.ID))
		}
		conn.mu.Unlock()
	}
}

func (c *Client) initialize(serverName string, conn *serverConnection) error {
	_, err := c.sendRequest(conn, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "infringement-agent",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return err
	}
	
	// 发送 initialized 通知
	c.sendNotification(conn, "notifications/initialized", map[string]interface{}{})
	
	return nil
}

func (c *Client) listTools(serverName string, conn *serverConnection) ([]Tool, error) {
	result, err := c.sendRequest(conn, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	
	var resp struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}
	
	return resp.Tools, nil
}

func (c *Client) sendRequest(conn *serverConnection, method string, params interface{}) (json.RawMessage, error) {
	conn.mu.Lock()
	conn.requestID++
	id := conn.requestID
	ch := make(chan json.RawMessage, 1)
	conn.pending[id] = ch
	conn.mu.Unlock()
	
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	
	data, _ := json.Marshal(req)
	conn.stdin.Write(append(data, '\n'))
	
	select {
	case result := <-ch:
		return result, nil
	case <-time.After(30 * time.Second):
		conn.mu.Lock()
		delete(conn.pending, id)
		conn.mu.Unlock()
		return nil, fmt.Errorf("timeout after 30s")
	}
}

func (c *Client) sendNotification(conn *serverConnection, method string, params interface{}) {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	data, _ := json.Marshal(req)
	conn.stdin.Write(append(data, '\n'))
}

func parseID(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		// 转换为 int，因为我们的 requestID 是 int 类型
		return int(n)
	}
	return nil
}
