package tools

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"infringement-agent-server/internal/evidence"
	"infringement-agent-server/internal/models"
	"infringement-agent-server/internal/notary"
)

// RegisterEvidenceTools 注册证据链条相关 Agent 工具
// 这些工具让 Agent 能够直接驱动证据固化、Merkle 上链、查验等能力
func RegisterEvidenceTools(registry *Registry, notarize *evidence.NotarizeService, anchor *evidence.ChainAnchor) {

	// ============ evidence_list 列出案件所有固化证据 ============
	registry.Register(
		"evidence_list",
		"列出指定案件的所有已固化证据（含哈希、TSA 时间戳、上链状态）",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"caseId": {Type: "string", Description: "案件 ID"},
			},
			Required: []string{"caseId"},
		},
		func(args map[string]interface{}) (string, error) {
			if notarize == nil {
				return "", fmt.Errorf("notarize service not initialized")
			}
			caseID, _ := args["caseId"].(string)
			list, err := notarize.ListNotarized(caseID)
			if err != nil {
				return "", err
			}
			summary := make([]map[string]interface{}, len(list))
			for i, ev := range list {
				summary[i] = map[string]interface{}{
					"evidenceId":     ev.EvidenceID,
					"sourceType":     ev.SourceType,
					"filePath":       ev.FilePath,
					"fileSize":       ev.FileSize,
					"collectedAt":    ev.CollectedAt,
					"clientHash":     ev.HashChain.ClientHash,
					"serverHash":     ev.HashChain.ServerHash,
					"tsaHash":        ev.HashChain.TSAHash,
					"blockchainHash": ev.HashChain.BlockchainHash,
					"tsaSerial":      "",
					"status":         ev.IntegrityStatus,
					"blockchainTx":   ev.BlockchainTx,
				}
				if ev.TSAToken != nil {
					summary[i]["tsaSerial"] = ev.TSAToken.SerialNumber
					summary[i]["tsaGenTime"] = ev.TSAToken.GenTime
				}
			}
			data, _ := json.MarshalIndent(map[string]interface{}{
				"caseId":    caseID,
				"count":     len(list),
				"evidences": summary,
			}, "", "  ")
			return string(data), nil
		},
	)

	// ============ evidence_verify 校验证据完整性（四值哈希链 + TSA 签名）============
	registry.Register(
		"evidence_verify",
		"校验指定证据的完整性（四值哈希链 + TSA 签名 + 区块链 Merkle Proof）",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"caseId":     {Type: "string", Description: "案件 ID"},
				"evidenceId": {Type: "string", Description: "证据 ID"},
			},
			Required: []string{"caseId", "evidenceId"},
		},
		func(args map[string]interface{}) (string, error) {
			caseID, _ := args["caseId"].(string)
			evID, _ := args["evidenceId"].(string)

			rec, result, err := notarize.VerifyIntegrity(caseID, evID)
			if err != nil {
				return "", err
			}

			out := map[string]interface{}{
				"evidenceId": evID,
				"caseId":     caseID,
				"hashChain":  rec.HashChain,
				"verify":     result,
			}

			if anchor != nil && rec.BlockchainTx != "" {
				ok, reason, _ := anchor.VerifyEvidenceOnChain(caseID, evID)
				out["blockchain"] = map[string]interface{}{
					"onChain": ok,
					"reason":  reason,
				}
			}

			data, _ := json.MarshalIndent(out, "", "  ")
			return string(data), nil
		},
	)

	// ============ evidence_anchor 批量上链 ============
	registry.Register(
		"evidence_anchor",
		"将案件中尚未上链的证据通过 Merkle Tree 批量聚合上链（蚂蚁链 Mock）",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"caseId":      {Type: "string", Description: "案件 ID"},
				"evidenceIds": {Type: "array", Description: "可选，指定上链证据 ID 列表；不填则上链所有未上链证据"},
			},
			Required: []string{"caseId"},
		},
		func(args map[string]interface{}) (string, error) {
			if anchor == nil {
				return "", fmt.Errorf("chain anchor not initialized")
			}
			caseID, _ := args["caseId"].(string)

			var ids []string
			if raw, ok := args["evidenceIds"].([]interface{}); ok {
				for _, r := range raw {
					if s, ok := r.(string); ok {
						ids = append(ids, s)
					}
				}
			}

			var batch *evidence.AnchorBatch
			var err error
			if len(ids) > 0 {
				batch, err = anchor.AnchorSpecificEvidences(caseID, ids)
			} else {
				batch, err = anchor.AnchorPendingEvidences(caseID)
			}
			if err != nil {
				return "", err
			}

			data, _ := json.MarshalIndent(map[string]interface{}{
				"batchId":       batch.BatchID,
				"merkleRoot":    batch.MerkleRoot,
				"txHash":        batch.Tx.TxHash,
				"blockHeight":   batch.Tx.BlockHeight,
				"explorerUrl":   batch.Tx.ChainURL,
				"evidenceCount": batch.EvidenceCnt,
				"anchoredAt":    batch.AnchoredAt,
			}, "", "  ")
			return string(data), nil
		},
	)

	// ============ custody_list 查看监督链审计日志 ============
	registry.Register(
		"custody_list",
		"查看案件的监督链（Chain of Custody）审计日志",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"caseId": {Type: "string", Description: "案件 ID"},
			},
			Required: []string{"caseId"},
		},
		func(args map[string]interface{}) (string, error) {
			caseID, _ := args["caseId"].(string)
			events, err := notarize.ListCustody(caseID)
			if err != nil {
				return "", err
			}
			data, _ := json.MarshalIndent(map[string]interface{}{
				"caseId": caseID,
				"count":  len(events),
				"events": events,
			}, "", "  ")
			return string(data), nil
		},
	)

	// ============ asr_mock Whisper ASR 模拟（论文 4.1.2）============
	registry.Register(
		"asr_transcribe",
		"对直播分段视频进行语音转文字（ASR）分析。当前为 Mock 实现，真实部署可替换为 Whisper HTTP 服务。",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"caseId":     {Type: "string", Description: "案件 ID"},
				"evidenceId": {Type: "string", Description: "证据 ID（live_segment）"},
			},
			Required: []string{"caseId", "evidenceId"},
		},
		func(args map[string]interface{}) (string, error) {
			caseID, _ := args["caseId"].(string)
			evID, _ := args["evidenceId"].(string)
			rec, err := notarize.GetNotarized(caseID, evID)
			if err != nil {
				return "", err
			}
			result := mockASR(rec)
			data, _ := json.MarshalIndent(result, "", "  ")
			return string(data), nil
		},
	)

	// ============ ocr_mock PaddleOCR 模拟 ============
	registry.Register(
		"ocr_recognize",
		"对直播分段关键帧进行 OCR 文字识别。当前为 Mock 实现，真实部署可替换为 PaddleOCR HTTP 服务。",
		models.FunctionParams{
			Type: "object",
			Properties: map[string]models.PropertyDefine{
				"caseId":     {Type: "string", Description: "案件 ID"},
				"evidenceId": {Type: "string", Description: "证据 ID"},
			},
			Required: []string{"caseId", "evidenceId"},
		},
		func(args map[string]interface{}) (string, error) {
			caseID, _ := args["caseId"].(string)
			evID, _ := args["evidenceId"].(string)
			rec, err := notarize.GetNotarized(caseID, evID)
			if err != nil {
				return "", err
			}
			result := mockOCR(rec)
			data, _ := json.MarshalIndent(result, "", "  ")
			return string(data), nil
		},
	)
}

// mockASR 基于证据 hash 的确定性 Mock，使同一证据多次调用结果一致
func mockASR(rec *evidence.NotarizedEvidence) map[string]interface{} {
	// 使用服务端哈希作为随机种子，保证可复现
	seed := hashToSeed(rec.HashChain.ServerHash)
	r := rand.New(rand.NewSource(seed))

	scripts := []string{
		"这款耳机和华为 FreeBuds Pro 3 是同一生产线出来的，音质完全一样，华为卖 1299 我们只要 199！",
		"家人们快下单，这个配置和华为 Mate 60 Pro 一模一样，不用花六千多了！",
		"这个手机壳是原厂同款，专为华为 Pura 70 定制的，用料扎实。",
		"OK 家人们，今晚只要 99，数量有限，错过不再有。",
		"这款式和苹果 AirPods 是同款供应链的，大家可以放心买。",
	}
	idx := r.Intn(len(scripts))
	text := scripts[idx]

	return map[string]interface{}{
		"engine":     "whisper-mock-v1",
		"evidenceId": rec.EvidenceID,
		"language":   "zh",
		"durationMs": rec.Meta["durationMs"],
		"segments": []map[string]interface{}{
			{
				"start": 0.0,
				"end":   4.8,
				"text":  text,
				"confidence": 0.95 + r.Float64()*0.04,
			},
		},
		"fullText": text,
		"keywordsHit": detectKeywords(text),
	}
}

// mockOCR
func mockOCR(rec *evidence.NotarizedEvidence) map[string]interface{} {
	seed := hashToSeed(rec.HashChain.ServerHash)
	r := rand.New(rand.NewSource(seed))

	scenes := [][]string{
		{"HUAWEI", "FreeBuds Pro 3", "主动降噪", "¥199"},
		{"HUAWEI", "Pura 70", "超光变镜头", "原厂同款"},
		{"华为", "Mate 60 Pro", "5G", "旗舰配置"},
		{"Apple", "AirPods", "Pro 2", "¥999"},
	}
	idx := r.Intn(len(scenes))
	words := scenes[idx]

	items := make([]map[string]interface{}, len(words))
	for i, w := range words {
		items[i] = map[string]interface{}{
			"text":       w,
			"confidence": 0.90 + r.Float64()*0.09,
			"bbox":       []int{100 + i*20, 100 + i*40, 300, 160 + i*40},
		}
	}

	return map[string]interface{}{
		"engine":      "paddleocr-pp-ocrv4-mock",
		"evidenceId":  rec.EvidenceID,
		"items":       items,
		"keywordsHit": detectKeywords(strings.Join(words, " ")),
	}
}

// detectKeywords 识别侵权敏感关键词
func detectKeywords(text string) []map[string]string {
	brands := map[string]string{
		"华为":         "HUAWEI 商标",
		"HUAWEI":     "HUAWEI 商标",
		"FreeBuds":   "华为 FreeBuds 系列注册商标",
		"Pura":       "华为 Pura 系列注册商标",
		"Mate":       "华为 Mate 系列注册商标",
		"原厂同款":       "虚假宣传疑似",
		"同一生产线":     "虚假宣传疑似",
		"完全一样":      "虚假宣传疑似",
	}
	var hits []map[string]string
	for k, v := range brands {
		if strings.Contains(text, k) {
			hits = append(hits, map[string]string{"keyword": k, "category": v})
		}
	}
	return hits
}

// hashToSeed 将 hex 哈希转为确定性 seed
func hashToSeed(hexHash string) int64 {
	var seed int64 = 0
	for i := 0; i < len(hexHash) && i < 16; i++ {
		c := hexHash[i]
		var d int64
		if c >= '0' && c <= '9' {
			d = int64(c - '0')
		} else if c >= 'a' && c <= 'f' {
			d = int64(c-'a') + 10
		}
		seed = seed*16 + d
	}
	if seed == 0 {
		seed = 1
	}
	return seed
}

// getEvidenceFileAbs 辅助：取绝对路径
func getEvidenceFileAbs(notarize *evidence.NotarizeService, caseID, relPath string) string {
	return filepath.Join(notarize.DataDir(), caseID, relPath)
}

// ReadEvidenceContent 辅助：读取证据文件字节
func ReadEvidenceContent(notarize *evidence.NotarizeService, caseID, relPath string) ([]byte, error) {
	_ = notary.ComputeBytesHash // 保留 import
	return os.ReadFile(getEvidenceFileAbs(notarize, caseID, relPath))
}
