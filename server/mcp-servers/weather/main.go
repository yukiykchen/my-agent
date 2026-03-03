package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// ==================== JSON-RPC 2.0 Types ====================

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ==================== MCP Types ====================

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]Property    `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ==================== Weather Tool ====================

func getWeather(city string) (string, error) {
	// 使用 wttr.in 免费天气 API
	url := fmt.Sprintf("https://wttr.in/%s?format=j1", urlEncode(city))
	
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch weather: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	
	// 解析并格式化输出
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return string(body), nil // 返回原始响应
	}
	
	// 提取关键信息
	current, ok := data["current_condition"].([]interface{})
	if !ok || len(current) == 0 {
		return string(body), nil
	}
	
	curr := current[0].(map[string]interface{})
	
	result := fmt.Sprintf(`天气信息 - %s

🌡️  温度: %s°C (体感 %s°C)
🌤️  天气: %s
💧  湿度: %s%%
💨  风速: %s km/h
🌧️  降水量: %s mm

详细数据:
%s`,
		city,
		getString(curr, "temp_C"),
		getString(curr, "FeelsLikeC"),
		getString(curr, "weatherDesc.0.value"),
		getString(curr, "humidity"),
		getString(curr, "windspeedKmph"),
		getString(curr, "precipMM"),
		prettyJSON(curr),
	)
	
	return result, nil
}

func getString(m map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	var v interface{} = m
	
	for _, p := range parts {
		switch vv := v.(type) {
		case map[string]interface{}:
			v = vv[p]
		case []interface{}:
			if idx := parseInt(p); idx >= 0 && idx < len(vv) {
				v = vv[idx]
			} else {
				return ""
			}
		default:
			return ""
		}
	}
	
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func urlEncode(s string) string {
	return strings.ReplaceAll(s, " ", "+")
}

func prettyJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// ==================== MCP Server ====================

type MCPServer struct {
_tools []Tool
}

func NewMCPServer() *MCPServer {
	return &MCPServer{
		_tools: []Tool{
			{
				Name:        "get_weather",
				Description: "获取指定城市的当前天气信息。支持中文或英文城市名。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"city": {
							Type:        "string",
							Description: "城市名称，如 'Beijing', 'Shanghai', '广州'",
						},
					},
					Required: []string{"city"},
				},
			},
		},
	}
}

func (s *MCPServer) HandleRequest(req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "notifications/initialized":
		// 通知，无需响应
		return JSONRPCResponse{}
	default:
		return JSONRPCResponse{
			ID: parseID(req.ID),
			Error: &RPCError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

func (s *MCPServer) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	return JSONRPCResponse{
		ID: parseID(req.ID),
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			ServerInfo: ServerInfo{
				Name:    "weather-mcp-server",
				Version: "1.0.0",
			},
		},
	}
}

func (s *MCPServer) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	return JSONRPCResponse{
		ID: parseID(req.ID),
		Result: ToolsListResult{
			Tools: s._tools,
		},
	}
}

func (s *MCPServer) handleToolsCall(req JSONRPCRequest) JSONRPCResponse {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			ID: parseID(req.ID),
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}
	
	var result string
	var err error
	
	switch params.Name {
	case "get_weather":
		city, _ := params.Arguments["city"].(string)
		if city == "" {
			return JSONRPCResponse{
				ID: parseID(req.ID),
				Result: ToolResult{
					Content: []ContentItem{{Type: "text", Text: "错误: 缺少城市参数"}},
					IsError: true,
				},
			}
		}
		result, err = getWeather(city)
	default:
		return JSONRPCResponse{
			ID: parseID(req.ID),
			Error: &RPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Unknown tool: %s", params.Name),
			},
		}
	}
	
	if err != nil {
		return JSONRPCResponse{
			ID: parseID(req.ID),
			Result: ToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("错误: %v", err)}},
				IsError: true,
			},
		}
	}
	
	return JSONRPCResponse{
		ID: parseID(req.ID),
		Result: ToolResult{
			Content: []ContentItem{{Type: "text", Text: result}},
		},
	}
}

func parseID(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	return nil
}

// ==================== Main ====================

func main() {
	server := NewMCPServer()
	
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	
	var mu sync.Mutex
	
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		
		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			mu.Lock()
			encoder.Encode(JSONRPCResponse{
				Error: &RPCError{
					Code:    -32700,
					Message: "Parse error",
				},
			})
			mu.Unlock()
			continue
		}
		
		resp := server.HandleRequest(req)
		
		// 通知类型不需要响应
		if strings.HasPrefix(req.Method, "notifications/") {
			continue
		}
		
		mu.Lock()
		encoder.Encode(resp)
		mu.Unlock()
	}
}
