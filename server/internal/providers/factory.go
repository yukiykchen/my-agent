package providers

import "fmt"

// ProviderType 供应商类型
type ProviderType string

const (
	Moonshot ProviderType = "moonshot"
	DeepSeek ProviderType = "deepseek"
	Zhipu    ProviderType = "zhipu"
	Gemini   ProviderType = "gemini"
	OpenAI   ProviderType = "openai"
)

// ProviderConfig 供应商配置
type ProviderConfig struct {
	Type    ProviderType
	APIKey  string
	BaseURL string // 仅 OpenAI 使用
}

// DefaultModels 各供应商默认模型
var DefaultModels = map[ProviderType]string{
	Moonshot: "kimi-k2.5",
	DeepSeek: "deepseek-chat",
	Zhipu:    "glm-4",
	Gemini:   "gemini-1.5-flash",
	OpenAI:   "gpt-4o",
}

// providerEndpoints 各供应商 API 地址
var providerEndpoints = map[ProviderType]string{
	Moonshot: "https://api.moonshot.cn/v1",
	DeepSeek: "https://api.deepseek.com/v1",
	Zhipu:    "https://open.bigmodel.cn/api/paas/v4",
	Gemini:   "https://generativelanguage.googleapis.com/v1beta/openai",
	OpenAI:   "https://api.openai.com/v1",
}

// providerNames 供应商显示名称
var providerNames = map[ProviderType]string{
	Moonshot: "Moonshot",
	DeepSeek: "DeepSeek",
	Zhipu:    "智谱 AI",
	Gemini:   "Gemini",
	OpenAI:   "OpenAI",
}

// NewProvider 创建 Provider
func NewProvider(cfg ProviderConfig) (Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s API key not set", cfg.Type)
	}

	baseURL := providerEndpoints[cfg.Type]
	if cfg.Type == OpenAI && cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}

	name := providerNames[cfg.Type]
	if name == "" {
		name = string(cfg.Type)
	}

	defaultModel := DefaultModels[cfg.Type]
	if defaultModel == "" {
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}

	return NewOpenAICompatible(name, cfg.APIKey, baseURL, defaultModel), nil
}
