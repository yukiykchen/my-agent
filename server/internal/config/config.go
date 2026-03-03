package config

import (
	"os"
	"strconv"

	"infringement-agent-server/internal/providers"
)

// Config 应用配置
type Config struct {
	Port            string
	ClientOrigin    string
	DefaultProvider providers.ProviderType
	MaxIterations   int

	MoonshotAPIKey string
	DeepSeekAPIKey string
	ZhipuAPIKey    string
	GeminiAPIKey   string
	OpenAIAPIKey   string
	OpenAIBaseURL  string
}

// Load 从环境变量加载配置
func Load() *Config {
	cfg := &Config{
		Port:            getEnv("PORT", "3001"),
		ClientOrigin:    getEnv("CLIENT_ORIGIN", "http://localhost:5173"),
		DefaultProvider: providers.ProviderType(getEnv("DEFAULT_PROVIDER", "moonshot")),
		MaxIterations:   getEnvInt("MAX_ITERATIONS", 20),

		MoonshotAPIKey: os.Getenv("MOONSHOT_API_KEY"),
		DeepSeekAPIKey: os.Getenv("DEEPSEEK_API_KEY"),
		ZhipuAPIKey:    os.Getenv("ZHIPU_API_KEY"),
		GeminiAPIKey:   os.Getenv("GEMINI_API_KEY"),
		OpenAIAPIKey:   os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:  getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
	}
	return cfg
}

// GetAPIKey 根据供应商类型获取 API Key
func (c *Config) GetAPIKey(pt providers.ProviderType) string {
	switch pt {
	case providers.Moonshot:
		return c.MoonshotAPIKey
	case providers.DeepSeek:
		return c.DeepSeekAPIKey
	case providers.Zhipu:
		return c.ZhipuAPIKey
	case providers.Gemini:
		return c.GeminiAPIKey
	case providers.OpenAI:
		return c.OpenAIAPIKey
	}
	return ""
}

// ProviderList 获取所有供应商及可用状态
func (c *Config) ProviderList() []ProviderInfo {
	return []ProviderInfo{
		{ID: "moonshot", Name: "Kimi", Available: c.MoonshotAPIKey != ""},
		{ID: "deepseek", Name: "DeepSeek", Available: c.DeepSeekAPIKey != ""},
		{ID: "zhipu", Name: "智谱 AI", Available: c.ZhipuAPIKey != ""},
		{ID: "gemini", Name: "Gemini", Available: c.GeminiAPIKey != ""},
		{ID: "openai", Name: "OpenAI", Available: c.OpenAIAPIKey != ""},
	}
}

// ProviderInfo 供应商信息
type ProviderInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
