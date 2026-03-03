package tools

import (
	"fmt"
	"sync"

	"infringement-agent-server/internal/models"
)

// Executor 工具执行函数
type Executor func(args map[string]interface{}) (string, error)

// RegisteredTool 注册的工具
type RegisteredTool struct {
	Definition models.ToolDefinition
	Exec       Executor
}

// Registry 工具注册中心
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*RegisteredTool
}

// NewRegistry 创建工具注册中心
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*RegisteredTool),
	}
}

// Register 注册工具
func (r *Registry) Register(name, description string, params models.FunctionParams, exec Executor) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[name] = &RegisteredTool{
		Definition: models.ToolDefinition{
			Type: "function",
			Function: models.FunctionDefine{
				Name:        name,
				Description: description,
				Parameters:  params,
			},
		},
		Exec: exec,
	}
}

// Execute 执行工具
func (r *Registry) Execute(name string, args map[string]interface{}) (string, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	return tool.Exec(args)
}

// GetDefinitions 获取所有工具定义
func (r *Registry) GetDefinitions() []models.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]models.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition)
	}
	return defs
}

// GetNames 获取所有工具名称
func (r *Registry) GetNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Size 工具数量
func (r *Registry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// DefaultRegistry 全局工具注册中心
var DefaultRegistry = NewRegistry()
