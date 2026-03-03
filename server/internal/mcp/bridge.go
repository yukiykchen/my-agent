package mcp

import (
	"fmt"
	"sync"

	"infringement-agent-server/internal/models"
	"infringement-agent-server/internal/tools"
)

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
		toolName := fmt.Sprintf("mcp_%s_%s", info.Server, info.Name)
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
				if len(result.Content) > 0 {
					return result.Content[0].Text, nil
				}
				return "", nil
			},
		)
		
		b.registered[toolNameCopy] = serverNameCopy
		count++
	}
	
	return count, nil
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
