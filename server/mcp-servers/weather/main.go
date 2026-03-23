package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ==================== Weather Tool ====================

func getWeather(city string) (string, error) {
	// 使用 wttr.in 免费天气 API
	url := fmt.Sprintf("http://wttr.in/%s?format=j1", urlEncode(city))

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

func main() {
	// 创建 MCP 服务器
	s := server.NewMCPServer(
		"weather-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// 定义天气工具
	weatherTool := mcp.NewTool("get_weather",
		mcp.WithDescription("获取指定城市的当前天气信息。支持中文或英文城市名。"),
		mcp.WithString("city",
			mcp.Required(),
			mcp.Description("城市名称，如 'Beijing', 'Shanghai', '广州'"),
		),
	)

	// 注册工具处理函数
	s.AddTool(weatherTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		city, err := request.RequireString("city")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少城市参数"), nil
		}

		result, err := getWeather(city)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("错误: %v", err)), nil
		}

		return mcp.NewToolResultText(result), nil
	})

	// 启动 stdio 服务器
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
