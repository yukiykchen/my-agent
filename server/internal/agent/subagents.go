package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"infringement-agent-server/internal/models"
)

const subAgentDelegateToolName = "delegate_to_subagent"

// SubAgentSpec 子 Agent 角色定义。
type SubAgentSpec struct {
	Name        string
	Description string
	RolePrompt  string
}

// DefaultSubAgents 返回网络侵权取证场景下的默认子 Agent 团队。
func DefaultSubAgents() []SubAgentSpec {
	return []SubAgentSpec{
		{
			Name:        "evidence-analyst",
			Description: "证据链分析子 Agent，负责证据清单、四值哈希链、TSA、上链与监督链校验。",
			RolePrompt:  "你是证据链分析子 Agent。重点核验证据完整性、哈希一致性、时间戳、区块链摘要存证和 Chain of Custody，不进行超出证据材料的事实推断。",
		},
		{
			Name:        "legal-analyst",
			Description: "法律分析子 Agent，负责商标、著作权、反不正当竞争与虚假宣传风险分析。",
			RolePrompt:  "你是法律分析子 Agent。请围绕法律依据、事实对应和结论判断进行分析，优先使用法律三段论结构，避免直接替代律师或司法机关作最终裁判。",
		},
		{
			Name:        "web-capture-analyst",
			Description: "静态网页取证子 Agent，负责网页截图、HTML、OCR、素材相似度和页面侵权证据组织。",
			RolePrompt:  "你是静态网页取证子 Agent。重点关注页面快照、HTML 存档、页面元数据、商品标题、详情图、OCR 结果和素材相似度证据。",
		},
		{
			Name:        "live-capture-analyst",
			Description: "直播取证子 Agent，负责直播分段、ASR、OCR、关键帧、多模态时间轴与重点证据定位。",
			RolePrompt:  "你是直播取证子 Agent。重点关注视频分段、语音转写、画面识别、关键帧、多模态时间轴和直播内容易逝性的证据固化问题。",
		},
	}
}

func (a *Agent) configuredSubAgents() []SubAgentSpec {
	if len(a.config.SubAgents) > 0 {
		return a.config.SubAgents
	}
	return DefaultSubAgents()
}

func (a *Agent) subAgentCoordinationPrompt() string {
	var b strings.Builder
	b.WriteString("\n\n## 多 Agent 协作机制\n")
	b.WriteString("你是协调 Agent。遇到复杂网络侵权取证任务时，应根据任务类型将子任务委派给专门的子 Agent，并综合其返回结果形成最终答复。可用子 Agent：\n")
	for _, spec := range a.configuredSubAgents() {
		b.WriteString("- ")
		b.WriteString(spec.Name)
		b.WriteString("：")
		b.WriteString(spec.Description)
		b.WriteString("\n")
	}
	b.WriteString("如需委派，请调用 delegate_to_subagent，并在最终回复中整合不同子 Agent 的结论。\n")
	return b.String()
}

func (a *Agent) registerSubAgentDelegationTool() {
	specs := a.configuredSubAgents()
	enum := make([]string, 0, len(specs))
	for _, spec := range specs {
		enum = append(enum, spec.Name)
	}

	a.toolRegistry.Register(
		subAgentDelegateToolName,
		"将一个明确的子任务委派给指定子 Agent 执行，并返回该子 Agent 的分析结果。适用于证据链校验、法律分析、网页取证分析、直播取证分析等专业子任务。",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"agent":   {Type: "string", Description: "子 Agent 名称", Enum: enum},
				"task":    {Type: "string", Description: "需要子 Agent 完成的明确任务"},
				"context": {Type: "string", Description: "可选，上下文信息，例如案件 ID、已知证据编号、用户目标或前置分析结果"},
			},
			Required: []string{"agent", "task"},
		},
		func(args map[string]interface{}) (string, error) {
			agentName, _ := args["agent"].(string)
			task, _ := args["task"].(string)
			contextText, _ := args["context"].(string)
			if agentName == "" {
				return "", fmt.Errorf("agent is required")
			}
			if strings.TrimSpace(task) == "" {
				return "", fmt.Errorf("task is required")
			}

			var selected *SubAgentSpec
			for i := range specs {
				if specs[i].Name == agentName {
					selected = &specs[i]
					break
				}
			}
			if selected == nil {
				return "", fmt.Errorf("unknown sub agent: %s", agentName)
			}

			maxIterations := a.config.SubAgentMaxIterations
			if maxIterations <= 0 {
				maxIterations = 6
			}

			subCfg := Config{
				MaxIterations:    maxIterations,
				PromptTemplate:   a.config.PromptTemplate,
				Verbose:          a.config.Verbose,
				RoleName:         selected.Name,
				RolePrompt:       selected.RolePrompt,
				DisableSubAgents: true,
				OnThinking: func(step string) {
					if a.config.OnThinking != nil {
						a.config.OnThinking(fmt.Sprintf("子 Agent[%s]：%s", selected.Name, step))
					}
				},
				OnToolCall: func(event ToolCallEvent) {
					if a.config.OnToolCall != nil {
						event.Tool = selected.Name + ":" + event.Tool
						a.config.OnToolCall(event)
					}
				},
			}

			subAgent := New(a.provider, a.toolRegistry, a.promptMgr, subCfg)
			message := "任务：" + task
			if strings.TrimSpace(contextText) != "" {
				message += "\n\n上下文：" + contextText
			}
			response, err := subAgent.Chat(message)
			if err != nil {
				return "", err
			}

			data, _ := json.MarshalIndent(map[string]interface{}{
				"agent":       selected.Name,
				"description": selected.Description,
				"task":        task,
				"context":     contextText,
				"response":    response,
			}, "", "  ")
			return string(data), nil
		},
	)
}
