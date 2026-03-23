package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	mcpsdk "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
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

// ToolInfo 工具信息
type ToolInfo struct {
	Server      string
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// serverConnection 服务器连接
type serverConnection struct {
	client *mcpsdk.Client
	tools  []mcp.Tool
	ready  bool
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

	fmt.Printf("  🔧 启动 MCP 服务器: %s\n", cmdPath)

	// 构建环境变量
	env := os.Environ()
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}

	// 使用 mcp-go SDK 创建 stdio 客户端
	sdkClient, err := mcpsdk.NewStdioMCPClient(cmdPath, env, cfg.Args...)
	if err != nil {
		return fmt.Errorf("create MCP client: %w", err)
	}

	// 初始化握手
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = sdkClient.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "infringement-agent",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	})
	if err != nil {
		sdkClient.Close()
		return fmt.Errorf("initialize: %w", err)
	}
	fmt.Printf("  ✅ MCP 握手成功: %s\n", name)

	// 获取工具列表
	toolsResult, err := sdkClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		sdkClient.Close()
		return fmt.Errorf("list tools: %w", err)
	}
	fmt.Printf("  📋 获取到 %d 个工具: %s\n", len(toolsResult.Tools), name)

	conn := &serverConnection{
		client: sdkClient,
		tools:  toolsResult.Tools,
		ready:  true,
	}
	c.connections[name] = conn

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
			// 将 mcp.Tool 的 InputSchema 转换为 map
			inputSchema := make(map[string]interface{})
			schemaBytes, _ := json.Marshal(t.InputSchema)
			json.Unmarshal(schemaBytes, &inputSchema)

			tools = append(tools, ToolInfo{
				Server:      serverName,
				Name:        t.Name,
				Description: t.Description,
				InputSchema: inputSchema,
			})
		}
	}
	return tools
}

// CallTool 调用工具
func (c *Client) CallTool(serverName, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	c.mu.RLock()
	conn, ok := c.connections[serverName]
	c.mu.RUnlock()

	if !ok || !conn.ready {
		return nil, fmt.Errorf("server %s not connected", serverName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := conn.client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// DisconnectAll 断开所有连接
func (c *Client) DisconnectAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for name, conn := range c.connections {
		if conn.client != nil {
			conn.client.Close()
			fmt.Printf("🔌 已断开 MCP 服务器: %s\n", name)
		}
	}
	c.connections = make(map[string]*serverConnection)
}
