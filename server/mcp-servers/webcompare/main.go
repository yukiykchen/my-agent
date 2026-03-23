package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yanyiwu/gojieba"
)

// ==================== Result Types ====================

// WebCompareResult 网页比对结果
type WebCompareResult struct {
	Page1         PageInfo         `json:"page1"`
	Page2         PageInfo         `json:"page2"`
	Similarity    SimilarityResult `json:"similarity"`
	Report        string           `json:"report"`
	ComparedAt    string           `json:"comparedAt"`
	DurationMs    int64            `json:"durationMs"`
}

// PageInfo 页面信息
type PageInfo struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	WordCount int    `json:"wordCount"`
	Excerpt   string `json:"excerpt"` // 内容前200字摘要
}

// SimilarityResult 相似度结果
type SimilarityResult struct {
	OverallScore     float64        `json:"overallScore"`
	CosineSimilarity float64        `json:"cosineSimilarity"`
	JaccardIndex     float64        `json:"jaccardIndex"`
	LCSRatio         float64        `json:"lcsRatio"`
	Verdict          string         `json:"verdict"`
	CommonWords      int            `json:"commonWords"`
	TopCommonWords   []string       `json:"topCommonWords"`   // 高频共有词（前20个）
	SimilarSegments  []SegmentPair  `json:"similarSegments"`  // 相似片段示例
}

// SegmentPair 相似文本片段对
type SegmentPair struct {
	Text1  string  `json:"text1"`
	Text2  string  `json:"text2"`
	Score  float64 `json:"score"`
}

// ==================== Readability JS ====================

const readabilityJS = `() => {
	// 精简版 Readability 提取
	function extractContent() {
		// 尝试常见正文选择器
		const selectors = [
			'article', '.article-content', '.post-content', '.entry-content',
			'.content', '#content', 'main', '.main-content',
			'.article-body', '.story-body', '.text-content', '.rich_media_content',
			'.Post-RichTextContainer', '.Post-Main',
		];
		
		for (const selector of selectors) {
			const el = document.querySelector(selector);
			if (el && el.innerText.trim().length > 100) {
				return {
					title: document.title || '',
					content: el.innerText.trim(),
				};
			}
		}

		// 降级：获取 body 文本
		return {
			title: document.title || '',
			content: document.body ? document.body.innerText.trim() : '',
		};
	}
	return extractContent();
}`

// ==================== Browser Manager ====================

type BrowserManager struct {
	browser *rod.Browser
}

func NewBrowserManager() *BrowserManager {
	return &BrowserManager{}
}

func (bm *BrowserManager) Close() {
	if bm.browser != nil {
		bm.browser.Close()
	}
}

func (bm *BrowserManager) ensureBrowser() (*rod.Browser, error) {
	if bm.browser != nil {
		return bm.browser, nil
	}

	path, _ := launcher.LookPath()
	u := launcher.New().
		Bin(path).
		Headless(true).
		Set("disable-gpu").
		Set("no-sandbox").
		MustLaunch()

	bm.browser = rod.New().ControlURL(u).MustConnect()
	return bm.browser, nil
}

// FetchPage 抓取单个页面内容
func (bm *BrowserManager) FetchPage(url string) (title, content string, err error) {
	browser, err := bm.ensureBrowser()
	if err != nil {
		return "", "", err
	}

	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return "", "", fmt.Errorf("create page failed: %v", err)
	}
	defer page.Close()

	err = page.Timeout(30 * time.Second).WaitLoad()
	if err != nil {
		return "", "", fmt.Errorf("page load timeout: %v", err)
	}

	// 额外等待动态内容渲染
	time.Sleep(2 * time.Second)

	val, err := page.Eval(readabilityJS)
	if err != nil {
		return "", "", fmt.Errorf("content extraction failed: %v", err)
	}

	if val != nil {
		if m, ok := val.Value.Val().(map[string]interface{}); ok {
			title, _ = m["title"].(string)
			content, _ = m["content"].(string)
		}
	}

	// 限制内容长度
	if len(content) > 80000 {
		content = content[:80000]
	}

	return title, content, nil
}

// ==================== Text Compare Engine ====================

type TextCompareEngine struct {
	jieba *gojieba.Jieba
}

func NewTextCompareEngine() *TextCompareEngine {
	return &TextCompareEngine{
		jieba: gojieba.NewJieba(),
	}
}

func (e *TextCompareEngine) Close() {
	e.jieba.Free()
}

func (e *TextCompareEngine) tokenize(text string) []string {
	words := e.jieba.Cut(text, true)
	var filtered []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" || isAllPunctOrSpace(w) {
			continue
		}
		filtered = append(filtered, w)
	}
	return filtered
}

func (e *TextCompareEngine) cosineSimilarity(words1, words2 []string) float64 {
	tf1 := termFrequency(words1)
	tf2 := termFrequency(words2)

	var dotProduct, norm1, norm2 float64
	for word, freq1 := range tf1 {
		if freq2, ok := tf2[word]; ok {
			dotProduct += freq1 * freq2
		}
		norm1 += freq1 * freq1
	}
	for _, freq2 := range tf2 {
		norm2 += freq2 * freq2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

func (e *TextCompareEngine) jaccardSimilarity(words1, words2 []string) float64 {
	set1 := makeSet(words1)
	set2 := makeSet(words2)

	intersection := 0
	for w := range set1 {
		if _, ok := set2[w]; ok {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func (e *TextCompareEngine) lcsRatio(text1, text2 string) float64 {
	runes1 := []rune(text1)
	runes2 := []rune(text2)

	// 对超长文本使用分词级 LCS
	if len(runes1) > 10000 || len(runes2) > 10000 {
		words1 := e.tokenize(text1)
		words2 := e.tokenize(text2)
		return lcsRatioSlice(words1, words2)
	}

	return lcsRatioRunes(runes1, runes2)
}

func lcsRatioRunes(runes1, runes2 []rune) float64 {
	m := len(runes1)
	n := len(runes2)
	if m == 0 || n == 0 {
		return 0
	}

	prev := make([]int, n+1)
	curr := make([]int, n+1)

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if runes1[i-1] == runes2[j-1] {
				curr[j] = prev[j-1] + 1
			} else {
				curr[j] = maxInt(prev[j], curr[j-1])
			}
		}
		prev, curr = curr, make([]int, n+1)
	}

	lcsLen := prev[n]
	maxLen := maxInt(m, n)
	return float64(lcsLen) / float64(maxLen)
}

func lcsRatioSlice(words1, words2 []string) float64 {
	m := len(words1)
	n := len(words2)
	if m == 0 || n == 0 {
		return 0
	}
	if m > 1000 {
		words1 = words1[:1000]
		m = 1000
	}
	if n > 1000 {
		words2 = words2[:1000]
		n = 1000
	}

	prev := make([]int, n+1)
	curr := make([]int, n+1)

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if words1[i-1] == words2[j-1] {
				curr[j] = prev[j-1] + 1
			} else {
				curr[j] = maxInt(prev[j], curr[j-1])
			}
		}
		prev, curr = curr, make([]int, n+1)
	}

	lcsLen := prev[n]
	maxLen := maxInt(m, n)
	return float64(lcsLen) / float64(maxLen)
}

// findTopCommonWords 找出高频共有词
func (e *TextCompareEngine) findTopCommonWords(words1, words2 []string, topN int) []string {
	tf1 := termFrequency(words1)
	tf2 := termFrequency(words2)

	type wordScore struct {
		word  string
		score float64
	}

	var common []wordScore
	for w, f1 := range tf1 {
		if f2, ok := tf2[w]; ok {
			// 跳过太短的词
			if len([]rune(w)) < 2 {
				continue
			}
			common = append(common, wordScore{w, f1 + f2})
		}
	}

	// 按分数排序
	for i := 0; i < len(common); i++ {
		for j := i + 1; j < len(common); j++ {
			if common[j].score > common[i].score {
				common[i], common[j] = common[j], common[i]
			}
		}
	}

	result := make([]string, 0, topN)
	for i, ws := range common {
		if i >= topN {
			break
		}
		result = append(result, ws.word)
	}
	return result
}

// findSimilarSegments 找相似文本片段（基于滑动窗口）
func (e *TextCompareEngine) findSimilarSegments(text1, text2 string, maxSegments int) []SegmentPair {
	// 按句子/段落切分
	sentences1 := splitSentences(text1)
	sentences2 := splitSentences(text2)

	if len(sentences1) == 0 || len(sentences2) == 0 {
		return nil
	}

	type scoredPair struct {
		i, j  int
		score float64
	}

	var pairs []scoredPair

	// 对每个句子1，找句子2中最相似的
	for i, s1 := range sentences1 {
		if len([]rune(s1)) < 20 {
			continue // 跳过太短的句子
		}
		words1 := e.tokenize(s1)
		if len(words1) < 5 {
			continue
		}

		bestScore := 0.0
		bestJ := -1

		for j, s2 := range sentences2 {
			if len([]rune(s2)) < 20 {
				continue
			}
			words2 := e.tokenize(s2)
			if len(words2) < 5 {
				continue
			}

			score := e.cosineSimilarity(words1, words2)
			if score > bestScore && score > 0.5 {
				bestScore = score
				bestJ = j
			}
		}

		if bestJ >= 0 {
			pairs = append(pairs, scoredPair{i, bestJ, bestScore})
		}
	}

	// 按相似度排序
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].score > pairs[i].score {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	// 取 top N
	var result []SegmentPair
	seen := make(map[int]bool)
	for _, p := range pairs {
		if len(result) >= maxSegments {
			break
		}
		if seen[p.j] {
			continue // 避免重复匹配同一句
		}
		seen[p.j] = true

		s1 := sentences1[p.i]
		s2 := sentences2[p.j]
		// 截取合理长度
		if len([]rune(s1)) > 200 {
			s1 = string([]rune(s1)[:200]) + "..."
		}
		if len([]rune(s2)) > 200 {
			s2 = string([]rune(s2)[:200]) + "..."
		}

		result = append(result, SegmentPair{
			Text1: s1,
			Text2: s2,
			Score: roundTo(p.score, 4),
		})
	}

	return result
}

// CompareTexts 比较两段文本，返回完整的相似度分析
func (e *TextCompareEngine) CompareTexts(text1, text2 string) SimilarityResult {
	words1 := e.tokenize(text1)
	words2 := e.tokenize(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return SimilarityResult{
			Verdict: "无法比较",
		}
	}

	cosine := e.cosineSimilarity(words1, words2)
	jaccard := e.jaccardSimilarity(words1, words2)
	lcs := e.lcsRatio(text1, text2)

	// 共有词
	set1 := makeSet(words1)
	set2 := makeSet(words2)
	commonCount := 0
	for w := range set1 {
		if _, ok := set2[w]; ok {
			commonCount++
		}
	}

	overallScore := cosine*0.4 + jaccard*0.3 + lcs*0.3

	topCommon := e.findTopCommonWords(words1, words2, 20)
	segments := e.findSimilarSegments(text1, text2, 5)

	return SimilarityResult{
		OverallScore:     roundTo(overallScore, 4),
		CosineSimilarity: roundTo(cosine, 4),
		JaccardIndex:     roundTo(jaccard, 4),
		LCSRatio:         roundTo(lcs, 4),
		Verdict:          getVerdict(overallScore),
		CommonWords:      commonCount,
		TopCommonWords:   topCommon,
		SimilarSegments:  segments,
	}
}

// ==================== Report Generator ====================

func generateReport(result *WebCompareResult) string {
	sim := result.Similarity
	p1 := result.Page1
	p2 := result.Page2

	var sb strings.Builder

	sb.WriteString("# 📊 网页内容相似性分析报告\n\n")
	sb.WriteString(fmt.Sprintf("**生成时间**: %s\n\n", result.ComparedAt))
	sb.WriteString("---\n\n")

	// 概览
	sb.WriteString("## 📋 比对概览\n\n")
	sb.WriteString("| 维度 | 页面 A | 页面 B |\n")
	sb.WriteString("|------|--------|--------|\n")
	sb.WriteString(fmt.Sprintf("| **URL** | %s | %s |\n", p1.URL, p2.URL))
	sb.WriteString(fmt.Sprintf("| **标题** | %s | %s |\n", p1.Title, p2.Title))
	sb.WriteString(fmt.Sprintf("| **字数** | %d 字 | %d 字 |\n", p1.WordCount, p2.WordCount))
	sb.WriteString("\n")

	// 综合评分
	scoreEmoji := "🟢"
	if sim.OverallScore >= 0.8 {
		scoreEmoji = "🔴"
	} else if sim.OverallScore >= 0.5 {
		scoreEmoji = "🟡"
	} else if sim.OverallScore >= 0.3 {
		scoreEmoji = "🟠"
	}
	sb.WriteString(fmt.Sprintf("## %s 综合评分: %.1f%% — %s\n\n", scoreEmoji, sim.OverallScore*100, sim.Verdict))

	// 详细维度
	sb.WriteString("## 📐 各维度分析\n\n")
	sb.WriteString("| 指标 | 得分 | 说明 |\n")
	sb.WriteString("|------|------|------|\n")
	sb.WriteString(fmt.Sprintf("| 余弦相似度 | %.1f%% | 衡量整体语义方向是否一致 |\n", sim.CosineSimilarity*100))
	sb.WriteString(fmt.Sprintf("| Jaccard 系数 | %.1f%% | 衡量词汇重叠程度 |\n", sim.JaccardIndex*100))
	sb.WriteString(fmt.Sprintf("| LCS 比率 | %.1f%% | 衡量内容连续相似性 |\n", sim.LCSRatio*100))
	sb.WriteString(fmt.Sprintf("| 共有词汇数 | %d 个 | 两篇文章共同使用的词汇 |\n", sim.CommonWords))
	sb.WriteString("\n")

	// 高频共有词
	if len(sim.TopCommonWords) > 0 {
		sb.WriteString("## 🏷️ 高频共有关键词\n\n")
		sb.WriteString("`")
		sb.WriteString(strings.Join(sim.TopCommonWords, "` `"))
		sb.WriteString("`\n\n")
	}

	// 相似片段
	if len(sim.SimilarSegments) > 0 {
		sb.WriteString("## 🔍 相似内容片段\n\n")
		for i, seg := range sim.SimilarSegments {
			sb.WriteString(fmt.Sprintf("### 片段 %d（相似度 %.1f%%）\n\n", i+1, seg.Score*100))
			sb.WriteString(fmt.Sprintf("**页面 A**:\n> %s\n\n", seg.Text1))
			sb.WriteString(fmt.Sprintf("**页面 B**:\n> %s\n\n", seg.Text2))
		}
	}

	// 判定说明
	sb.WriteString("## ⚖️ 分析结论\n\n")
	switch {
	case sim.OverallScore >= 0.8:
		sb.WriteString("🔴 **高度相似**：两个网页的内容高度重合，存在较大的抄袭/侵权嫌疑。建议：\n\n")
		sb.WriteString("1. 确认两篇文章的发布时间先后顺序\n")
		sb.WriteString("2. 检查是否存在授权转载声明\n")
		sb.WriteString("3. 保留截图、网页快照等证据\n")
		sb.WriteString("4. 考虑通过法律途径维权\n")
	case sim.OverallScore >= 0.5:
		sb.WriteString("🟡 **中度相似**：两个网页存在一定程度的内容重合，可能存在部分借鉴或引用。建议：\n\n")
		sb.WriteString("1. 逐段比对具体相似的内容片段\n")
		sb.WriteString("2. 判断相似部分是否属于公共知识或通用表述\n")
		sb.WriteString("3. 检查是否有引用标注\n")
	case sim.OverallScore >= 0.3:
		sb.WriteString("🟠 **低度相似**：两个网页有少量相似内容，可能是巧合或共同引用了公共素材。\n\n")
	default:
		sb.WriteString("🟢 **差异较大**：两个网页内容差异明显，未发现显著相似性。\n\n")
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString(fmt.Sprintf("*分析耗时: %dms*\n", result.DurationMs))

	return sb.String()
}

// ==================== Helper Functions ====================

func splitSentences(text string) []string {
	// 按常见的句子分隔符切分
	delimiters := []string{"。", "！", "？", ".", "!", "?", "\n\n", "\n"}
	result := []string{text}

	for _, delim := range delimiters {
		var newResult []string
		for _, s := range result {
			parts := strings.Split(s, delim)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if len(p) > 0 {
					newResult = append(newResult, p)
				}
			}
		}
		if len(newResult) > 3 {
			result = newResult
			break // 用第一个有效的分隔符
		}
	}
	return result
}

func termFrequency(words []string) map[string]float64 {
	tf := make(map[string]float64)
	for _, w := range words {
		tf[w]++
	}
	return tf
}

func makeSet(words []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}

func isAllPunctOrSpace(s string) bool {
	for _, r := range s {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) && !unicode.IsSymbol(r) {
			return false
		}
	}
	return true
}

func getVerdict(score float64) string {
	switch {
	case score >= 0.8:
		return "高度相似"
	case score >= 0.5:
		return "中度相似"
	case score >= 0.3:
		return "低度相似"
	default:
		return "无明显相似"
	}
}

func roundTo(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func countWords(text string) int {
	count := 0
	inWord := false
	for _, r := range text {
		if r > 0x4E00 && r < 0x9FFF {
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

func excerpt(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}

// ==================== MCP Server ====================

func main() {
	browserMgr := NewBrowserManager()
	defer browserMgr.Close()

	engine := NewTextCompareEngine()
	defer engine.Close()

	// 创建 MCP 服务器
	s := server.NewMCPServer(
		"webcompare-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// ======= Tool 1: compare_webpages =======
	compareWebpagesTool := mcp.NewTool("compare_webpages",
		mcp.WithDescription(
			"比对两个网页的内容相似性，生成详细的分析报告（Markdown 格式）。"+
				"自动抓取两个 URL 的正文内容，使用中文分词后计算余弦相似度、Jaccard 系数和 LCS 比率，"+
				"同时提取高频共有关键词和相似内容片段，给出综合评分和判定建议。"+
				"适用于网页内容侵权检测、文章抄袭比对、竞品内容分析等场景。"),
		mcp.WithString("url1",
			mcp.Required(),
			mcp.Description("第一个网页 URL（通常为原创内容页面）"),
		),
		mcp.WithString("url2",
			mcp.Required(),
			mcp.Description("第二个网页 URL（通常为疑似侵权/抄袭的页面）"),
		),
	)

	// ======= Tool 2: compare_texts_with_report =======
	compareTextsTool := mcp.NewTool("compare_texts_with_report",
		mcp.WithDescription(
			"比对两段文本的相似性，生成详细的分析报告（Markdown 格式）。"+
				"直接传入文本内容，无需提供 URL。使用中文分词后进行多维度相似度分析，"+
				"包含余弦相似度、Jaccard 系数、LCS 比率、高频共有词和相似片段。"+
				"适用于已有文本内容的相似性比对场景。"),
		mcp.WithString("text1",
			mcp.Required(),
			mcp.Description("第一段文本（通常为原创内容）"),
		),
		mcp.WithString("text2",
			mcp.Required(),
			mcp.Description("第二段文本（通常为疑似抄袭内容）"),
		),
		mcp.WithString("label1",
			mcp.Description("可选：第一段文本的来源标签，如 URL 或文章名"),
		),
		mcp.WithString("label2",
			mcp.Description("可选：第二段文本的来源标签，如 URL 或文章名"),
		),
	)

	// 注册 compare_webpages 处理函数
	s.AddTool(compareWebpagesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url1, err := request.RequireString("url1")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少 url1 参数"), nil
		}
		url2, err := request.RequireString("url2")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少 url2 参数"), nil
		}

		startTime := time.Now()

		// 并发抓取两个网页
		type fetchResult struct {
			title   string
			content string
			err     error
		}

		ch1 := make(chan fetchResult, 1)
		ch2 := make(chan fetchResult, 1)

		go func() {
			t, c, e := browserMgr.FetchPage(url1)
			ch1 <- fetchResult{t, c, e}
		}()
		go func() {
			t, c, e := browserMgr.FetchPage(url2)
			ch2 <- fetchResult{t, c, e}
		}()

		r1 := <-ch1
		r2 := <-ch2

		if r1.err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("抓取页面1失败: %v", r1.err)), nil
		}
		if r2.err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("抓取页面2失败: %v", r2.err)), nil
		}

		if r1.content == "" {
			return mcp.NewToolResultError("页面1未能提取到有效内容"), nil
		}
		if r2.content == "" {
			return mcp.NewToolResultError("页面2未能提取到有效内容"), nil
		}

		// 文本比对
		similarity := engine.CompareTexts(r1.content, r2.content)
		duration := time.Since(startTime).Milliseconds()

		result := &WebCompareResult{
			Page1: PageInfo{
				URL:       url1,
				Title:     r1.title,
				WordCount: countWords(r1.content),
				Excerpt:   excerpt(r1.content, 200),
			},
			Page2: PageInfo{
				URL:       url2,
				Title:     r2.title,
				WordCount: countWords(r2.content),
				Excerpt:   excerpt(r2.content, 200),
			},
			Similarity: similarity,
			ComparedAt: time.Now().Format("2006-01-02 15:04:05"),
			DurationMs: duration,
		}

		// 生成 Markdown 报告
		result.Report = generateReport(result)

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	// 注册 compare_texts_with_report 处理函数
	s.AddTool(compareTextsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text1, err := request.RequireString("text1")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少 text1 参数"), nil
		}
		text2, err := request.RequireString("text2")
		if err != nil {
			return mcp.NewToolResultError("错误: 缺少 text2 参数"), nil
		}

		if text1 == "" || text2 == "" {
			return mcp.NewToolResultError("错误: text1 和 text2 都不能为空"), nil
		}

		label1 := "文本 A"
		label2 := "文本 B"
		if l, ok := request.GetArguments()["label1"].(string); ok && l != "" {
			label1 = l
		}
		if l, ok := request.GetArguments()["label2"].(string); ok && l != "" {
			label2 = l
		}

		startTime := time.Now()

		similarity := engine.CompareTexts(text1, text2)
		duration := time.Since(startTime).Milliseconds()

		result := &WebCompareResult{
			Page1: PageInfo{
				URL:       label1,
				Title:     label1,
				WordCount: countWords(text1),
				Excerpt:   excerpt(text1, 200),
			},
			Page2: PageInfo{
				URL:       label2,
				Title:     label2,
				WordCount: countWords(text2),
				Excerpt:   excerpt(text2, 200),
			},
			Similarity: similarity,
			ComparedAt: time.Now().Format("2006-01-02 15:04:05"),
			DurationMs: duration,
		}

		result.Report = generateReport(result)

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	// 启动 stdio 服务器
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
