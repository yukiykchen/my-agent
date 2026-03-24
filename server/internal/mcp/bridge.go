package mcp

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"

	"infringement-agent-server/internal/models"
	"infringement-agent-server/internal/tools"
)

// sanitizeToolName 清理工具名称，确保符合 LLM API 的命名规则
// 规则：以字母开头，只能包含字母、数字、下划线和短横线
var invalidNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitizeToolName(name string) string {
	// 将点号等非法字符替换为下划线
	sanitized := invalidNameChars.ReplaceAllString(name, "_")
	// 合并连续下划线
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}
	// 去掉首尾下划线
	sanitized = strings.Trim(sanitized, "_")
	// 确保以字母开头
	if len(sanitized) > 0 && !((sanitized[0] >= 'a' && sanitized[0] <= 'z') || (sanitized[0] >= 'A' && sanitized[0] <= 'Z')) {
		sanitized = "t_" + sanitized
	}
	return sanitized
}

// Bridge MCP 工具桥接器
type Bridge struct {
	client     *Client
	registry   *tools.Registry
	registered map[string]string // toolName -> serverName
	mu         sync.RWMutex
}

// NewBridge 创建桥接器
func NewBridge(client *Client, registry *tools.Registry) *Bridge {
	return &Bridge{
		client:     client,
		registry:   registry,
		registered: make(map[string]string),
	}
}

// RegisterAll 注册所有 MCP 工具到工具注册中心
func (b *Bridge) RegisterAll() (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	toolInfos := b.client.GetTools()
	count := 0

	for _, info := range toolInfos {
		toolName := sanitizeToolName(fmt.Sprintf("mcp_%s_%s", info.Server, info.Name))
		serverName := info.Server

		// 转换 InputSchema 到 FunctionParams
		params := convertInputSchema(info.InputSchema)

		// 注册到工具中心
		toolNameCopy := toolName
		serverNameCopy := serverName
		toolNameForCall := info.Name

		b.registry.Register(
			toolNameCopy,
			info.Description,
			params,
			func(args map[string]interface{}) (string, error) {
				result, err := b.client.CallTool(serverNameCopy, toolNameForCall, args)
				if err != nil {
					return "", err
				}
				// 从 mcp.CallToolResult 中提取文本内容
				return extractTextFromResult(result), nil
			},
		)

		b.registered[toolNameCopy] = serverNameCopy
		count++
	}

	return count, nil
}

// extractTextFromResult 从 MCP SDK 的 CallToolResult 中提取文本
func extractTextFromResult(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			return textContent.Text
		}
	}
	return ""
}

// GetRegisteredTools 获取已注册的 MCP 工具
func (b *Bridge) GetRegisteredTools() map[string]string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range b.registered {
		result[k] = v
	}
	return result
}

func convertInputSchema(schema map[string]interface{}) models.FunctionParams {
	params := models.FunctionParams{
		Type:       "object",
		Properties: make(map[string]models.PropertyDefine),
	}

	if schema == nil {
		return params
	}

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for name, p := range props {
			if prop, ok := p.(map[string]interface{}); ok {
				pd := models.PropertyDefine{}
				if t, ok := prop["type"].(string); ok {
					pd.Type = t
				}
				if d, ok := prop["description"].(string); ok {
					pd.Description = d
				}
				if e, ok := prop["enum"].([]interface{}); ok {
					for _, v := range e {
						if s, ok := v.(string); ok {
							pd.Enum = append(pd.Enum, s)
						}
					}
				}
				params.Properties[name] = pd
			}
		}
	}

	if req, ok := schema["required"].([]interface{}); ok {
		for _, v := range req {
			if s, ok := v.(string); ok {
				params.Required = append(params.Required, s)
			}
		}
	}

	return params
}
