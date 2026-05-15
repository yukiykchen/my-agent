package agent

import (
	"strings"
	"testing"

	"infringement-agent-server/internal/models"
	"infringement-agent-server/internal/prompt"
	"infringement-agent-server/internal/tools"
)

type providerCall struct {
	messages []models.Message
	tools    []models.ToolDefinition
}

type fakeProvider struct {
	responses []*models.ModelResponse
	calls     []providerCall
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Chat(messages []models.Message, toolDefs []models.ToolDefinition, config models.ModelConfig) (*models.ModelResponse, error) {
	copiedMessages := append([]models.Message(nil), messages...)
	copiedTools := append([]models.ToolDefinition(nil), toolDefs...)
	f.calls = append(f.calls, providerCall{messages: copiedMessages, tools: copiedTools})

	if len(f.responses) == 0 {
		return &models.ModelResponse{Content: "", FinishReason: "stop"}, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func hasTool(toolDefs []models.ToolDefinition, name string) bool {
	for _, def := range toolDefs {
		if def.Function.Name == name {
			return true
		}
	}
	return false
}

func TestCoordinatorAdvertisesSubAgentDelegationTool(t *testing.T) {
	provider := &fakeProvider{responses: []*models.ModelResponse{{Content: "ok", FinishReason: "stop"}}}
	agent := New(provider, tools.NewRegistry(), prompt.NewManager(t.TempDir()), Config{MaxIterations: 6})

	if _, err := agent.Chat("请分析当前案件"); err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(provider.calls))
	}
	if !hasTool(provider.calls[0].tools, "delegate_to_subagent") {
		t.Fatalf("expected coordinator to expose delegate_to_subagent tool, got tools: %#v", provider.calls[0].tools)
	}
}

func TestDelegateToSubAgentInvokesSpecializedAgent(t *testing.T) {
	provider := &fakeProvider{responses: []*models.ModelResponse{
		{
			FinishReason: "tool_calls",
			ToolCalls: []models.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: models.FunctionCall{
					Name:      "delegate_to_subagent",
					Arguments: `{"agent":"legal-analyst","task":"分析侵权事实","context":"case_1"}`,
				},
			}},
		},
		{Content: "子 Agent 法律分析完成", FinishReason: "stop"},
		{Content: "协调完成", FinishReason: "stop"},
	}}
	agent := New(provider, tools.NewRegistry(), prompt.NewManager(t.TempDir()), Config{MaxIterations: 6})

	got, err := agent.Chat("请委派法律子 Agent 分析")
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if got != "协调完成" {
		t.Fatalf("expected final coordinator response, got %q", got)
	}
	if len(provider.calls) != 3 {
		t.Fatalf("expected coordinator call, sub-agent call, final coordinator call; got %d calls", len(provider.calls))
	}

	subAgentSystemPrompt := provider.calls[1].messages[0].Content.GetText()
	if !strings.Contains(subAgentSystemPrompt, "法律分析子 Agent") {
		t.Fatalf("expected sub-agent system prompt to contain legal role, got: %s", subAgentSystemPrompt)
	}
	subAgentUserMessage := provider.calls[1].messages[len(provider.calls[1].messages)-1].Content.GetText()
	if !strings.Contains(subAgentUserMessage, "分析侵权事实") || !strings.Contains(subAgentUserMessage, "case_1") {
		t.Fatalf("expected sub-agent user message to include task and context, got: %s", subAgentUserMessage)
	}

	finalCoordinatorContext := provider.calls[2].messages[len(provider.calls[2].messages)-1].Content.GetText()
	if !strings.Contains(finalCoordinatorContext, "子 Agent 法律分析完成") {
		t.Fatalf("expected coordinator to receive sub-agent result as tool context, got: %s", finalCoordinatorContext)
	}
}
