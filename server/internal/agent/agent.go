package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"infringement-agent-server/internal/models"
	"infringement-agent-server/internal/prompt"
	"infringement-agent-server/internal/providers"
	"infringement-agent-server/internal/tools"
)

// ToolCallEvent 工具调用事件
type ToolCallEvent struct {
	Tool     string                 `json:"tool"`
	Args     map[string]interface{} `json:"args"`
	Result   string                 `json:"result,omitempty"`
	Success  bool                   `json:"success,omitempty"`
	Duration int64                  `json:"duration,omitempty"` // ms
}

// Config Agent 配置
type Config struct {
	MaxIterations  int
	PromptTemplate string
	Verbose        bool
	OnToolCall     func(ToolCallEvent)
	OnThinking     func(step string)
}

// Agent 智能体
type Agent struct {
	provider      providers.Provider
	providerModel string
	toolRegistry  *tools.Registry
	promptMgr     *prompt.Manager
	config        Config
	messages      []models.Message
	iterations    int
	isRunning     bool
}

// New 创建 Agent
func New(provider providers.Provider, registry *tools.Registry, promptMgr *prompt.Manager, cfg Config) *Agent {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 20
	}
	return &Agent{
		provider:     provider,
		toolRegistry: registry,
		promptMgr:    promptMgr,
		config:       cfg,
		messages:     make([]models.Message, 0),
	}
}

// Initialize 初始化 Agent
func (a *Agent) Initialize() error {
	systemPrompt := prompt.DefaultSystemPrompt

	if a.config.PromptTemplate != "" {
		if content, err := a.promptMgr.GetContent(a.config.PromptTemplate); err == nil {
			systemPrompt = content
		}
	}

	// 附加可用工具列表
	toolNames := a.toolRegistry.GetNames()
	if len(toolNames) > 0 {
		systemPrompt += "\n\n## 可用工具\n" + strings.Join(toolNames, ", ")
	}

	a.messages = []models.Message{
		{Role: models.RoleSystem, Content: models.NewTextContent(systemPrompt)},
	}
	return nil
}

// Chat 处理用户输入（ReAct 循环）
func (a *Agent) Chat(userMessage string) (string, error) {
	return a.ChatWithAttachments(userMessage, nil)
}

// ChatWithAttachments 处理带附件的用户输入（多模态）
func (a *Agent) ChatWithAttachments(userMessage string, attachments []models.Attachment) (string, error) {
	if len(a.messages) == 0 {
		if err := a.Initialize(); err != nil {
			return "", err
		}
	}

	// 构造用户消息
	var userContent models.MessageContent
	if len(attachments) > 0 {
		// 多模态消息：文本 + 图片
		parts := []models.ContentPart{
			{Type: "text", Text: userMessage},
		}
		for _, att := range attachments {
			if att.DataURI != "" {
				parts = append(parts, models.ContentPart{
					Type: "image_url",
					ImageURL: &models.ImageURL{
						URL:    att.DataURI,
						Detail: "auto",
					},
				})
			}
		}
		userContent = models.NewMultimodalContent(parts)
	} else {
		userContent = models.NewTextContent(userMessage)
	}

	a.messages = append(a.messages, models.Message{
		Role:    models.RoleUser,
		Content: userContent,
	})

	a.isRunning = true
	a.iterations = 0

	toolDefs := a.toolRegistry.GetDefinitions()
	modelCfg := models.ModelConfig{
		Temperature: 0.7,
		MaxTokens:   4096,
	}

	var finalResponse string

	// 预留最后 1 轮用于阶段性总结（至少需要 3 轮才启用该机制）
	summaryThreshold := a.config.MaxIterations - 1
	if summaryThreshold < 2 {
		summaryThreshold = a.config.MaxIterations
	}

	for a.isRunning && a.iterations < a.config.MaxIterations {
		a.iterations++

		// 检测是否即将达到上限，若是则请求 LLM 生成阶段性总结
		if a.iterations >= summaryThreshold && a.isRunning {
			if a.config.OnThinking != nil {
				a.config.OnThinking("即将达到推理上限，正在生成阶段性总结...")
			}
			if a.config.Verbose {
				log.Printf("⚠️ 第 %d 轮（已达阈值 %d），请求阶段性总结", a.iterations, summaryThreshold)
			}

			summaryResp, err := a.requestSummary(modelCfg)
			if err != nil {
				log.Printf("❌ 生成总结失败: %v", err)
				finalResponse = "⚠️ 已达到最大推理轮次，总结生成失败。请尝试将任务拆分为更小的步骤后重新提问。"
			} else {
				finalResponse = summaryResp
			}
			a.messages = append(a.messages, models.Message{
				Role:    models.RoleAssistant,
				Content: models.NewTextContent(finalResponse),
			})
			a.isRunning = false
			break
		}

		if a.config.OnThinking != nil {
			a.config.OnThinking(fmt.Sprintf("第 %d/%d 轮推理...", a.iterations, a.config.MaxIterations))
		}

		if a.config.Verbose {
			log.Printf("🤔 第 %d/%d 轮推理...", a.iterations, a.config.MaxIterations)
		}

		resp, err := a.provider.Chat(a.messages, toolDefs, modelCfg)
		if err != nil {
			log.Printf("❌ 模型调用错误: %v", err)
			finalResponse = fmt.Sprintf("发生错误: %v", err)
			a.isRunning = false
			break
		}

		if len(resp.ToolCalls) > 0 {
			a.handleToolCalls(resp.ToolCalls, resp.Content)
		} else {
			finalResponse = resp.Content
			a.messages = append(a.messages, models.Message{
				Role:    models.RoleAssistant,
				Content: models.NewTextContent(finalResponse),
			})
			a.isRunning = false
		}
	}

	// 兜底：理论上不应走到这里，但以防万一
	if a.iterations >= a.config.MaxIterations && a.isRunning {
		finalResponse = "⚠️ 已达到最大推理轮次，任务可能未完全完成。您可以继续发送消息让我从当前进度继续。"
		a.isRunning = false
	}

	return finalResponse, nil
}

// requestSummary 请求 LLM 生成阶段性总结（不允许工具调用）
func (a *Agent) requestSummary(modelCfg models.ModelConfig) (string, error) {
	// 注入总结指令
	summaryInstruction := models.Message{
		Role: models.RoleUser,
		Content: models.NewTextContent(
			"【系统提示】你已经接近最大推理轮次限制，请不要再调用任何工具，直接生成一份阶段性总结回复给用户。\n\n" +
				"请按以下格式总结：\n" +
				"## ✅ 已完成的工作\n（列出你目前已经完成的步骤和获取到的结果）\n\n" +
				"## ⏳ 尚未完成的部分\n（列出还没来得及完成的任务）\n\n" +
				"## 💡 建议\n（告诉用户接下来可以怎么做，比如可以继续发消息让你从当前进度继续，或者把任务拆分）\n\n" +
				"请用中文回复，语气友好专业。",
		),
	}

	// 临时追加到消息列表
	messagesWithSummary := make([]models.Message, len(a.messages))
	copy(messagesWithSummary, a.messages)
	messagesWithSummary = append(messagesWithSummary, summaryInstruction)

	// 不传工具定义，强制 LLM 只能生成文本回复
	resp, err := a.provider.Chat(messagesWithSummary, nil, modelCfg)
	if err != nil {
		return "", err
	}

	// 将总结指令也追加到正式消息列表（保留上下文连贯性）
	a.messages = append(a.messages, summaryInstruction)

	return resp.Content, nil
}

// handleToolCalls 处理工具调用
func (a *Agent) handleToolCalls(toolCalls []models.ToolCall, content string) {
	a.messages = append(a.messages, models.Message{
		Role:      models.RoleAssistant,
		Content:   models.NewTextContent(content),
		ToolCalls: toolCalls,
	})

	for _, tc := range toolCalls {
		name := tc.Function.Name
		argsStr := tc.Function.Arguments

		if a.config.Verbose {
			log.Printf("🔧 调用工具: %s", name)
		}

		var args map[string]interface{}
		if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
			args = make(map[string]interface{})
		}

		// 通知前端（开始调用）
		if a.config.OnToolCall != nil {
			a.config.OnToolCall(ToolCallEvent{Tool: name, Args: args})
		}

		startTime := time.Now()
		result, err := a.toolRegistry.Execute(name, args)
		duration := time.Since(startTime).Milliseconds()

		success := true
		if err != nil {
			success = false
			result = fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
			if a.config.Verbose {
				log.Printf("   ❌ 错误: %v", err)
			}
		} else if a.config.Verbose {
			preview := result
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			log.Printf("   ✅ 结果: %s", preview)
		}

		// 通知前端（调用完成）
		if a.config.OnToolCall != nil {
			a.config.OnToolCall(ToolCallEvent{
				Tool:     name,
				Args:     map[string]interface{}{},
				Result:   result,
				Success:  success,
				Duration: duration,
			})
		}

		a.messages = append(a.messages, models.Message{
			Role:       models.RoleTool,
			ToolCallID: tc.ID,
			Content:    models.NewTextContent(result),
		})
	}
}

// Reset 重置对话
func (a *Agent) Reset() {
	a.messages = nil
	a.iterations = 0
	a.isRunning = false
}

// GetHistory 获取对话历史
func (a *Agent) GetHistory() []models.Message {
	cp := make([]models.Message, len(a.messages))
	copy(cp, a.messages)
	return cp
}
