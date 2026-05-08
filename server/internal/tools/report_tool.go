package tools

import (
	"encoding/json"
	"fmt"

	"infringement-agent-server/internal/models"
	"infringement-agent-server/internal/report"
)

// RegisterReportTool 注册法律规范化报告生成工具（论文 4.5 节）
func RegisterReportTool(registry *Registry, gen *report.Generator) {
	if gen == nil {
		return
	}

	registry.Register(
		"report_generate",
		"生成符合法律文书规范的《网络侵权行为分析报告》（七段式结构，含证据清单、真实性校验说明、法律三段论分析等）。调用前必须先通过 evidence_list 确认案件下有已固化证据，否则本工具将返回错误。",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"caseId":          {Type: "string", Description: "案件 ID（必填）。报告中的证据清单、真实性校验、哈希值、时间戳、链上交易哈希将全部从该案件的真实固化记录中自动读取，请勿编造。"},
				"caseTitle":       {Type: "string", Description: "案件名称，如'淘宝平台华为 Pura 70 仿冒商品页取证报告'"},
				"rightsHolder":    {Type: "string", Description: "权利人（如：华为技术有限公司）"},
				"defendant":       {Type: "string", Description: "被取证对象（如：淘宝店铺XX数码旗舰店）"},
				"platform":        {Type: "string", Description: "取证平台（如：淘宝/抖音直播）"},
				"incidentSummary": {Type: "string", Description: "案情概述：发生的侵权行为、时间、影响范围等"},
				"factFinding":     {Type: "string", Description: "侵权事实认定：基于采集到的证据归纳出的客观事实"},
				"legalAnalysis":   {Type: "array", Description: "法律三段论分析数组，每项包含 violationType/majorPremise/minorPremise/conclusion 四个字段"},
				"recommendations": {Type: "array", Description: "维权建议字符串数组"},
				"reportAuthor":    {Type: "string", Description: "报告出具人（如：智能取证系统 / 律师姓名）"},
			},
			Required: []string{"caseId"},
		},
		func(args map[string]interface{}) (string, error) {
			caseID := asString(args["caseId"])
			if caseID == "" {
				return "", fmt.Errorf("caseId 不能为空")
			}

			// 前置检查：必须有已固化证据，否则拒绝生成，防止模型瞎编
			if gen == nil {
				return "", fmt.Errorf("报告生成器未初始化")
			}
			evs, err := gen.NotarizeService().ListNotarized(caseID)
			if err != nil {
				return "", fmt.Errorf("查询案件证据失败: %v", err)
			}
			if len(evs) == 0 {
				return "", fmt.Errorf("该案件下没有找到已固化的证据。请先用网页截图、直播取证或文件上传等方式采集证据并完成固化，再调用本工具生成报告。")
			}

			req := &report.ReportRequest{
				CaseID:          caseID,
				CaseTitle:       asString(args["caseTitle"]),
				RightsHolder:    asString(args["rightsHolder"]),
				Defendant:       asString(args["defendant"]),
				Platform:        asString(args["platform"]),
				IncidentSummary: asString(args["incidentSummary"]),
				FactFinding:     asString(args["factFinding"]),
				ReportAuthor:    asString(args["reportAuthor"]),
			}

			// 维权建议
			if recs, ok := args["recommendations"].([]interface{}); ok {
				for _, r := range recs {
					if s, ok := r.(string); ok {
						req.Recommendations = append(req.Recommendations, s)
					}
				}
			}

			// 法律三段论
			if analyses, ok := args["legalAnalysis"].([]interface{}); ok {
				for _, a := range analyses {
					if m, ok := a.(map[string]interface{}); ok {
						req.LegalAnalysis = append(req.LegalAnalysis, report.LegalSyllogism{
							ViolationType: asString(m["violationType"]),
							MajorPremise:  asString(m["majorPremise"]),
							MinorPremise:  asString(m["minorPremise"]),
							Conclusion:    asString(m["conclusion"]),
						})
					}
				}
			}

			out, err := gen.Generate(req)
			if err != nil {
				return "", err
			}

			data, _ := json.MarshalIndent(map[string]interface{}{
				"reportId":      out.ReportID,
				"caseId":        out.CaseID,
				"generatedAt":   out.GeneratedAt,
				"markdownPath":  out.MarkdownPath,
				"overallHash":   out.OverallHash,
				"evidenceCount": out.EvidenceCount,
				"preview":       truncate(out.Markdown, 500),
			}, "", "  ")
			return string(data), nil
		},
	)
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n\n…（省略，完整内容见 markdownPath）"
}
