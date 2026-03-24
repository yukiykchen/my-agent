package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"infringement-agent-server/internal/models"
)

// OpenAICompatibleProvider OpenAI 兼容接口适配器
type OpenAICompatibleProvider struct {
	name         string
	apiKey       string
	baseURL      string
	defaultModel string
	client       *http.Client
}

// NewOpenAICompatible 创建 OpenAI 兼容的 Provider
func NewOpenAICompatible(name, apiKey, baseURL, defaultModel string) *OpenAICompatibleProvider {
	return &OpenAICompatibleProvider{
		name:         name,
		apiKey:       apiKey,
		baseURL:      strings.TrimRight(baseURL, "/"),
		defaultModel: defaultModel,
		client:       &http.Client{},
	}
}

func (p *OpenAICompatibleProvider) Name() string {
	return p.name
}

// chatRequest OpenAI 请求体
type chatRequest struct {
	Model       string                  `json:"model"`
	Messages    []chatMessageForAPI     `json:"messages"`
	Tools       []models.ToolDefinition `json:"tools,omitempty"`
	Temperature float64                 `json:"temperature"`
	MaxTokens   int                     `json:"max_tokens"`
}

// chatMessageForAPI 发给 API 的消息格式
// Content 需要支持 string 和 array 两种序列化
type chatMessageForAPI struct {
	Role             models.MessageRole `json:"role"`
	Content          json.RawMessage    `json:"content"`
	ReasoningContent *string            `json:"reasoning_content,omitempty"` // Kimi K2.5 thinking 推理内容
	Name             string             `json:"name,omitempty"`
	ToolCalls        []models.ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID       string             `json:"tool_call_id,omitempty"`
}

// convertMessagesToAPI 将内部 Message 转为 API 请求格式
func convertMessagesToAPI(messages []models.Message) []chatMessageForAPI {
	result := make([]chatMessageForAPI, 0, len(messages))
	for _, msg := range messages {
		apiMsg := chatMessageForAPI{
			Role:       msg.Role,
			Name:       msg.Name,
			ToolCalls:  msg.ToolCalls,
			ToolCallID: msg.ToolCallID,
		}

		// 传递 reasoning_content（Kimi K2.5 thinking 模式）
		if msg.ReasoningContent != "" {
			rc := msg.ReasoningContent
			apiMsg.ReasoningContent = &rc
		}

		// 序列化 Content（string 或 array）
		contentBytes, _ := json.Marshal(msg.Content)
		apiMsg.Content = contentBytes

		result = append(result, apiMsg)
	}
	return result
}

// chatResponseChoice OpenAI 响应选项
type chatResponseChoice struct {
	Message      choiceMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type choiceMessage struct {
	Content          *string           `json:"content"`
	ReasoningContent *string           `json:"reasoning_content,omitempty"` // Kimi K2.5 thinking 推理内容
	ToolCalls        []models.ToolCall `json:"tool_calls,omitempty"`
}

type chatResponse struct {
	Choices []chatResponseChoice `json:"choices"`
	Usage   *models.Usage        `json:"usage,omitempty"`
	Error   *apiError            `json:"error,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
}

func (p *OpenAICompatibleProvider) Chat(messages []models.Message, tools []models.ToolDefinition, config models.ModelConfig) (*models.ModelResponse, error) {
	model := config.Model
	if model == "" {
		model = p.defaultModel
	}

	reqBody := chatRequest{
		Model:       model,
		Messages:    convertMessagesToAPI(messages),
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s API request failed: %w", p.name, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp chatResponse
		_ = json.Unmarshal(respBody, &errResp)
		msg := resp.Status
		if errResp.Error != nil {
			msg = errResp.Error.Message
		}
		return nil, fmt.Errorf("%s API error: %s", p.name, msg)
	}

	var result chatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("%s: no choices in response", p.name)
	}

	choice := result.Choices[0]
	content := ""
	if choice.Message.Content != nil {
		content = *choice.Message.Content
	}
	reasoningContent := ""
	if choice.Message.ReasoningContent != nil {
		reasoningContent = *choice.Message.ReasoningContent
	}

	finishReason := "stop"
	if choice.FinishReason == "tool_calls" {
		finishReason = "tool_calls"
	}

	return &models.ModelResponse{
		Content:          content,
		ReasoningContent: reasoningContent,
		ToolCalls:        choice.Message.ToolCalls,
		FinishReason:     finishReason,
		Usage:            result.Usage,
	}, nil
}
