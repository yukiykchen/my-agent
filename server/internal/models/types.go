package models

// MessageRole 消息角色
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// Message 消息结构
type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用详情
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Type     string         `json:"type"`
	Function FunctionDefine `json:"function"`
}

// FunctionDefine 函数定义
type FunctionDefine struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Parameters  FunctionParams   `json:"parameters"`
}

// FunctionParams 函数参数定义
type FunctionParams struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertyDefine `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// PropertyDefine 属性定义
type PropertyDefine struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// ModelResponse 模型响应
type ModelResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	Usage        *Usage     `json:"usage,omitempty"`
}

// Usage token 用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}
