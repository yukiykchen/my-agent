package providers

import "infringement-agent-server/internal/models"

// Provider 模型供应商接口
type Provider interface {
	Name() string
	Chat(messages []models.Message, tools []models.ToolDefinition, config models.ModelConfig) (*models.ModelResponse, error)
}
