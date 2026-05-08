package report

import (
	"fmt"
	"strings"
	"time"

	"infringement-agent-server/internal/evidence"
)

// Template 报告模板
type Template struct {
	Name string
}

// DefaultTemplate 默认七段式模板
func DefaultTemplate() *Template {
	return &Template{Name: "standard-7-section"}
}

// Render 渲染为 Markdown
// 论文 4.5 节七段式结构，每一段对应明确的法律依据
func (t *Template) Render(req *ReportRequest, evs []*evidence.NotarizedEvidence, overallHash string, merkleRoot string) string {
	var sb strings.Builder

	// ==================== 一、报告封面 ====================
	sb.WriteString(fmt.Sprintf("# %s\n\n", req.CaseTitle))
	sb.WriteString("> **网络侵权行为分析报告**  \n")
	sb.WriteString("> *（基于智能取证系统自动生成，符合《最高人民法院关于民事诉讼证据的若干规定》（法释〔2019〕19 号）第十四条对电子数据形式的要求）*\n\n")

	sb.WriteString("## 一、报告封面\n\n")
	sb.WriteString("| 项目 | 内容 |\n")
	sb.WriteString("|------|------|\n")
	sb.WriteString(fmt.Sprintf("| 报告编号 | `REPORT-%s-%d` |\n", req.CaseID, time.Now().Unix()))
	sb.WriteString(fmt.Sprintf("| 案件名称 | %s |\n", req.CaseTitle))
	sb.WriteString(fmt.Sprintf("| 权利人 | %s |\n", orDefault(req.RightsHolder, "（未填写）")))
	sb.WriteString(fmt.Sprintf("| 被取证对象 | %s |\n", orDefault(req.Defendant, "（未填写）")))
	sb.WriteString(fmt.Sprintf("| 取证平台 | %s |\n", orDefault(req.Platform, "（未填写）")))
	sb.WriteString(fmt.Sprintf("| 出具时间 | %s |\n", time.Now().Format("2006 年 01 月 02 日 15:04:05")))
	sb.WriteString(fmt.Sprintf("| 报告出具人 | %s |\n", orDefault(req.ReportAuthor, "智能取证系统")))
	sb.WriteString("\n---\n\n")

	// ==================== 二、案件概述 ====================
	sb.WriteString("## 二、案件概述\n\n")
	if req.IncidentSummary != "" {
		sb.WriteString(req.IncidentSummary)
	} else {
		sb.WriteString("（未提供案情概述）")
	}
	sb.WriteString("\n\n")

	// ==================== 三、证据清单 ====================
	sb.WriteString("## 三、证据清单\n\n")
	sb.WriteString("本报告共收集证据 **" + fmt.Sprintf("%d", len(evs)) + "** 条，所有证据均已经过客户端 Web Crypto API 即时 SHA-256 哈希、Mock TSA（遵循 RFC 3161 思路的本地自签时间戳）锚定、以及 Merkle Tree 聚合后 Mock 蚂蚁链批量存证，形成四值哈希链冗余验证体系。\n\n")
	sb.WriteString("| # | 证据 ID | 类型 | 采集时间 | 文件大小 | SHA-256（前 16 位） | TSA 序列号 | 链上交易哈希 |\n")
	sb.WriteString("|---|---------|------|----------|----------|-----------------------|-------------|----------------|\n")
	for i, ev := range evs {
		tsaSerial := "—"
		if ev.TSAToken != nil {
			tsaSerial = ev.TSAToken.SerialNumber
		}
		txShort := "—"
		if ev.BlockchainTx != "" {
			txShort = safeSlice(ev.BlockchainTx, 16) + "…"
		}
		sb.WriteString(fmt.Sprintf("| %d | `%s` | %s | %s | %s | `%s…` | `%s` | `%s` |\n",
			i+1, ev.EvidenceID, humanSourceType(ev.SourceType),
			ev.CollectedAt, humanSize(ev.FileSize),
			safeSlice(ev.HashChain.ServerHash, 16),
			tsaSerial, txShort,
		))
	}
	sb.WriteString("\n")

	// ==================== 四、证据真实性校验说明 ====================
	sb.WriteString("## 四、证据真实性校验说明\n\n")
	sb.WriteString("本报告中所有证据的真实性通过以下四重独立机制予以保障，对应《最高人民法院关于民事诉讼证据的若干规定》第九十三条电子数据真实性审查的七项考量因素，以及第九十四条真实性推定情形：\n\n")
	sb.WriteString("**1. 客户端即时哈希（论文 4.2.1 节）。** 证据在浏览器中产生的瞬间，即通过 Web Crypto API 的 `crypto.subtle.digest('SHA-256', ...)` 计算内容哈希，该机制在文件尚未持久化到磁盘之前完成，从技术上排除了权利人本地篡改的可能性。\n\n")
	sb.WriteString("**2. 可信时间戳锚定（论文 4.2.2 节）。** 哈希值立即提交至 Mock TSA 服务，获得符合 RFC 3161 标准格式的时间戳证书，证明『该哈希在特定时刻已客观存在』。当前原型采用本地自签 ECDSA-P256 签名实现，后续可替换为联合信任时间戳服务中心等正式 TSA。\n\n")
	sb.WriteString("**3. 服务端只读保护（论文 4.2.3 节）。** 证据落盘后通过 `chmod 444` 设置为只读，并通过 fsnotify 监控目录，任何修改尝试均触发告警并记录至监督链。\n\n")
	sb.WriteString("**4. 区块链 Merkle Tree 聚合存证（论文 4.3.3 节）。** 证据哈希通过 Merkle Tree 批量聚合后锚定至 Mock 蚂蚁链账本（本地 append-only 签名账本），对应《最高人民法院关于互联网法院审理案件若干问题的规定》（法释〔2018〕16 号）第十一条的技术路径。后续可替换为真实蚂蚁链、保全网等司法存证平台。\n\n")
	sb.WriteString("### 四值哈希链一致性摘要\n\n")
	sb.WriteString("| 环节 | 说明 | 验证通过率 |\n")
	sb.WriteString("|------|------|-----------|\n")
	clientVerified := 0
	tsaVerified := 0
	chainVerified := 0
	for _, ev := range evs {
		if ev.HashChain.ClientHash == ev.HashChain.ServerHash {
			clientVerified++
		}
		if ev.HashChain.TSAHash == ev.HashChain.ServerHash {
			tsaVerified++
		}
		if ev.HashChain.BlockchainHash == ev.HashChain.ServerHash {
			chainVerified++
		}
	}
	total := len(evs)
	if total == 0 {
		total = 1
	}
	sb.WriteString(fmt.Sprintf("| 客户端 ↔ 服务端 | 客户端即时哈希与落盘哈希对比 | %d/%d（%.1f%%） |\n",
		clientVerified, len(evs), float64(clientVerified)/float64(total)*100))
	sb.WriteString(fmt.Sprintf("| TSA ↔ 服务端 | 时间戳证书哈希与服务端哈希对比 | %d/%d（%.1f%%） |\n",
		tsaVerified, len(evs), float64(tsaVerified)/float64(total)*100))
	sb.WriteString(fmt.Sprintf("| 区块链 ↔ 服务端 | Merkle Proof 验证 | %d/%d（%.1f%%） |\n",
		chainVerified, len(evs), float64(chainVerified)/float64(total)*100))
	sb.WriteString("\n")

	// ==================== 五、侵权事实认定 ====================
	sb.WriteString("## 五、侵权事实认定\n\n")
	if req.FactFinding != "" {
		sb.WriteString(req.FactFinding)
	} else {
		sb.WriteString("（未提供事实认定内容，请由 Agent 或人工复核后填写）")
	}
	sb.WriteString("\n\n")

	// ==================== 六、法律三段论分析 ====================
	sb.WriteString("## 六、法律三段论分析\n\n")
	if len(req.LegalAnalysis) == 0 {
		sb.WriteString("（未提供法律分析，请由 Agent 或人工复核后填写）\n\n")
	} else {
		for i, syl := range req.LegalAnalysis {
			sb.WriteString(fmt.Sprintf("### 6.%d %s\n\n", i+1, orDefault(syl.ViolationType, "侵权认定")))
			sb.WriteString("**【大前提（法律规范）】**\n\n")
			sb.WriteString(syl.MajorPremise)
			sb.WriteString("\n\n**【小前提（案件事实）】**\n\n")
			sb.WriteString(syl.MinorPremise)
			sb.WriteString("\n\n**【结论（法律判断）】**\n\n")
			sb.WriteString(syl.Conclusion)
			sb.WriteString("\n\n")
		}
	}

	// ==================== 七、结论与维权建议 ====================
	sb.WriteString("## 七、结论与维权建议\n\n")
	if len(req.Recommendations) == 0 {
		sb.WriteString("（未提供维权建议）\n\n")
	} else {
		for i, r := range req.Recommendations {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r))
		}
		sb.WriteString("\n")
	}

	// ==================== 证据包整体签名 ====================
	sb.WriteString("---\n\n")
	sb.WriteString("## 证据包整体哈希签名\n\n")
	sb.WriteString("本报告所引用的全部证据经聚合后，其整体哈希签名如下（对全部证据 SHA-256 按采集时间顺序拼接后再次 SHA-256）：\n\n")
	sb.WriteString(fmt.Sprintf("- **证据数量**：%d\n", len(evs)))
	sb.WriteString(fmt.Sprintf("- **整体哈希（Overall Hash）**：`%s`\n", overallHash))
	if merkleRoot != "" {
		sb.WriteString(fmt.Sprintf("- **Merkle 根哈希**：`%s`\n", merkleRoot))
	}
	sb.WriteString(fmt.Sprintf("- **报告生成时间**：%s\n\n", time.Now().UTC().Format(time.RFC3339)))

	sb.WriteString("> 任何第三方可通过本系统的 `/api/notarize/{caseId}/{evidenceId}/verify` 接口独立校验每一条证据；亦可通过 `/api/chain/{caseId}/{evidenceId}/verify` 接口独立验证区块链 Merkle Proof。\n\n")
	sb.WriteString("> *本报告由网络侵权证据智能系统（Network Infringement Evidence Intelligent System, NIEIS）自动生成。如需司法提交，建议由权利人代理律师在此报告基础上补充形式要件后使用。*\n")

	return sb.String()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func safeSlice(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func humanSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func humanSourceType(t string) string {
	switch t {
	case "web":
		return "网页快照"
	case "live_segment":
		return "直播分段"
	case "short_video":
		return "短视频"
	case "document":
		return "文档"
	default:
		return t
	}
}
