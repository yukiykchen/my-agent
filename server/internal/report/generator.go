// Package report 法律规范化报告生成器（论文 4.5 节七段式模板）
// 七段式结构：
//   1. 报告封面
//   2. 案件概述
//   3. 证据清单（含 SHA-256 / TSA 时间戳 / 蚂蚁链交易哈希）
//   4. 证据真实性校验说明
//   5. 侵权事实认定
//   6. 法律三段论分析
//   7. 结论与维权建议
package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"infringement-agent-server/internal/evidence"
	"infringement-agent-server/internal/notary"
)

// ReportRequest 生成报告的请求参数
type ReportRequest struct {
	CaseID          string             `json:"caseId"`
	CaseTitle       string             `json:"caseTitle"`
	RightsHolder    string             `json:"rightsHolder"`    // 权利人
	Defendant       string             `json:"defendant"`       // 被取证对象
	Platform        string             `json:"platform"`        // 取证平台
	IncidentSummary string             `json:"incidentSummary"` // 案情概述
	FactFinding     string             `json:"factFinding"`     // 事实认定
	LegalAnalysis   []LegalSyllogism   `json:"legalAnalysis"`   // 法律三段论分析（可多段）
	Recommendations []string           `json:"recommendations"` // 维权建议
	ReportAuthor    string             `json:"reportAuthor"`    // 报告出具人
}

// LegalSyllogism 法律三段论
type LegalSyllogism struct {
	ViolationType string `json:"violationType"` // 侵权类型
	MajorPremise  string `json:"majorPremise"`  // 大前提：法条
	MinorPremise  string `json:"minorPremise"`  // 小前提：事实
	Conclusion    string `json:"conclusion"`    // 结论：法律判断
}

// GeneratedReport 生成的报告
type GeneratedReport struct {
	ReportID         string    `json:"reportId"`
	CaseID           string    `json:"caseId"`
	GeneratedAt      time.Time `json:"generatedAt"`
	Markdown         string    `json:"markdown"`
	MarkdownPath     string    `json:"markdownPath"`
	OverallHash      string    `json:"overallHash"`     // 证据包整体哈希签名
	EvidenceManifest []string  `json:"evidenceManifest"`
	EvidenceCount    int       `json:"evidenceCount"`
}

// Generator 报告生成器
type Generator struct {
	notarize *evidence.NotarizeService
	anchor   *evidence.ChainAnchor
	tpl      *Template
}

// NewGenerator 构造
func NewGenerator(notarize *evidence.NotarizeService, anchor *evidence.ChainAnchor) *Generator {
	return &Generator{
		notarize: notarize,
		anchor:   anchor,
		tpl:      DefaultTemplate(),
	}
}

// NotarizeService 返回底层固化服务（供工具层做前置检查）
func (g *Generator) NotarizeService() *evidence.NotarizeService {
	return g.notarize
}

// Generate 生成报告
func (g *Generator) Generate(req *ReportRequest) (*GeneratedReport, error) {
	if req.CaseID == "" {
		return nil, fmt.Errorf("caseId is required")
	}
	if req.CaseTitle == "" {
		req.CaseTitle = "网络侵权证据分析报告"
	}

	// 1. 查询全部固化证据
	evs, err := g.notarize.ListNotarized(req.CaseID)
	if err != nil {
		return nil, err
	}
	sort.Slice(evs, func(i, j int) bool {
		return evs[i].CollectedAt < evs[j].CollectedAt
	})

	// 2. 计算证据包整体哈希（对所有证据 serverHash 做 sha256，模拟"整体签名"）
	var leafHashes []string
	for _, ev := range evs {
		leafHashes = append(leafHashes, ev.HashChain.ServerHash)
	}
	overallHash := computeOverallHash(leafHashes)

	// 3. 构建 Merkle 根（若证据数 > 0）
	var merkleRoot string
	if len(leafHashes) > 0 {
		if tree, err := notary.BuildMerkleTree(leafHashes); err == nil {
			merkleRoot = tree.Root()
		}
	}

	// 4. 渲染 Markdown
	md := g.tpl.Render(req, evs, overallHash, merkleRoot)

	// 5. 持久化
	reportID := fmt.Sprintf("report_%d", time.Now().UnixMilli())
	caseDir := filepath.Join(g.notarize.DataDir(), req.CaseID, "reports")
	_ = os.MkdirAll(caseDir, 0755)
	mdPath := filepath.Join(caseDir, reportID+".md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return nil, err
	}

	// 6. 元数据
	meta := &GeneratedReport{
		ReportID:         reportID,
		CaseID:           req.CaseID,
		GeneratedAt:      time.Now().UTC(),
		Markdown:         md,
		MarkdownPath:     mdPath,
		OverallHash:      overallHash,
		EvidenceManifest: leafHashes,
		EvidenceCount:    len(evs),
	}
	metaPath := filepath.Join(caseDir, reportID+".meta.json")
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(metaPath, data, 0644)

	// 7. 监督链记录
	_ = g.notarize.AppendCustody(req.CaseID, notary.CustodyEvent{
		EventType: "create",
		Actor:     req.ReportAuthor,
		Remarks:   "法律规范化报告已生成：" + reportID,
		Meta: map[string]string{
			"reportId":      reportID,
			"overallHash":   overallHash,
			"merkleRoot":    merkleRoot,
			"evidenceCount": fmt.Sprintf("%d", len(evs)),
		},
	})

	return meta, nil
}

// computeOverallHash 对证据 hash 列表做一次 SHA-256（顺序相关）
func computeOverallHash(hashes []string) string {
	h := sha256.New()
	for _, s := range hashes {
		h.Write([]byte(strings.ToLower(s)))
		h.Write([]byte{'|'})
	}
	return hex.EncodeToString(h.Sum(nil))
}
