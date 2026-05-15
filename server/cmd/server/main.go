package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	gopdf "github.com/ledongthuc/pdf"
	extmcp "github.com/mark3labs/mcp-go/mcp"

	"infringement-agent-server/internal/agent"
	"infringement-agent-server/internal/config"
	"infringement-agent-server/internal/evidence"
	"infringement-agent-server/internal/mcp"
	"infringement-agent-server/internal/models"
	"infringement-agent-server/internal/notary"
	"infringement-agent-server/internal/prompt"
	"infringement-agent-server/internal/providers"
	"infringement-agent-server/internal/report"
	"infringement-agent-server/internal/tools"
)

// Session 会话
type Session struct {
	Agent *agent.Agent
	WS    *websocket.Conn
	mu    sync.Mutex
}

var (
	sessions   = make(map[string]*Session)
	sessionsMu sync.RWMutex
	upgrader   = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	cfg           *config.Config
	toolRegistry  *tools.Registry
	promptMgr     *prompt.Manager
	evidenceStore *evidence.Store
	notarizeSvc   *evidence.NotarizeService
	chainAnchor   *evidence.ChainAnchor
	mcpClient     *mcp.Client
	mcpBridge     *mcp.Bridge
	reportGen     *report.Generator
)

func main() {
	// 加载 .env
	_ = godotenv.Load()
	cfg = config.Load()

	// 初始化工具注册中心
	toolRegistry = tools.NewRegistry()
	tools.RegisterBuiltinTools(toolRegistry)

	// 初始化 MCP 客户端
	mcpClient = mcp.NewClient()
	if err := mcpClient.LoadConfig(".mcp.json"); err != nil {
		log.Printf("⚠️  加载 MCP 配置失败: %v", err)
	}
	if mcpClient.HasServers() {
		fmt.Println("  ℹ  发现 MCP 服务器配置，正在连接...")
		if err := mcpClient.ConnectAll(); err != nil {
			log.Printf("⚠️  MCP 连接失败: %v", err)
		}
		mcpBridge = mcp.NewBridge(mcpClient, toolRegistry)
		if count, err := mcpBridge.RegisterAll(); err != nil {
			log.Printf("⚠️  MCP 工具注册失败: %v", err)
		} else {
			fmt.Printf("  ✅ MCP 工具注册完成，共 %d 个工具\n", count)
		}
	}

	// 初始化提示词管理器
	promptMgr = prompt.NewManager("./prompts")

	// 初始化截图存储目录
	screenshotDir := "./data/screenshots"
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		log.Printf("⚠️  截图目录创建失败: %v", err)
	}

	// 初始化上传文件存储目录
	uploadDir := "./data/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("⚠️  上传目录创建失败: %v", err)
	}

	// 初始化证据存储
	evidenceStore = evidence.NewStore("")
	if err := evidenceStore.Init(); err != nil {
		log.Printf("⚠️  证据存储初始化失败: %v", err)
	}

	// 初始化证据即时固化服务（TSA + fsnotify + 监督链）
	notarizeServiceInstance, notarizeErr := evidence.NewNotarizeService(evidenceStore)
	if notarizeErr != nil {
		log.Printf("⚠️  固化服务初始化失败: %v", notarizeErr)
	} else {
		notarizeSvc = notarizeServiceInstance
		fmt.Println("  ✅ 证据即时固化服务已启动（Mock TSA + fsnotify + 监督链）")

		// 初始化 Mock 区块链锚定服务
		anchorInstance, anchorErr := evidence.NewChainAnchor(notarizeSvc)
		if anchorErr != nil {
			log.Printf("⚠️  Mock 区块链服务初始化失败: %v", anchorErr)
		} else {
			chainAnchor = anchorInstance
			fmt.Println("  ✅ Mock 蚂蚁链存证服务已启动（Merkle Tree 批量聚合）")
		}

		// 注册证据链条相关的 Agent 工具
		tools.RegisterEvidenceTools(toolRegistry, notarizeSvc, chainAnchor)

		// 注册法律规范化报告生成工具
		reportGen = report.NewGenerator(notarizeSvc, chainAnchor)
		tools.RegisterReportTool(toolRegistry, reportGen)
		fmt.Println("  ✅ 证据链条工具集已注册（evidence_list / evidence_verify / evidence_anchor / custody_list / asr_transcribe / ocr_recognize / report_generate）")
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
		api.GET("/mcp/status", handleMCPStatus)

		// 会话管理
		api.POST("/session", handleCreateSession)
		api.POST("/chat", handleChat)
		api.POST("/reset", handleReset)
		api.DELETE("/session/:sessionId", handleDeleteSession)

		// 证据管理
		api.GET("/cases", handleListCases)
		api.GET("/cases/:caseId", handleGetCase)
		api.GET("/evidence/:caseId/:filename", handleGetEvidence)

		// 证据即时固化（论文 4.2 节核心）
		api.POST("/notarize/fixate", handleNotarizeFixate)
		api.GET("/notarize/:caseId", handleListNotarized)
		api.GET("/notarize/:caseId/:evidenceId", handleGetNotarized)
		api.GET("/notarize/:caseId/:evidenceId/file", handleGetNotarizedFile)
		api.POST("/notarize/:caseId/:evidenceId/verify", handleVerifyNotarized)

		// 监督链（Chain of Custody）
		api.GET("/custody/:caseId", handleListCustody)
		api.GET("/custody/:caseId/verify", handleVerifyCustody)

		// 区块链锚定（论文 4.3.3 Merkle Tree 批量上链）
		api.POST("/chain/:caseId/anchor", handleChainAnchor)
		api.GET("/chain/:caseId/:evidenceId/verify", handleChainVerify)
		api.GET("/chain/stats", handleChainStats)

		// 网页取证（直接驱动 MCP 工具 + 自动固化，不依赖 Agent 自主决策）
		api.POST("/evidence/screenshot", handleWebScreenshot)
		api.POST("/evidence/crawl", handleWebCrawl)

		// 报告管理（论文 4.5 节七段式报告）
		api.GET("/reports/:caseId", handleListReports)
		api.GET("/reports/:caseId/:reportId", handleGetReport)

		// 截图静态文件服务
		api.GET("/screenshots/:filename", handleScreenshot)

		// 文件上传
		api.POST("/upload", handleUpload)
		api.GET("/uploads/:filename", handleUploadedFile)
	}

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("\n正在关闭服务...")
		// 关闭 WebSocket 连接
		sessionsMu.Lock()
		for _, s := range sessions {
			s.mu.Lock()
			if s.WS != nil {
				_ = s.WS.Close()
			}
			s.mu.Unlock()
		}
		sessionsMu.Unlock()
		// 断开 MCP 服务器
		if mcpClient != nil {
			mcpClient.DisconnectAll()
		}
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
		toolType := "builtin"
		if len(d.Function.Name) > 4 && d.Function.Name[:4] == "mcp_" {
			toolType = "mcp"
		}
		result = append(result, gin.H{
			"name":        d.Function.Name,
			"description": d.Function.Description,
			"type":        toolType,
			"parameters":  d.Function.Parameters,
		})
	}
	c.JSON(200, result)
}

func handleMCPStatus(c *gin.Context) {
	if mcpClient == nil {
		c.JSON(200, gin.H{"servers": []interface{}{}, "tools": []interface{}{}})
		return
	}
	status := mcpClient.GetServerStatus()
	tools := mcpClient.GetTools()
	c.JSON(200, gin.H{
		"servers": status,
		"tools":   tools,
	})
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
		SessionID   string              `json:"sessionId"`
		Message     string              `json:"message"`
		Attachments []models.Attachment `json:"attachments,omitempty"`
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

	response, err := session.Agent.ChatWithAttachments(req.Message, req.Attachments)
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

// ==================== 证据即时固化（论文 4.2）====================

// handleNotarizeFixate 即时固化一份证据
// multipart/form-data:
//
//	file:        文件
//	caseId:      案件 ID
//	evidenceId:  （可选）
//	sourceType:  web / live_segment / short_video / document
//	clientHash:  （必填）客户端 Web Crypto API 即时哈希
//	collector:   采集者
//	meta:        JSON 字符串（可选）
func handleNotarizeFixate(c *gin.Context) {
	if notarizeSvc == nil {
		c.JSON(500, gin.H{"success": false, "error": "固化服务未初始化"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 200*1024*1024) // 200MB

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"success": false, "error": "文件读取失败: " + err.Error()})
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "文件读取失败"})
		return
	}

	metaStr := c.PostForm("meta")
	meta := make(map[string]string)
	if metaStr != "" {
		_ = jsonUnmarshalTo(metaStr, &meta)
	}
	if meta == nil {
		meta = map[string]string{}
	}
	if filename := header.Filename; filename != "" {
		meta["originalFilename"] = filename
	}

	req := evidence.FixationRequest{
		CaseID:     c.PostForm("caseId"),
		EvidenceID: c.PostForm("evidenceId"),
		SourceType: c.PostForm("sourceType"),
		Filename:   header.Filename,
		ClientHash: c.PostForm("clientHash"),
		Collector:  c.PostForm("collector"),
		ClientIP:   c.ClientIP(),
		Meta:       meta,
	}
	if req.SourceType == "" {
		req.SourceType = "misc"
	}

	rec, err := notarizeSvc.Fixate(req, fileBytes)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "evidence": rec})
}

func handleListNotarized(c *gin.Context) {
	caseID := c.Param("caseId")
	list, err := notarizeSvc.ListNotarized(caseID)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "evidences": list, "count": len(list)})
}

func handleGetNotarized(c *gin.Context) {
	caseID := c.Param("caseId")
	evID := c.Param("evidenceId")
	rec, err := notarizeSvc.GetNotarized(caseID, evID)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "evidence": rec})
}

func handleGetNotarizedFile(c *gin.Context) {
	caseID := c.Param("caseId")
	evID := c.Param("evidenceId")
	rec, err := notarizeSvc.GetNotarized(caseID, evID)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "error": err.Error()})
		return
	}
	filePath := evidenceStore.GetEvidenceFilePath(caseID, rec.FilePath)
	if filePath == "" {
		c.JSON(404, gin.H{"success": false, "error": "证据文件不存在"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(rec.FilePath)))
	c.File(filePath)
}

func handleVerifyNotarized(c *gin.Context) {
	caseID := c.Param("caseId")
	evID := c.Param("evidenceId")
	rec, result, err := notarizeSvc.VerifyIntegrity(caseID, evID)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"success":  true,
		"evidence": rec,
		"verify":   result,
	})
}

func handleListCustody(c *gin.Context) {
	caseID := c.Param("caseId")
	events, err := notarizeSvc.ListCustody(caseID)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "events": events, "count": len(events)})
}

func handleVerifyCustody(c *gin.Context) {
	caseID := c.Param("caseId")
	ok, reason, err := notarizeSvc.VerifyCustodyChain(caseID)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "valid": ok, "reason": reason})
}

// ==================== 网页取证（直接驱动 MCP + 自动固化）====================

// handleWebScreenshot 对指定 URL 截图并自动固化
// body: { url: string, caseId: string }
func handleWebScreenshot(c *gin.Context) {
	if mcpClient == nil || notarizeSvc == nil {
		c.JSON(500, gin.H{"success": false, "error": "MCP 客户端或固化服务未初始化"})
		return
	}
	var req struct {
		URL    string `json:"url" binding:"required"`
		CaseID string `json:"caseId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "参数错误: " + err.Error()})
		return
	}

	// 1. 调用 MCP screenshot 工具
	result, err := mcpClient.CallTool("screenshot", "take_screenshot", map[string]interface{}{
		"url": req.URL,
	})
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "截图失败: " + err.Error()})
		return
	}
	if result.IsError || len(result.Content) == 0 {
		c.JSON(500, gin.H{"success": false, "error": "截图工具返回错误"})
		return
	}

	// 2. 解析结果 JSON
	textContent, ok := extmcp.AsTextContent(result.Content[0])
	if !ok {
		c.JSON(500, gin.H{"success": false, "error": "截图工具返回非文本内容"})
		return
	}
	textResult := textContent.Text
	var screenshotData struct {
		ScreenshotURL string `json:"screenshotUrl"`
		PageTitle     string `json:"pageTitle"`
		PageURL       string `json:"pageUrl"`
		Timestamp     string `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(textResult), &screenshotData); err != nil {
		c.JSON(500, gin.H{"success": false, "error": "解析截图结果失败: " + err.Error()})
		return
	}

	// 3. 读取截图文件（screenshot server 返回的是 URL 路径 /api/screenshots/xxx.png）
	fileName := filepath.Base(screenshotData.ScreenshotURL)
	screenshotPath := filepath.Join("data", "screenshots", fileName)
	fileBytes, err := os.ReadFile(screenshotPath)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "读取截图文件失败: " + err.Error()})
		return
	}

	// 4. 计算客户端哈希（后端模拟）
	clientHash := notary.ComputeBytesHash(fileBytes)

	// 5. 自动固化
	fixReq := evidence.FixationRequest{
		CaseID:     req.CaseID,
		SourceType: "web",
		Filename:   filepath.Base(screenshotData.ScreenshotURL),
		ClientHash: clientHash,
		Collector:  "web_screenshot_api",
		ClientIP:   c.ClientIP(),
		Meta: map[string]string{
			"pageTitle": screenshotData.PageTitle,
			"pageURL":   screenshotData.PageURL,
			"timestamp": screenshotData.Timestamp,
		},
	}
	rec, err := notarizeSvc.Fixate(fixReq, fileBytes)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "固化失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":    true,
		"message":    "网页截图取证并固化成功",
		"screenshot": screenshotData,
		"evidence":   rec,
	})
}

// handleWebCrawl 抓取网页正文并自动固化
// body: { url: string, caseId: string }
func handleWebCrawl(c *gin.Context) {
	if mcpClient == nil || notarizeSvc == nil {
		c.JSON(500, gin.H{"success": false, "error": "MCP 客户端或固化服务未初始化"})
		return
	}
	var req struct {
		URL    string `json:"url" binding:"required"`
		CaseID string `json:"caseId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "参数错误: " + err.Error()})
		return
	}

	// 1. 调用 MCP webcrawl 工具
	result, err := mcpClient.CallTool("webcrawl", "crawl_page", map[string]interface{}{
		"url": req.URL,
	})
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "抓取失败: " + err.Error()})
		return
	}
	if result.IsError || len(result.Content) == 0 {
		c.JSON(500, gin.H{"success": false, "error": "抓取工具返回错误"})
		return
	}

	// 2. 解析结果 JSON
	textContent, ok := extmcp.AsTextContent(result.Content[0])
	if !ok {
		c.JSON(500, gin.H{"success": false, "error": "抓取工具返回非文本内容"})
		return
	}
	textResult := textContent.Text
	var crawlData struct {
		Title       string            `json:"title"`
		Content     string            `json:"content"`
		URL         string            `json:"url"`
		Author      string            `json:"author"`
		PublishDate string            `json:"publishDate"`
		WordCount   int               `json:"wordCount"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(textResult), &crawlData); err != nil {
		c.JSON(500, gin.H{"success": false, "error": "解析抓取结果失败: " + err.Error()})
		return
	}

	// 3. 保存为 Markdown 文件
	fileName := fmt.Sprintf("crawl_%d.md", time.Now().UnixMilli())
	caseDir := filepath.Join(notarizeSvc.DataDir(), req.CaseID, "web")
	_ = os.MkdirAll(caseDir, 0755)
	filePath := filepath.Join(caseDir, fileName)
	mdContent := fmt.Sprintf("# %s\n\n> URL: %s\n> Author: %s\n> Date: %s\n> WordCount: %d\n\n---\n\n%s",
		crawlData.Title, crawlData.URL, crawlData.Author, crawlData.PublishDate, crawlData.WordCount, crawlData.Content)
	if err := os.WriteFile(filePath, []byte(mdContent), 0644); err != nil {
		c.JSON(500, gin.H{"success": false, "error": "保存抓取内容失败: " + err.Error()})
		return
	}

	// 4. 读取并固化
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "读取文件失败: " + err.Error()})
		return
	}
	clientHash := notary.ComputeBytesHash(fileBytes)
	fixReq := evidence.FixationRequest{
		CaseID:     req.CaseID,
		SourceType: "web",
		Filename:   fileName,
		ClientHash: clientHash,
		Collector:  "web_crawl_api",
		ClientIP:   c.ClientIP(),
		Meta: map[string]string{
			"title":       crawlData.Title,
			"originalURL": crawlData.URL,
			"author":      crawlData.Author,
			"wordCount":   fmt.Sprintf("%d", crawlData.WordCount),
		},
	}
	rec, err := notarizeSvc.Fixate(fixReq, fileBytes)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "固化失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":  true,
		"message":  "网页抓取取证并固化成功",
		"crawl":    crawlData,
		"evidence": rec,
	})
}

// ==================== 区块链锚定（论文 4.3.3）====================

// handleChainAnchor 触发一个案件的证据批量上链（Merkle Tree 聚合）
// body: { evidenceIds?: string[] }（可选，不填则全部未上链证据）
func handleChainAnchor(c *gin.Context) {
	if chainAnchor == nil {
		c.JSON(500, gin.H{"success": false, "error": "区块链锚定服务未初始化"})
		return
	}
	caseID := c.Param("caseId")
	var req struct {
		EvidenceIDs []string `json:"evidenceIds"`
	}
	_ = c.ShouldBindJSON(&req)

	var batch *evidence.AnchorBatch
	var err error
	if len(req.EvidenceIDs) > 0 {
		batch, err = chainAnchor.AnchorSpecificEvidences(caseID, req.EvidenceIDs)
	} else {
		batch, err = chainAnchor.AnchorPendingEvidences(caseID)
	}
	if err != nil {
		c.JSON(400, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "batch": batch})
}

func handleChainVerify(c *gin.Context) {
	if chainAnchor == nil {
		c.JSON(500, gin.H{"success": false, "error": "区块链锚定服务未初始化"})
		return
	}
	caseID := c.Param("caseId")
	evID := c.Param("evidenceId")
	ok, reason, err := chainAnchor.VerifyEvidenceOnChain(caseID, evID)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "onChain": ok, "reason": reason})
}

func handleChainStats(c *gin.Context) {
	if chainAnchor == nil {
		c.JSON(500, gin.H{"success": false, "error": "区块链锚定服务未初始化"})
		return
	}
	c.JSON(200, gin.H{"success": true, "stats": chainAnchor.Stats()})
}

// jsonUnmarshalTo 辅助：JSON 解析
func jsonUnmarshalTo(s string, v interface{}) error {
	return jsonDecode(s, v)
}

func jsonDecode(s string, v interface{}) error {
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), v)
}

// ==================== Helpers ====================

func handleScreenshot(c *gin.Context) {
	filename := c.Param("filename")
	// 安全检查：防止路径穿越
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		c.JSON(400, gin.H{"error": "无效的文件名"})
		return
	}
	filePath := filepath.Join("./data/screenshots", filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "截图文件不存在"})
		return
	}
	c.File(filePath)
}

func init() {
	// placeholder
}

// ==================== 文件上传 ====================

// 支持的图片 MIME 类型
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"image/bmp":  true,
}

// 支持的文档 MIME 类型
var allowedDocTypes = map[string]bool{
	"text/plain":         true,
	"text/markdown":      true,
	"text/csv":           true,
	"application/pdf":    true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

const maxUploadSize = 20 * 1024 * 1024 // 20MB

func handleUpload(c *gin.Context) {
	// 限制请求体大小
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"success": false, "error": "文件读取失败: " + err.Error()})
		return
	}
	defer file.Close()

	// 检查文件大小
	if header.Size > maxUploadSize {
		c.JSON(400, gin.H{"success": false, "error": "文件大小超过 20MB 限制"})
		return
	}

	// 检查 MIME 类型
	mimeType := header.Header.Get("Content-Type")
	isImage := allowedImageTypes[mimeType]
	isDoc := allowedDocTypes[mimeType]

	if !isImage && !isDoc {
		c.JSON(400, gin.H{"success": false, "error": "不支持的文件类型: " + mimeType})
		return
	}

	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "error": "文件读取失败"})
		return
	}

	// 生成唯一文件名
	ext := filepath.Ext(header.Filename)
	fileID := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), uuid.New().String()[:8])
	safeFilename := fileID + ext

	// 保存到磁盘
	savePath := filepath.Join("./data/uploads", safeFilename)
	if err := os.WriteFile(savePath, fileBytes, 0644); err != nil {
		c.JSON(500, gin.H{"success": false, "error": "文件保存失败"})
		return
	}

	// 构建响应
	attachment := models.Attachment{
		ID:       fileID,
		Filename: header.Filename,
		MimeType: mimeType,
		Size:     header.Size,
		URL:      "/api/uploads/" + safeFilename,
	}

	// 对图片生成 base64 Data URI（发给 LLM 的多模态能力用）
	if isImage {
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(fileBytes))
		attachment.DataURI = dataURI
	}

	// 对文档提取文本内容（发给 LLM 作为上下文）
	if isDoc {
		textContent := extractDocumentText(mimeType, fileBytes, savePath)
		if textContent != "" {
			// 限制文本长度，避免超出 LLM 上下文窗口
			const maxTextLen = 50000
			if len(textContent) > maxTextLen {
				textContent = textContent[:maxTextLen] + "\n\n...[文本已截断，原文共 " + fmt.Sprintf("%d", len(textContent)) + " 字符]"
			}
			attachment.TextContent = textContent
		}
	}

	c.JSON(200, gin.H{
		"success":    true,
		"attachment": attachment,
	})
}

func handleUploadedFile(c *gin.Context) {
	filename := c.Param("filename")
	// 安全检查
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		c.JSON(400, gin.H{"error": "无效的文件名"})
		return
	}
	filePath := filepath.Join("./data/uploads", filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "文件不存在"})
		return
	}
	c.File(filePath)
}

// ==================== 文档文本提取 ====================

// extractDocumentText 根据 MIME 类型提取文档的文本内容
func extractDocumentText(mimeType string, fileBytes []byte, filePath string) string {
	switch mimeType {
	case "application/pdf":
		return extractPDFText(filePath)
	case "text/plain", "text/markdown", "text/csv":
		return string(fileBytes)
	case "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		// Word 文档简易提取：尝试提取 docx 中的纯文本
		return extractDocxText(fileBytes)
	default:
		return ""
	}
}

// extractPDFText 从 PDF 文件中提取文本
func extractPDFText(filePath string) string {
	f, r, err := gopdf.Open(filePath)
	if err != nil {
		log.Printf("⚠️ PDF 打开失败: %v", err)
		return ""
	}
	defer f.Close()

	var buf bytes.Buffer
	totalPages := r.NumPage()
	for i := 1; i <= totalPages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			log.Printf("⚠️ PDF 第 %d 页文本提取失败: %v", i, err)
			continue
		}
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString("\n\n")
			}
			buf.WriteString(fmt.Sprintf("--- 第 %d 页 ---\n", i))
			buf.WriteString(text)
		}
	}

	result := buf.String()
	if result == "" {
		log.Printf("⚠️ PDF 文本提取结果为空（可能是扫描版/图片 PDF）")
		return "[此 PDF 为扫描版或图片格式，无法直接提取文本。建议使用 OCR 工具处理。]"
	}

	log.Printf("✅ PDF 文本提取成功: %d 页, %d 字符", totalPages, len(result))
	return result
}

// extractDocxText 从 docx 文件中提取纯文本（简易实现）
func extractDocxText(fileBytes []byte) string {
	// docx 本质是 ZIP 文件，其中 word/document.xml 包含文档内容
	reader := bytes.NewReader(fileBytes)
	zipReader, err := readZipFromBytes(reader, int64(len(fileBytes)))
	if err != nil {
		log.Printf("⚠️ docx 解压失败: %v", err)
		return ""
	}

	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return ""
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return ""
			}
			// 简易提取 XML 中的文本内容（去除 XML 标签）
			text := stripXMLTags(string(data))
			log.Printf("✅ docx 文本提取成功: %d 字符", len(text))
			return text
		}
	}
	return ""
}

// readZipFromBytes 从字节读取 ZIP 文件
func readZipFromBytes(reader *bytes.Reader, size int64) (*zip.Reader, error) {
	return zip.NewReader(reader, size)
}

// stripXMLTags 简易去除 XML 标签，保留文本
func stripXMLTags(s string) string {
	var result strings.Builder
	inTag := false
	lastWasSpace := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			// 在标签结束后添加空格分隔
			if !lastWasSpace {
				result.WriteRune(' ')
				lastWasSpace = true
			}
			continue
		}
		if !inTag {
			if c == '\n' || c == '\r' || c == '\t' {
				if !lastWasSpace {
					result.WriteRune(' ')
					lastWasSpace = true
				}
			} else {
				result.WriteRune(c)
				lastWasSpace = (c == ' ')
			}
		}
	}
	return strings.TrimSpace(result.String())
}

// ==================== 报告管理（论文 4.5）====================

// handleListReports 列出案件下所有报告
func handleListReports(c *gin.Context) {
	caseID := c.Param("caseId")
	if notarizeSvc == nil {
		c.JSON(500, gin.H{"success": false, "error": "固化服务未初始化"})
		return
	}

	reportsDir := filepath.Join(notarizeSvc.DataDir(), caseID, "reports")
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		c.JSON(200, gin.H{"success": true, "reports": []interface{}{}, "count": 0})
		return
	}

	var reportList []map[string]interface{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		// 读取 .meta.json
		metaName := strings.TrimSuffix(entry.Name(), ".json") + ".meta.json"
		metaPath := filepath.Join(reportsDir, metaName)
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta map[string]interface{}
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		reportList = append(reportList, meta)
	}

	c.JSON(200, gin.H{"success": true, "reports": reportList, "count": len(reportList)})
}

// handleGetReport 获取单个报告详情（含 Markdown 正文）
func handleGetReport(c *gin.Context) {
	caseID := c.Param("caseId")
	reportID := c.Param("reportId")
	if notarizeSvc == nil {
		c.JSON(500, gin.H{"success": false, "error": "固化服务未初始化"})
		return
	}

	reportsDir := filepath.Join(notarizeSvc.DataDir(), caseID, "reports")

	// 读取 .meta.json
	metaPath := filepath.Join(reportsDir, reportID+".meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "error": "报告元数据不存在"})
		return
	}
	var meta report.GeneratedReport
	if err := json.Unmarshal(metaData, &meta); err != nil {
		c.JSON(500, gin.H{"success": false, "error": "解析报告元数据失败"})
		return
	}

	// 读取 Markdown 正文
	mdPath := filepath.Join(reportsDir, reportID+".md")
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "error": "报告正文不存在"})
		return
	}

	c.JSON(200, gin.H{
		"success":  true,
		"meta":     meta,
		"markdown": string(mdData),
	})
}
