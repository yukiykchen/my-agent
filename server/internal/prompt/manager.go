package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultSystemPrompt 默认系统提示词
const DefaultSystemPrompt = `你是一个强大的AI编程助手。你的任务是帮助用户完成各种编程任务。

## 核心能力
- 理解和分析代码
- 编写高质量的代码
- 调试和修复问题
- 解释技术概念

## 工作原则
1. **准确性**：确保代码正确、可运行
2. **简洁性**：代码清晰易读，避免冗余
3. **安全性**：注意安全最佳实践
4. **效率性**：考虑性能和资源使用

## 交互方式
- 仔细阅读用户的问题
- 如果需要更多信息，主动询问
- 使用工具来完成实际操作
- 清晰地解释你的思考过程`

// Manager 提示词管理器
type Manager struct {
	promptsDir string
	cache      map[string]string
	mu         sync.RWMutex
}

// NewManager 创建提示词管理器
func NewManager(promptsDir string) *Manager {
	return &Manager{
		promptsDir: promptsDir,
		cache:      make(map[string]string),
	}
}

// GetContent 获取提示词内容
func (m *Manager) GetContent(name string) (string, error) {
	m.mu.RLock()
	if content, ok := m.cache[name]; ok {
		m.mu.RUnlock()
		return content, nil
	}
	m.mu.RUnlock()

	filePath := filepath.Join(m.promptsDir, name+".md")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	content := string(data)
	// 简单去除 frontmatter (--- ... ---)
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			content = strings.TrimSpace(parts[2])
		}
	}

	m.mu.Lock()
	m.cache[name] = content
	m.mu.Unlock()

	return content, nil
}

// ListTemplates 列出所有可用模板
func (m *Manager) ListTemplates() []TemplateInfo {
	// 硬编码模板列表（与 TS 版本一致）
	return []TemplateInfo{
		{ID: "infringement-analyst", Name: "⚖️ 侵权分析专家", Description: "网络侵权证据分析与法律推理"},
		{ID: "gemini-cli", Name: "🖥️ Gemini CLI 风格", Description: "专业的编程助手"},
		{ID: "coding-mentor", Name: "👨‍🏫 编程导师", Description: "耐心的教学风格"},
		{ID: "strict-engineer", Name: "👔 严格工程师", Description: "注重代码质量"},
		{ID: "personal-assistant", Name: "😊 个人助手", Description: "友好的助手"},
		{ID: "sarcastic-friend", Name: "😏 毒舌朋友", Description: "幽默的风格"},
		{ID: "anime-girl", Name: "🌸 二次元少女", Description: "可爱的助手"},
	}
}

// TemplateInfo 模板信息
type TemplateInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
