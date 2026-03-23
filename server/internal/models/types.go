package models

import "encoding/json"

// MessageRole 消息角色
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// ContentPart 多模态内容片段（兼容 OpenAI vision API）
type ContentPart struct {
	Type     string    `json:"type"`               // "text" | "image_url"
	Text     string    `json:"text,omitempty"`      // type=text 时
	ImageURL *ImageURL `json:"image_url,omitempty"` // type=image_url 时
}

// ImageURL 图片URL
type ImageURL struct {
	URL    string `json:"url"`              // 图片URL或base64 data URI
	Detail string `json:"detail,omitempty"` // "auto" | "low" | "high"
}

// MessageContent 消息内容，支持纯文本（string）和多模态（[]ContentPart）
// OpenAI API 的 content 字段可以是 string 或 array
type MessageContent struct {
	Text  string        // 纯文本内容
	Parts []ContentPart // 多模态内容
}

// IsMultimodal 是否包含多模态内容
func (mc *MessageContent) IsMultimodal() bool {
	return len(mc.Parts) > 0
}

// GetText 获取纯文本内容（如果是多模态则拼接所有文本片段）
func (mc *MessageContent) GetText() string {
	if !mc.IsMultimodal() {
		return mc.Text
	}
	var text string
	for _, p := range mc.Parts {
		if p.Type == "text" {
			text += p.Text
		}
	}
	return text
}

// MarshalJSON 自定义 JSON 序列化
// 纯文本 -> "content": "hello"
// 多模态 -> "content": [{"type":"text","text":"hello"}, {"type":"image_url",...}]
func (mc MessageContent) MarshalJSON() ([]byte, error) {
	if mc.IsMultimodal() {
		return json.Marshal(mc.Parts)
	}
	return json.Marshal(mc.Text)
}

// UnmarshalJSON 自定义 JSON 反序列化
func (mc *MessageContent) UnmarshalJSON(data []byte) error {
	// 先尝试作为字符串
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		mc.Text = text
		mc.Parts = nil
		return nil
	}
	// 再尝试作为数组
	var parts []ContentPart
	if err := json.Unmarshal(data, &parts); err == nil {
		mc.Parts = parts
		mc.Text = ""
		return nil
	}
	return nil
}

// NewTextContent 创建纯文本内容
func NewTextContent(text string) MessageContent {
	return MessageContent{Text: text}
}

// NewMultimodalContent 创建多模态内容
func NewMultimodalContent(parts []ContentPart) MessageContent {
	return MessageContent{Parts: parts}
}

// Message 消息结构
type Message struct {
	Role       MessageRole    `json:"role"`
	Content    MessageContent `json:"content"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
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
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  FunctionParams `json:"parameters"`
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

// Attachment 用户上传的附件信息
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
	URL      string `json:"url"`      // 服务端访问路径
	DataURI  string `json:"dataURI"`  // base64 data URI（图片用，直接传给 LLM）
}
