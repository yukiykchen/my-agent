package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yanyiwu/gojieba"
)

// ==================== Text Compare Result ====================

type TextCompareResult struct {
	OverallScore     float64 `json:"overallScore"`
	CosineSimilarity float64 `json:"cosineSimilarity"`
	JaccardIndex     float64 `json:"jaccardIndex"`
	LCSRatio         float64 `json:"lcsRatio"`
	Verdict          string  `json:"verdict"`
	Details          string  `json:"details"`
	Text1Length      int     `json:"text1Length"`
	Text2Length      int     `json:"text2Length"`
	CommonWords      int     `json:"commonWords"`
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

func (e *TextCompareEngine) Compare(text1, text2 string) *TextCompareResult {
	// 分词
	words1 := e.tokenize(text1)
	words2 := e.tokenize(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return &TextCompareResult{
			OverallScore:     0,
			CosineSimilarity: 0,
			JaccardIndex:     0,
			LCSRatio:         0,
			Verdict:          "无法比较",
			Details:          "一段或两段文本为空或无有效内容",
			Text1Length:      len([]rune(text1)),
			Text2Length:      len([]rune(text2)),
			CommonWords:      0,
		}
	}

	// 计算各维度相似度
	cosine := e.cosineSimilarity(words1, words2)
	jaccard := e.jaccardSimilarity(words1, words2)

	// LCS 对超长文本降级为分词级别
	var lcsRatio float64
	if len([]rune(text1)) > 10000 || len([]rune(text2)) > 10000 {
		// 分词级 LCS
		lcsRatio = e.lcsRatioWords(words1, words2)
	} else {
		// 字符级 LCS
		lcsRatio = e.lcsRatioChars(text1, text2)
	}

	// 计算共同词汇数
	set1 := makeSet(words1)
	set2 := makeSet(words2)
	commonWords := 0
	for w := range set1 {
		if _, ok := set2[w]; ok {
			commonWords++
		}
	}

	// 综合评分（加权平均）
	overallScore := cosine*0.4 + jaccard*0.3 + lcsRatio*0.3

	// 判定结论
	verdict := getVerdict(overallScore)

	// 生成详细说明
	details := fmt.Sprintf(
		"文本1共%d字（%d个词），文本2共%d字（%d个词）。"+
			"共有%d个相同词汇。"+
			"余弦相似度%.2f（衡量语义方向一致性），"+
			"Jaccard系数%.2f（衡量词汇重叠度），"+
			"LCS比率%.2f（衡量内容连续相似性）。"+
			"综合评分%.2f，%s。",
		len([]rune(text1)), len(words1),
		len([]rune(text2)), len(words2),
		commonWords,
		cosine, jaccard, lcsRatio,
		overallScore, verdictExplanation(overallScore),
	)

	return &TextCompareResult{
		OverallScore:     roundTo(overallScore, 4),
		CosineSimilarity: roundTo(cosine, 4),
		JaccardIndex:     roundTo(jaccard, 4),
		LCSRatio:         roundTo(lcsRatio, 4),
		Verdict:          verdict,
		Details:          details,
		Text1Length:      len([]rune(text1)),
		Text2Length:      len([]rune(text2)),
		CommonWords:      commonWords,
	}
}

func (e *TextCompareEngine) tokenize(text string) []string {
	words := e.jieba.Cut(text, true)
	// 过滤停用词和标点
	var filtered []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		// 跳过纯标点和空白
		if isAllPunctOrSpace(w) {
			continue
		}
		filtered = append(filtered, w)
	}
	return filtered
}

func (e *TextCompareEngine) cosineSimilarity(words1, words2 []string) float64 {
	// 构建词频向量
	tf1 := termFrequency(words1)
	tf2 := termFrequency(words2)

	// 计算点积和模长
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

func (e *TextCompareEngine) lcsRatioChars(text1, text2 string) float64 {
	runes1 := []rune(text1)
	runes2 := []rune(text2)

	m := len(runes1)
	n := len(runes2)

	if m == 0 || n == 0 {
		return 0
	}

	// 优化内存：只用两行
	prev := make([]int, n+1)
	curr := make([]int, n+1)

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if runes1[i-1] == runes2[j-1] {
				curr[j] = prev[j-1] + 1
			} else {
				curr[j] = max(prev[j], curr[j-1])
			}
		}
		prev, curr = curr, make([]int, n+1)
	}

	lcsLen := prev[n]
	maxLen := max(m, n)
	return float64(lcsLen) / float64(maxLen)
}

func (e *TextCompareEngine) lcsRatioWords(words1, words2 []string) float64 {
	m := len(words1)
	n := len(words2)

	if m == 0 || n == 0 {
		return 0
	}

	// 限制计算规模
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
				curr[j] = max(prev[j], curr[j-1])
			}
		}
		prev, curr = curr, make([]int, n+1)
	}

	lcsLen := prev[n]
	maxLen := max(m, n)
	return float64(lcsLen) / float64(maxLen)
}

// ==================== Helper Functions ====================

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

func verdictExplanation(score float64) string {
	switch {
	case score >= 0.8:
		return "两段文本高度相似，存在较大的抄袭/侵权嫌疑，建议进一步进行人工审核和法律分析"
	case score >= 0.5:
		return "两段文本存在一定程度的相似性，可能存在部分借鉴或引用，建议结合具体内容进一步分析"
	case score >= 0.3:
		return "两段文本有少量相似内容，可能是巧合或共同引用了公共素材"
	default:
		return "两段文本差异较大，未发现明显相似性"
	}
}

func roundTo(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ==================== MCP Server ====================

func main() {
	engine := NewTextCompareEngine()
	defer engine.Close()

	// 创建 MCP 服务器
	s := server.NewMCPServer(
		"textcompare-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// 定义文本比较工具
	compareTool := mcp.NewTool("compare_texts",
		mcp.WithDescription("比较两段文本的相似度。使用中文分词后计算余弦相似度、Jaccard系数和最长公共子序列(LCS)比率三个维度的相似度，并给出综合评分和侵权判定建议。适用于判断文本是否存在抄袭或侵权。"),
		mcp.WithString("text1",
			mcp.Required(),
			mcp.Description("第一段文本（通常为原创/原始内容）"),
		),
		mcp.WithString("text2",
			mcp.Required(),
			mcp.Description("第二段文本（通常为疑似侵权内容）"),
		),
	)

	// 注册工具处理函数
	s.AddTool(compareTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text1, err := request.RequireString("text1")
		if err != nil {
			return mcp.NewToolResultError("错误: text1 不能为空"), nil
		}
		text2, err := request.RequireString("text2")
		if err != nil {
			return mcp.NewToolResultError("错误: text2 不能为空"), nil
		}

		if text1 == "" || text2 == "" {
			return mcp.NewToolResultError("错误: text1 和 text2 都不能为空"), nil
		}

		result := engine.Compare(text1, text2)
		resultJSON, _ := json.Marshal(result)

		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	// 启动 stdio 服务器
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
