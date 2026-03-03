package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"

	"infringement-agent-server/internal/agent"
	"infringement-agent-server/internal/config"
	"infringement-agent-server/internal/evidence"
	"infringement-agent-server/internal/prompt"
	"infringement-agent-server/internal/providers"
	"infringement-agent-server/internal/tools"
)

// Session 会话
type Session struct {
	Agent *agent.Agent
	WS    *websocket.Conn
	mu    sync.Mutex
}

var (
	sessions      = make(map[string]*Session)
	sessionsMu    sync.RWMutex
	upgrader      = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	cfg           *config.Config
	toolRegistry  *tools.Registry
	promptMgr     *prompt.Manager
	evidenceStore *evidence.Store
)

func main() {
	// 加载 .env
	_ = godotenv.Load()
	cfg = config.Load()

	// 初始化工具注册中心
	toolRegistry = tools.NewRegistry()
	tools.RegisterBuiltinTools(toolRegistry)

	// 初始化提示词管理器
	promptMgr = prompt.NewManager("./prompts")

	// 初始化证据存储
	evidenceStore = evidence.NewStore("")
	if err := evidenceStore.Init(); err != nil {
		log.Printf("⚠️  证据存储初始化失败: %v", err)
	}

	// Gin 路由
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.ClientOrigin, "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// ==================== WebSocket ====================
	r.GET("/ws", handleWebSocket)

	// ==================== REST API ====================
	api := r.Group("/api")
	{
		api.GET("/health", handleHealth)
		api.GET("/providers", handleProviders)
		api.GET("/prompts", handlePrompts)
		api.GET("/tools", handleTools)

		// 会话管理
		api.POST("/session", handleCreateSession)
		api.POST("/chat", handleChat)
		api.POST("/reset", handleReset)
		api.DELETE("/session/:sessionId", handleDeleteSession)

		// 证据管理
		api.GET("/cases", handleListCases)
		api.GET("/cases/:caseId", handleGetCase)
		api.GET("/evidence/:caseId/:filename", handleGetEvidence)
	}

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("\n正在关闭服务...")
		sessionsMu.Lock()
		for _, s := range sessions {
			s.mu.Lock()
			if s.WS != nil {
				_ = s.WS.Close()
			}
			s.mu.Unlock()
		}
		sessionsMu.Unlock()
		os.Exit(0)
	}()

	toolNames := toolRegistry.GetNames()
	fmt.Printf(`
╔═══════════════════════════════════════════════════╗
║   ⚖️  网络侵权证据智能分析系统 - Go 后端服务       ║
╚═══════════════════════════════════════════════════╝

  API 服务:     http://localhost:%s
  WebSocket:    ws://localhost:%s/ws
  前端地址:     %s
  当前模型:     %s
  已注册工具:   %d 个
`, cfg.Port, cfg.Port, cfg.ClientOrigin, cfg.DefaultProvider, len(toolNames))

	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}

// pushToClient 向客户端推送 WebSocket 消息
func pushToClient(sessionID string, data interface{}) {
	sessionsMu.RLock()
	s, ok := sessions[sessionID]
	sessionsMu.RUnlock()
	if !ok {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.WS != nil {
		_ = s.WS.WriteJSON(data)
	}
}

// ==================== WebSocket Handler ====================

func handleWebSocket(c *gin.Context) {
	sessionID := c.Query("sessionId")

	sessionsMu.RLock()
	session, ok := sessions[sessionID]
	sessionsMu.RUnlock()

	if !ok || sessionID == "" {
		c.JSON(400, gin.H{"error": "无效的会话 ID"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	session.mu.Lock()
	session.WS = conn
	session.mu.Unlock()

	_ = conn.WriteJSON(gin.H{"type": "connected", "sessionId": sessionID})

	// 保持连接，读取消息直到关闭
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	session.mu.Lock()
	if session.WS == conn {
		session.WS = nil
	}
	session.mu.Unlock()
}

// ==================== REST Handlers ====================

func handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok", "timestamp": fmt.Sprintf("%d", time.Now().UnixMilli())})
}

func handleProviders(c *gin.Context) {
	c.JSON(200, cfg.ProviderList())
}

func handlePrompts(c *gin.Context) {
	c.JSON(200, promptMgr.ListTemplates())
}

func handleTools(c *gin.Context) {
	defs := toolRegistry.GetDefinitions()
	result := make([]gin.H, 0, len(defs))
	for _, d := range defs {
		result = append(result, gin.H{
			"name":        d.Function.Name,
			"description": d.Function.Description,
			"type":        "builtin",
			"parameters":  d.Function.Parameters,
		})
	}
	c.JSON(200, result)
}

func handleCreateSession(c *gin.Context) {
	var req struct {
		PromptTemplate string `json:"promptTemplate"`
		Provider       string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "invalid request"})
		return
	}

	providerType := providers.ProviderType(req.Provider)
	if providerType == "" {
		providerType = cfg.DefaultProvider
	}

	apiKey := cfg.GetAPIKey(providerType)
	provider, err := providers.NewProvider(providers.ProviderConfig{
		Type:   providerType,
		APIKey: apiKey,
	})
	if err != nil {
		c.JSON(400, gin.H{"success": false, "error": err.Error()})
		return
	}

	sessionID := uuid.New().String()

	agentCfg := agent.Config{
		MaxIterations:  cfg.MaxIterations,
		PromptTemplate: req.PromptTemplate,
		Verbose:        true,
		OnToolCall: func(event agent.ToolCallEvent) {
			pushToClient(sessionID, map[string]interface{}{
				"type":     "tool_call",
				"tool":     event.Tool,
				"args":     event.Args,
				"result":   event.Result,
				"success":  event.Success,
				"duration": event.Duration,
			})
		},
		OnThinking: func(step string) {
			pushToClient(sessionID, map[string]interface{}{
				"type": "thinking",
				"step": step,
			})
		},
	}
	if agentCfg.PromptTemplate == "" {
		agentCfg.PromptTemplate = "infringement-analyst"
	}

	a := agent.New(provider, toolRegistry, promptMgr, agentCfg)
	if err := a.Initialize(); err != nil {
		c.JSON(500, gin.H{"success": false, "error": err.Error()})
		return
	}

	sessionsMu.Lock()
	sessions[sessionID] = &Session{Agent: a}
	sessionsMu.Unlock()

	c.JSON(200, gin.H{
		"success":   true,
		"sessionId": sessionID,
		"provider":  provider.Name(),
		"toolCount": toolRegistry.Size(),
	})
}

func handleChat(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId"`
		Message   string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "invalid request"})
		return
	}

	sessionsMu.RLock()
	session, ok := sessions[req.SessionID]
	sessionsMu.RUnlock()

	if !ok {
		c.JSON(404, gin.H{"success": false, "error": "会话不存在"})
		return
	}

	pushToClient(req.SessionID, map[string]interface{}{
		"type": "status", "status": "thinking",
	})

	response, err := session.Agent.Chat(req.Message)
	if err != nil {
		pushToClient(req.SessionID, map[string]interface{}{
			"type": "status", "status": "error",
		})
		c.JSON(500, gin.H{"success": false, "error": err.Error()})
		return
	}

	pushToClient(req.SessionID, map[string]interface{}{
		"type": "status", "status": "done",
	})

	c.JSON(200, gin.H{
		"success":   true,
		"response":  response,
		"toolCalls": []interface{}{},
	})
}

func handleReset(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	_ = c.ShouldBindJSON(&req)

	sessionsMu.RLock()
	session, ok := sessions[req.SessionID]
	sessionsMu.RUnlock()

	if ok {
		session.Agent.Reset()
		_ = session.Agent.Initialize()
	}
	c.JSON(200, gin.H{"success": true})
}

func handleDeleteSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	sessionsMu.Lock()
	s, ok := sessions[sessionID]
	if ok {
		s.mu.Lock()
		if s.WS != nil {
			_ = s.WS.Close()
		}
		s.mu.Unlock()
		delete(sessions, sessionID)
	}
	sessionsMu.Unlock()

	c.JSON(200, gin.H{"success": true})
}

// ==================== 证据管理 ====================

func handleListCases(c *gin.Context) {
	c.JSON(200, evidenceStore.ListCases())
}

func handleGetCase(c *gin.Context) {
	caseID := c.Param("caseId")
	detail := evidenceStore.GetCase(caseID)
	if detail == nil {
		c.JSON(404, gin.H{"error": "案件不存在"})
		return
	}
	c.JSON(200, detail)
}

func handleGetEvidence(c *gin.Context) {
	caseID := c.Param("caseId")
	filename := c.Param("filename")
	filePath := evidenceStore.GetEvidenceFilePath(caseID, filename)
	if filePath == "" {
		c.JSON(404, gin.H{"error": "证据文件不存在"})
		return
	}
	c.File(filePath)
}

// ==================== Helpers ====================

func init() {
	// placeholder
}
