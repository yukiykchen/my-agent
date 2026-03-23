package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ==================== Screenshot Result Types ====================

type ScreenshotResult struct {
	ScreenshotURL string   `json:"screenshotUrl"`
	PageTitle     string   `json:"pageTitle"`
	PageURL       string   `json:"pageUrl"`
	Timestamp     string   `json:"timestamp"`
	Viewport      Viewport `json:"viewport"`
}

type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type FetchPageResult struct {
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	URL         string            `json:"url"`
	Author      string            `json:"author,omitempty"`
	PublishDate string            `json:"publishDate,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ==================== Browser Manager ====================

type BrowserManager struct {
	browser       *rod.Browser
	screenshotDir string
	mu            sync.Mutex
}

func NewBrowserManager(screenshotDir string) *BrowserManager {
	return &BrowserManager{
		screenshotDir: screenshotDir,
	}
}

func (bm *BrowserManager) ensureBrowser() (*rod.Browser, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.browser != nil {
		return bm.browser, nil
	}

	// 使用 launcher 自动下载和管理浏览器
	u := launcher.New().
		Headless(true).
		MustLaunch()

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect browser: %v", err)
	}

	bm.browser = browser
	return bm.browser, nil
}

func (bm *BrowserManager) Close() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.browser != nil {
		bm.browser.Close()
		bm.browser = nil
	}
}

func (bm *BrowserManager) TakeScreenshot(url string, fullPage bool) (*ScreenshotResult, error) {
	browser, err := bm.ensureBrowser()
	if err != nil {
		return nil, err
	}

	// 创建新页面
	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %v", err)
	}
	defer page.Close()

	// 等待页面加载完成，超时 15 秒
	err = page.Timeout(15 * time.Second).WaitLoad()
	if err != nil {
		return nil, fmt.Errorf("page load timeout: %v", err)
	}

	// 额外等待 2 秒让动态内容渲染
	time.Sleep(2 * time.Second)

	// 获取页面标题
	title, _ := page.Eval(`() => document.title`)
	pageTitle := ""
	if title != nil {
		pageTitle = title.Value.String()
	}

	// 生成文件名
	filename := fmt.Sprintf("%s_%s.png", time.Now().Format("20060102_150405"), randomHex(4))
	savePath := filepath.Join(bm.screenshotDir, filename)

	// 确保目录存在
	os.MkdirAll(bm.screenshotDir, 0755)

	// 截图
	format := proto.PageCaptureScreenshotFormatPng
	screenshotData, err := page.Screenshot(fullPage, &proto.PageCaptureScreenshot{
		Format: format,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %v", err)
	}

	// 保存截图文件
	if err := os.WriteFile(savePath, screenshotData, 0644); err != nil {
		return nil, fmt.Errorf("failed to save screenshot: %v", err)
	}

	return &ScreenshotResult{
		ScreenshotURL: fmt.Sprintf("/api/screenshots/%s", filename),
		PageTitle:     pageTitle,
		PageURL:       url,
		Timestamp:     time.Now().Format(time.RFC3339),
		Viewport:      Viewport{Width: 1280, Height: 720},
	}, nil
}

func (bm *BrowserManager) FetchPage(url string) (*FetchPageResult, error) {
	browser, err := bm.ensureBrowser()
	if err != nil {
		return nil, err
	}

	// 创建新页面
	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %v", err)
	}
	defer page.Close()

	// 等待页面加载完成
	err = page.Timeout(15 * time.Second).WaitLoad()
	if err != nil {
		return nil, fmt.Errorf("page load timeout: %v", err)
	}

	// 额外等待动态内容渲染
	time.Sleep(2 * time.Second)

	// 获取页面标题
	titleVal, _ := page.Eval(`() => document.title`)
	title := ""
	if titleVal != nil {
		title = titleVal.Value.String()
	}

	// 提取正文内容（使用多种策略）
	contentScript := `() => {
		// 尝试常见的文章正文选择器
		const selectors = [
			'article', '.article-content', '.post-content', '.entry-content',
			'.content', '#content', 'main', '.main-content',
			'.article-body', '.story-body', '.text-content'
		];
		
		for (const selector of selectors) {
			const el = document.querySelector(selector);
			if (el && el.innerText.trim().length > 100) {
				return el.innerText.trim();
			}
		}
		
		// 降级：获取 body 文本
		return document.body ? document.body.innerText.trim() : '';
	}`
	contentVal, _ := page.Eval(contentScript)
	content := ""
	if contentVal != nil {
		content = contentVal.Value.String()
	}

	// 限制内容长度（避免过大）
	if len(content) > 50000 {
		content = content[:50000] + "\n\n...[内容已截断，原文超过50000字符]"
	}

	// 提取元数据
	metaScript := `() => {
		const meta = {};
		const metaTags = document.querySelectorAll('meta');
		metaTags.forEach(tag => {
			const name = tag.getAttribute('name') || tag.getAttribute('property') || '';
			const content = tag.getAttribute('content') || '';
			if (name && content) {
				meta[name] = content;
			}
		});
		return meta;
	}`
	metaVal, _ := page.Eval(metaScript)
	metadata := make(map[string]string)
	if metaVal != nil {
		if m, ok := metaVal.Value.Val().(map[string]interface{}); ok {
			for k, v := range m {
				if s, ok := v.(string); ok {
					metadata[k] = s
				}
			}
		}
	}

	// 从元数据中提取作者和发布日期
	author := extractFromMetadata(metadata, "author", "article:author", "og:article:author")
	publishDate := extractFromMetadata(metadata, "date", "article:published_time", "og:article:published_time", "publishdate")

	return &FetchPageResult{
		Title:       title,
		Content:     content,
		URL:         url,
		Author:      author,
		PublishDate: publishDate,
		Metadata:    metadata,
	}, nil
}

func extractFromMetadata(meta map[string]string, keys ...string) string {
	for _, key := range keys {
		if v, ok := meta[key]; ok && v != "" {
			return v
		}
	}
	return ""
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ==================== MCP Server ====================

func main() {
	// 从环境变量获取截图存储目录
	screenshotDir := os.Getenv("SCREENSHOT_DIR")
	if screenshotDir == "" {
		screenshotDir = "./data/screenshots"
	}

	// 确保截图目录存在
	os.MkdirAll(screenshotDir, 0755)

	browserMgr := NewBrowserManager(screenshotDir)
	defer browserMgr.Close()

	// 创建 MCP 服务器
	s := server.NewMCPServer(
		"screenshot-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// 定义截图工具
	screenshotTool := mcp.NewTool("take_screenshot",
		mcp.WithDescription("对指定URL的网页进行截图取证。使用无头浏览器渲染页面后截图，保存为PNG文件。返回截图的访问URL、页面标题、时间戳等元数据。适用于网络侵权证据采集场景。"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("要截图的目标网页URL，如 'https://example.com/article'"),
		),
		mcp.WithBoolean("fullPage",
			mcp.Description("是否进行全页面截图（包含滚动区域）。默认 false 只截取视口区域"),
		),
	)

	// 定义页面抓取工具
	fetchTool := mcp.NewTool("fetch_page",
		mcp.WithDescription("增强版网页内容抓取。使用无头浏览器渲染页面后提取内容，支持JavaScript动态渲染的页面。返回页面标题、正文内容、作者、发布日期等结构化信息。比普通HTTP请求更强大，能抓取SPA等动态页面。"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("要抓取内容的目标网页URL"),
		),
	)

	// 注册截图工具处理函数
	s.AddTool(screenshotTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, err := request.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少 url 参数"), nil
		}

		fullPage := false
		if fp, ok := request.GetArguments()["fullPage"].(bool); ok {
			fullPage = fp
		}

		result, err := browserMgr.TakeScreenshot(url, fullPage)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("截图失败: %v", err)), nil
		}

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	// 注册页面抓取工具处理函数
	s.AddTool(fetchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, err := request.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少 url 参数"), nil
		}

		result, err := browserMgr.FetchPage(url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("页面抓取失败: %v", err)), nil
		}

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	// 启动 stdio 服务器
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
