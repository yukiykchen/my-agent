package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ==================== Result Types ====================

// CrawlPageResult 单页面抓取结果
type CrawlPageResult struct {
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	URL         string            `json:"url"`
	Author      string            `json:"author,omitempty"`
	PublishDate string            `json:"publishDate,omitempty"`
	WordCount   int               `json:"wordCount"`
	Links       []LinkInfo        `json:"links,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// LinkInfo 链接信息
type LinkInfo struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// CrawlBatchResult 批量抓取结果
type CrawlBatchResult struct {
	Total   int               `json:"total"`
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Results []CrawlPageResult `json:"results"`
	Errors  []CrawlError      `json:"errors,omitempty"`
}

// CrawlError 抓取错误
type CrawlError struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

// ==================== Browser Manager ====================

// BrowserManager 浏览器管理器（复用浏览器实例）
type BrowserManager struct {
	browser *rod.Browser
	mu      sync.Mutex
}

// NewBrowserManager 创建浏览器管理器
func NewBrowserManager() *BrowserManager {
	return &BrowserManager{}
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

// Close 关闭浏览器
func (bm *BrowserManager) Close() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.browser != nil {
		bm.browser.Close()
		bm.browser = nil
	}
}

// ==================== Readability.js (Minified Core) ====================

// readabilityJS 是一个简化版的 Readability 算法，注入到浏览器中执行
// 核心逻辑：计算每个节点的文本密度，去掉导航栏、页脚、广告等噪音
const readabilityJS = `() => {
	// ====== 简化版 Readability 正文提取 ======
	function extractArticle() {
		// 1. 优先查找 <article> 或语义化标签
		const semanticSelectors = [
			'article',
			'[role="article"]',
			'[itemprop="articleBody"]',
			'.article-content', '.article-body', '.article_content',
			'.post-content', '.post-body', '.post_content',
			'.entry-content', '.entry-body',
			'.story-body', '.story-content',
			'.news-content', '.news-body',
			'.blog-content', '.blog-post',
			'.rich_media_content',   // 微信公众号
			'#js_content',           // 微信公众号
			'.Post-RichTextContainer', // 知乎
			'.markdown-body',        // GitHub
		];

		for (const sel of semanticSelectors) {
			const el = document.querySelector(sel);
			if (el && el.innerText.trim().length > 200) {
				return el;
			}
		}

		// 2. 文本密度算法：找到文本最密集的块级元素
		const candidates = [];
		const blocks = document.querySelectorAll('div, section, main, td');

		for (const block of blocks) {
			const text = block.innerText || '';
			const textLen = text.trim().length;
			if (textLen < 100) continue;

			// 计算文本密度 = 文本长度 / 子元素数量
			const childCount = block.children.length || 1;
			const linkText = Array.from(block.querySelectorAll('a'))
				.map(a => (a.innerText || '').length)
				.reduce((a, b) => a + b, 0);
			const linkDensity = textLen > 0 ? linkText / textLen : 0;

			// 排除链接密度过高的区域（导航栏等）
			if (linkDensity > 0.5) continue;

			const score = textLen / childCount * (1 - linkDensity);
			candidates.push({ el: block, score, textLen });
		}

		// 按得分排序
		candidates.sort((a, b) => b.score - a.score);

		if (candidates.length > 0) {
			return candidates[0].el;
		}

		// 3. 降级: body
		return document.body;
	}

	const article = extractArticle();
	if (!article) return { title: document.title || '', content: '', html: '' };

	return {
		title: document.title || '',
		content: article.innerText.trim(),
		html: article.innerHTML
	};
}`

// ==================== HTML to Markdown Converter ====================

// htmlToMarkdownJS 浏览器端 HTML→Markdown 转换（纯 JS 实现）
const htmlToMarkdownJS = `(html) => {
	if (!html) return '';

	// 创建临时容器
	const div = document.createElement('div');
	div.innerHTML = html;

	function convert(node, indent) {
		if (!node) return '';
		indent = indent || '';

		// 文本节点
		if (node.nodeType === 3) {
			return node.textContent.replace(/\s+/g, ' ');
		}

		// 非元素节点
		if (node.nodeType !== 1) return '';

		const tag = node.tagName.toLowerCase();
		let result = '';

		switch (tag) {
			case 'h1': return '\n# ' + getChildText(node) + '\n\n';
			case 'h2': return '\n## ' + getChildText(node) + '\n\n';
			case 'h3': return '\n### ' + getChildText(node) + '\n\n';
			case 'h4': return '\n#### ' + getChildText(node) + '\n\n';
			case 'h5': return '\n##### ' + getChildText(node) + '\n\n';
			case 'h6': return '\n###### ' + getChildText(node) + '\n\n';

			case 'p':
				return '\n' + getChildText(node) + '\n\n';

			case 'br':
				return '\n';

			case 'hr':
				return '\n---\n\n';

			case 'strong': case 'b':
				return '**' + getChildText(node) + '**';

			case 'em': case 'i':
				return '*' + getChildText(node) + '*';

			case 'code':
				if (node.parentElement && node.parentElement.tagName.toLowerCase() === 'pre') {
					return node.textContent;
				}
				return '` + "`" + `' + node.textContent + '` + "`" + `';

			case 'pre':
				const codeEl = node.querySelector('code');
				const lang = codeEl ? (codeEl.className.match(/language-(\w+)/) || [])[1] || '' : '';
				const code = codeEl ? codeEl.textContent : node.textContent;
				return '\n` + "```" + `' + lang + '\n' + code + '\n` + "```" + `\n\n';

			case 'a':
				const href = node.getAttribute('href') || '';
				const text = getChildText(node);
				if (!href || href.startsWith('javascript:')) return text;
				return '[' + text + '](' + href + ')';

			case 'img':
				const src = node.getAttribute('src') || '';
				const alt = node.getAttribute('alt') || 'image';
				if (!src) return '';
				return '![' + alt + '](' + src + ')';

			case 'ul':
				result = '\n';
				for (const li of node.children) {
					if (li.tagName && li.tagName.toLowerCase() === 'li') {
						result += indent + '- ' + getChildText(li).trim() + '\n';
					}
				}
				return result + '\n';

			case 'ol':
				result = '\n';
				let idx = 1;
				for (const li of node.children) {
					if (li.tagName && li.tagName.toLowerCase() === 'li') {
						result += indent + idx + '. ' + getChildText(li).trim() + '\n';
						idx++;
					}
				}
				return result + '\n';

			case 'blockquote':
				const lines = getChildText(node).split('\n');
				return '\n' + lines.map(l => '> ' + l).join('\n') + '\n\n';

			case 'table':
				return convertTable(node);

			case 'script': case 'style': case 'noscript': case 'svg':
				return '';

			default:
				return getChildren(node, indent);
		}
	}

	function getChildText(node) {
		let text = '';
		for (const child of node.childNodes) {
			text += convert(child);
		}
		return text;
	}

	function getChildren(node, indent) {
		let result = '';
		for (const child of node.childNodes) {
			result += convert(child, indent);
		}
		return result;
	}

	function convertTable(table) {
		const rows = table.querySelectorAll('tr');
		if (!rows.length) return '';

		let md = '\n';
		let isFirst = true;
		for (const row of rows) {
			const cells = row.querySelectorAll('th, td');
			const cellTexts = Array.from(cells).map(c => c.innerText.trim().replace(/\|/g, '\\|'));
			md += '| ' + cellTexts.join(' | ') + ' |\n';

			if (isFirst) {
				md += '| ' + cellTexts.map(() => '---').join(' | ') + ' |\n';
				isFirst = false;
			}
		}
		return md + '\n';
	}

	let markdown = convert(div);

	// 后处理：清理多余空行
	markdown = markdown.replace(/\n{3,}/g, '\n\n');
	markdown = markdown.trim();
	return markdown;
}`

// ==================== Core Crawl Functions ====================

// CrawlPage 抓取单个页面
func (bm *BrowserManager) CrawlPage(url string, waitSelector string, extractLinks bool) (*CrawlPageResult, error) {
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

	// 等待页面加载完成，超时 30 秒
	err = page.Timeout(30 * time.Second).WaitLoad()
	if err != nil {
		return nil, fmt.Errorf("page load timeout: %v", err)
	}

	// 如果指定了等待选择器，额外等待该元素出现
	if waitSelector != "" {
		err = page.Timeout(10 * time.Second).MustElement(waitSelector).WaitVisible()
		if err != nil {
			// 不报错，继续处理
		}
	}

	// 额外等待 2 秒让动态内容渲染
	time.Sleep(2 * time.Second)

	// 1. 使用 Readability 算法提取正文
	articleVal, err := page.Eval(readabilityJS)
	if err != nil {
		return nil, fmt.Errorf("readability extraction failed: %v", err)
	}

	var articleData map[string]interface{}
	if articleVal != nil {
		if m, ok := articleVal.Value.Val().(map[string]interface{}); ok {
			articleData = m
		}
	}
	if articleData == nil {
		articleData = make(map[string]interface{})
	}

	title, _ := articleData["title"].(string)
	plainContent, _ := articleData["content"].(string)
	htmlContent, _ := articleData["html"].(string)

	// 2. HTML → Markdown 转换
	var markdownContent string
	if htmlContent != "" {
		mdVal, err := page.Eval(htmlToMarkdownJS, htmlContent)
		if err == nil && mdVal != nil {
			markdownContent = mdVal.Value.String()
		}
	}

	// 如果 Markdown 转换失败，降级为纯文本
	if markdownContent == "" {
		markdownContent = plainContent
	}

	// 3. 提取元数据
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

	// 4. 提取链接（可选）
	var links []LinkInfo
	if extractLinks {
		links = bm.extractPageLinks(page, url)
	}

	// 5. 从元数据中提取作者和发布日期
	author := extractFromMeta(metadata, "author", "article:author", "og:article:author")
	publishDate := extractFromMeta(metadata, "date", "article:published_time", "og:article:published_time", "publishdate")

	// 限制内容长度
	if len(markdownContent) > 80000 {
		markdownContent = markdownContent[:80000] + "\n\n...[内容已截断，原文超过80000字符]"
	}

	// 计算字数（中文按字符数，英文按空格分词）
	wordCount := countWords(markdownContent)

	return &CrawlPageResult{
		Title:       title,
		Content:     markdownContent,
		URL:         url,
		Author:      author,
		PublishDate: publishDate,
		WordCount:   wordCount,
		Links:       links,
		Metadata:    metadata,
	}, nil
}

// extractPageLinks 提取页面所有链接
func (bm *BrowserManager) extractPageLinks(page *rod.Page, baseURL string) []LinkInfo {
	linksScript := `() => {
		const links = [];
		const seen = new Set();
		document.querySelectorAll('a[href]').forEach(a => {
			const href = a.href; // 浏览器已解析为绝对 URL
			const text = (a.innerText || a.title || '').trim();
			if (!href || href.startsWith('javascript:') || href.startsWith('#')) return;
			if (seen.has(href)) return;
			seen.add(href);
			links.push({ text: text.substring(0, 200), url: href });
		});
		return links;
	}`
	linksVal, err := page.Eval(linksScript)
	if err != nil {
		return nil
	}

	var links []LinkInfo
	if linksVal != nil {
		data, _ := json.Marshal(linksVal.Value.Val())
		json.Unmarshal(data, &links)
	}
	return links
}

// CrawlLinks 只提取页面链接
func (bm *BrowserManager) CrawlLinks(url string, filterPattern string) ([]LinkInfo, error) {
	browser, err := bm.ensureBrowser()
	if err != nil {
		return nil, err
	}

	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %v", err)
	}
	defer page.Close()

	err = page.Timeout(30 * time.Second).WaitLoad()
	if err != nil {
		return nil, fmt.Errorf("page load timeout: %v", err)
	}

	time.Sleep(2 * time.Second)

	links := bm.extractPageLinks(page, url)

	// 如果有过滤模式，进行正则过滤
	if filterPattern != "" {
		re, err := regexp.Compile(filterPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid filter pattern: %v", err)
		}
		var filtered []LinkInfo
		for _, link := range links {
			if re.MatchString(link.URL) || re.MatchString(link.Text) {
				filtered = append(filtered, link)
			}
		}
		links = filtered
	}

	return links, nil
}

// CrawlBatch 批量抓取
func (bm *BrowserManager) CrawlBatch(urls []string, concurrency int) *CrawlBatchResult {
	if concurrency <= 0 {
		concurrency = 3
	}
	if concurrency > 5 {
		concurrency = 5 // 限制最大并发
	}

	result := &CrawlBatchResult{
		Total:   len(urls),
		Results: make([]CrawlPageResult, 0),
	}

	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		sem <- struct{}{}

		go func(u string) {
			defer wg.Done()
			defer func() { <-sem }()

			pageResult, err := bm.CrawlPage(u, "", false)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, CrawlError{URL: u, Error: err.Error()})
			} else {
				result.Success++
				result.Results = append(result.Results, *pageResult)
			}
		}(url)
	}

	wg.Wait()
	return result
}

// ==================== Helper Functions ====================

func extractFromMeta(meta map[string]string, keys ...string) string {
	for _, key := range keys {
		if v, ok := meta[key]; ok && v != "" {
			return v
		}
	}
	return ""
}

func countWords(text string) int {
	// 统计中英文混合文本的字数
	// 中文: 每个字算一个词
	// 英文: 按空格分词
	count := 0
	inWord := false
	for _, r := range text {
		if r > 0x4E00 && r < 0x9FFF {
			// CJK 字符
			count++
			inWord = false
		} else if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			if inWord {
				count++
				inWord = false
			}
		} else if r > 32 {
			inWord = true
		}
	}
	if inWord {
		count++
	}
	return count
}

// ==================== MCP Server ====================

func main() {
	browserMgr := NewBrowserManager()
	defer browserMgr.Close()

	// 创建 MCP 服务器
	s := server.NewMCPServer(
		"webcrawl-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// ======= Tool 1: crawl_page =======
	crawlPageTool := mcp.NewTool("crawl_page",
		mcp.WithDescription("智能抓取网页内容并转换为 Markdown 格式。使用无头浏览器渲染页面（支持 JS 动态页面），通过 Readability 算法自动提取正文（去除导航栏、广告、页脚等噪音），将 HTML 转为结构化 Markdown。返回标题、正文（Markdown）、作者、发布日期、字数等信息。适用于文章内容抓取、侵权证据采集、网页内容分析等场景。"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("要抓取的目标网页 URL，如 'https://example.com/article'"),
		),
		mcp.WithString("waitForSelector",
			mcp.Description("可选：等待页面中某个 CSS 选择器出现后再提取内容。适用于需要等待异步加载的 SPA 页面，如 '.article-content'"),
		),
		mcp.WithBoolean("extractLinks",
			mcp.Description("是否同时提取页面中的所有链接。默认 false"),
		),
	)

	// ======= Tool 2: crawl_links =======
	crawlLinksTool := mcp.NewTool("crawl_links",
		mcp.WithDescription("提取网页中的所有链接。使用无头浏览器渲染页面后提取所有 <a> 标签的链接和文本。支持正则表达式过滤。适用于网站链接发现、页面导航分析、批量链接采集等场景。"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("要提取链接的目标网页 URL"),
		),
		mcp.WithString("filter",
			mcp.Description("可选：正则表达式过滤链接。只返回 URL 或链接文本匹配该模式的链接，如 '/article/' 或 'blog\\\\.example\\\\.com'"),
		),
	)

	// ======= Tool 3: crawl_batch =======
	crawlBatchTool := mcp.NewTool("crawl_batch",
		mcp.WithDescription("批量抓取多个网页内容。并发抓取多个 URL 的页面内容，每个页面都使用 Readability 算法提取正文并转为 Markdown。支持控制并发数。适用于批量证据采集、多页面对比分析等场景。"),
		mcp.WithString("urls",
			mcp.Required(),
			mcp.Description("要抓取的 URL 列表，用逗号分隔或 JSON 数组格式。如 'https://a.com,https://b.com' 或 '[\"https://a.com\",\"https://b.com\"]'"),
		),
		mcp.WithNumber("concurrency",
			mcp.Description("并发数，默认 3，最大 5。控制同时抓取的页面数量"),
		),
	)

	// 注册 crawl_page 处理函数
	s.AddTool(crawlPageTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, err := request.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少 url 参数"), nil
		}

		waitSelector := ""
		if ws, ok := request.GetArguments()["waitForSelector"].(string); ok {
			waitSelector = ws
		}

		extractLinks := false
		if el, ok := request.GetArguments()["extractLinks"].(bool); ok {
			extractLinks = el
		}

		result, err := browserMgr.CrawlPage(url, waitSelector, extractLinks)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("页面抓取失败: %v", err)), nil
		}

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	// 注册 crawl_links 处理函数
	s.AddTool(crawlLinksTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, err := request.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少 url 参数"), nil
		}

		filter := ""
		if f, ok := request.GetArguments()["filter"].(string); ok {
			filter = f
		}

		links, err := browserMgr.CrawlLinks(url, filter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("链接提取失败: %v", err)), nil
		}

		resultJSON, _ := json.Marshal(map[string]interface{}{
			"url":        url,
			"totalLinks": len(links),
			"links":      links,
		})
		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	// 注册 crawl_batch 处理函数
	s.AddTool(crawlBatchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		urlsRaw, err := request.RequireString("urls")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少 urls 参数"), nil
		}

		// 解析 URL 列表：支持 JSON 数组和逗号分隔
		var urls []string
		if err := json.Unmarshal([]byte(urlsRaw), &urls); err != nil {
			// 尝试逗号分隔
			for _, u := range strings.Split(urlsRaw, ",") {
				u = strings.TrimSpace(u)
				if u != "" {
					urls = append(urls, u)
				}
			}
		}

		if len(urls) == 0 {
			return mcp.NewToolResultError("错误: urls 列表为空"), nil
		}

		if len(urls) > 10 {
			return mcp.NewToolResultError("错误: 一次最多抓取 10 个 URL"), nil
		}

		concurrency := 3
		if c, ok := request.GetArguments()["concurrency"].(float64); ok {
			concurrency = int(c)
		}

		result := browserMgr.CrawlBatch(urls, concurrency)

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	// 启动 stdio 服务器
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
