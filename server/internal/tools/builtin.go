package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"infringement-agent-server/internal/models"
)

// RegisterBuiltinTools 注册内置工具
func RegisterBuiltinTools(registry *Registry) {
	// read_file 工具
	registry.Register(
		"read_file",
		"读取指定路径的文件内容",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"path": {Type: "string", Description: "文件路径"},
			},
			Required: []string{"path"},
		},
		func(args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "", fmt.Errorf("path is required")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	)

	// write_file 工具
	registry.Register(
		"write_file",
		"将内容写入指定文件",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"path":    {Type: "string", Description: "文件路径"},
				"content": {Type: "string", Description: "文件内容"},
			},
			Required: []string{"path", "content"},
		},
		func(args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if path == "" {
				return "", fmt.Errorf("path is required")
			}
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", err
			}
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "", err
			}
			return fmt.Sprintf("文件已写入: %s", path), nil
		},
	)

	// read_folder 工具
	registry.Register(
		"read_folder",
		"列出目录内容",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"path": {Type: "string", Description: "目录路径"},
			},
			Required: []string{"path"},
		},
		func(args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "", fmt.Errorf("path is required")
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return "", err
			}
			var lines []string
			for _, e := range entries {
				prefix := "📄"
				if e.IsDir() {
					prefix = "📁"
				}
				lines = append(lines, fmt.Sprintf("%s %s", prefix, e.Name()))
			}
			return strings.Join(lines, "\n"), nil
		},
	)

	// run_shell_command 工具
	registry.Register(
		"run_shell_command",
		"执行 Shell 命令并返回输出",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"command": {Type: "string", Description: "要执行的命令"},
			},
			Required: []string{"command"},
		},
		func(args map[string]interface{}) (string, error) {
			command, _ := args["command"].(string)
			if command == "" {
				return "", fmt.Errorf("command is required")
			}
			cmd := exec.Command("sh", "-c", command)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("命令执行失败: %s\n输出: %s", err.Error(), string(output)), nil
			}
			return string(output), nil
		},
	)

	// web_fetch 工具
	registry.Register(
		"web_fetch",
		"抓取指定 URL 的网页内容，返回文本",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"url": {Type: "string", Description: "要抓取的 URL"},
			},
			Required: []string{"url"},
		},
		func(args map[string]interface{}) (string, error) {
			url, _ := args["url"].(string)
			if url == "" {
				return "", fmt.Errorf("url is required")
			}
			resp, err := http.Get(url)
			if err != nil {
				return "", fmt.Errorf("fetch failed: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024)) // 限制 100KB
			if err != nil {
				return "", fmt.Errorf("read body: %w", err)
			}

			result := map[string]interface{}{
				"status":      resp.StatusCode,
				"contentType": resp.Header.Get("Content-Type"),
				"body":        string(body),
			}
			b, _ := json.MarshalIndent(result, "", "  ")
			return string(b), nil
		},
	)
}
