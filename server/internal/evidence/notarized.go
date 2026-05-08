package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"infringement-agent-server/internal/notary"
)

// NotarizedEvidence 经固化的证据记录（论文 4.2 节）
// 存储在 cases/{caseId}/notarized/{evidenceId}.json
type NotarizedEvidence struct {
	EvidenceID      string                    `json:"evidenceId"`
	CaseID          string                    `json:"caseId"`
	SourceType      string                    `json:"sourceType"`        // web | live_segment | short_video | document
	FilePath        string                    `json:"filePath"`          // 相对 case 目录
	FileSize        int64                     `json:"fileSize"`
	CollectedAt     string                    `json:"collectedAt"`
	FixedAt         string                    `json:"fixedAt"`           // 固化完成时间
	HashChain       notary.HashChain          `json:"hashChain"`
	TSAToken        *notary.TimestampToken    `json:"tsaToken,omitempty"`
	TSAFile         string                    `json:"tsaFile,omitempty"` // 时间戳证书文件相对路径
	BlockchainTx    string                    `json:"blockchainTx,omitempty"`
	MerkleRoot      string                    `json:"merkleRoot,omitempty"`
	MerkleProof     []string                  `json:"merkleProof,omitempty"`
	IntegrityStatus string                    `json:"integrityStatus"`   // verified | compromised | pending
	Collector       string                    `json:"collector,omitempty"`
	Meta            map[string]string         `json:"meta,omitempty"`
	FixationLatency int64                     `json:"fixationLatencyMs"` // 从收到上传到完成固化的毫秒数
}

// NotarizeService 固化服务（聚合 TSA + fsnotify + 监督链）
type NotarizeService struct {
	store     *Store
	tsa       *notary.TSAService
	watcher   *notary.FileWatcher
	custody   map[string]*notary.CustodyLog // caseID -> log
}

// NewNotarizeService 创建固化服务
func NewNotarizeService(store *Store) (*NotarizeService, error) {
	tsa, err := notary.NewTSAService(filepath.Join(store.dataDir, ".ca"))
	if err != nil {
		return nil, err
	}
	w, err := notary.NewFileWatcher()
	if err != nil {
		return nil, err
	}

	svc := &NotarizeService{
		store:   store,
		tsa:     tsa,
		watcher: w,
		custody: make(map[string]*notary.CustodyLog),
	}

	w.SetOnEvent(func(ev notary.TamperEvent) {
		// 过滤系统产生的写入事件（监督链日志、案件元数据、TSA/链目录等）
		// 只关心真正的证据文件变更，避免自环告警
		p := filepath.ToSlash(ev.Path)
		if shouldSkipWatchEvent(p) {
			return
		}
		// 尝试提取 caseID：目录结构为 dataDir/caseID/...
		caseID := svc.extractCaseID(ev.Path)
		if caseID == "" {
			return
		}
		log, _ := svc.getCustody(caseID)
		if log != nil {
			_ = log.Append(notary.CustodyEvent{
				EventType:  "alarm",
				Actor:      "system",
				TargetPath: ev.Path,
				Remarks:    "fsnotify 监测到证据目录下的文件变更事件：" + ev.EventType,
			})
		}
	})
	w.Start()

	// 监听整个证据根目录
	if err := w.WatchDir(store.dataDir); err != nil {
		// 非致命
	}
	return svc, nil
}

// extractCaseID 从路径反推 caseID
func (s *NotarizeService) extractCaseID(path string) string {
	absData, _ := filepath.Abs(s.store.dataDir)
	absPath, _ := filepath.Abs(path)
	if len(absPath) <= len(absData) {
		return ""
	}
	rel := absPath[len(absData):]
	rel = filepath.ToSlash(rel)
	for len(rel) > 0 && rel[0] == '/' {
		rel = rel[1:]
	}
	// 取第一段作为 caseID
	for i := 0; i < len(rel); i++ {
		if rel[i] == '/' {
			return rel[:i]
		}
	}
	return rel
}

func (s *NotarizeService) getCustody(caseID string) (*notary.CustodyLog, error) {
	if log, ok := s.custody[caseID]; ok {
		return log, nil
	}
	caseDir := filepath.Join(s.store.dataDir, caseID)
	log, err := notary.NewCustodyLog(caseDir, caseID)
	if err != nil {
		return nil, err
	}
	s.custody[caseID] = log
	return log, nil
}

// FixationRequest 固化请求
type FixationRequest struct {
	CaseID     string            `json:"caseId"`
	EvidenceID string            `json:"evidenceId"` // 可空，自动生成
	SourceType string            `json:"sourceType"` // web | live_segment | short_video | document
	Filename   string            `json:"filename"`   // 期望的文件名
	ClientHash string            `json:"clientHash"` // 客户端预计算的 sha256（必填，论文 4.2.1）
	Collector  string            `json:"collector,omitempty"`
	ClientIP   string            `json:"clientIp,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}

// Fixate 执行完整的即时固化流水线
// 输入文件字节；输出 NotarizedEvidence（已含 TSA 时间戳 + 服务端哈希 + 只读）
// 对应论文 4.2 节全流程
func (s *NotarizeService) Fixate(req FixationRequest, data []byte) (*NotarizedEvidence, error) {
	startTs := time.Now()

	if req.CaseID == "" {
		return nil, fmt.Errorf("caseId is required")
	}
	if req.ClientHash == "" {
		return nil, fmt.Errorf("clientHash is required: 客户端必须在落盘前提交即时哈希")
	}

	// 1. 服务端重算 hash
	serverHash := notary.ComputeBytesHash(data)
	if req.ClientHash != serverHash {
		return nil, fmt.Errorf("哈希校验失败：客户端 %s ≠ 服务端 %s，拒绝入库", req.ClientHash, serverHash)
	}

	// 2. 生成 evidenceId
	evID := req.EvidenceID
	if evID == "" {
		evID = fmt.Sprintf("ev_%d_%s", time.Now().UnixMilli(), shortRand(6))
	}

	// 3. 确认案件存在，必要时自动创建
	s.store.mu.Lock()
	c, ok := s.store.cases[req.CaseID]
	if !ok {
		// 自动创建一个案件
		now := time.Now()
		c = &Case{
			ID:        req.CaseID,
			Title:     "自动创建案件 " + req.CaseID,
			CreatedAt: now.Format(time.RFC3339),
			UpdatedAt: now.Format(time.RFC3339),
			Status:    "collecting",
			Evidences: []Evidence{},
		}
		s.store.cases[req.CaseID] = c
		_ = os.MkdirAll(filepath.Join(s.store.dataDir, req.CaseID), 0755)
	}
	s.store.mu.Unlock()

	// 4. 落盘
	caseDir := filepath.Join(s.store.dataDir, req.CaseID)
	subDir := filepath.Join(caseDir, sourceSubDir(req.SourceType))
	_ = os.MkdirAll(subDir, 0755)
	fileName := req.Filename
	if fileName == "" {
		fileName = evID + ".bin"
	}
	// 防止重名
	relPath := filepath.Join(sourceSubDir(req.SourceType), fileName)
	absPath := filepath.Join(caseDir, relPath)
	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return nil, fmt.Errorf("落盘失败: %w", err)
	}

	// 5. 申请 TSA 时间戳
	token, err := s.tsa.ApplyTimestamp(serverHash)
	if err != nil {
		return nil, fmt.Errorf("TSA 时间戳申请失败: %w", err)
	}
	tsaFileRel := filepath.Join("notarized", evID+".tsr.json")
	tsaFileAbs := filepath.Join(caseDir, tsaFileRel)
	if err := s.tsa.SaveTokenFile(token, tsaFileAbs); err != nil {
		return nil, fmt.Errorf("时间戳证书持久化失败: %w", err)
	}

	// 6. 设置只读保护
	if err := notary.SetReadOnly(absPath); err != nil {
		// 非致命
	}

	latency := time.Since(startTs).Milliseconds()

	// 7. 组装记录
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rec := &NotarizedEvidence{
		EvidenceID:  evID,
		CaseID:      req.CaseID,
		SourceType:  req.SourceType,
		FilePath:    relPath,
		FileSize:    int64(len(data)),
		CollectedAt: now,
		FixedAt:     now,
		HashChain: notary.HashChain{
			ClientHash: req.ClientHash,
			ServerHash: serverHash,
			TSAHash:    token.HashedMessage,
		},
		TSAToken:        token,
		TSAFile:         tsaFileRel,
		IntegrityStatus: "verified",
		Collector:       req.Collector,
		Meta:            req.Meta,
		FixationLatency: latency,
	}

	// 8. 持久化固化记录
	notarizedDir := filepath.Join(caseDir, "notarized")
	_ = os.MkdirAll(notarizedDir, 0755)
	recPath := filepath.Join(notarizedDir, evID+".json")
	recBytes, _ := json.MarshalIndent(rec, "", "  ")
	if err := os.WriteFile(recPath, recBytes, 0644); err != nil {
		return nil, err
	}

	// 9. 同步追加到监督链
	if log, _ := s.getCustody(req.CaseID); log != nil {
		_ = log.Append(notary.CustodyEvent{
			EventType:  "create",
			Actor:      req.Collector,
			ClientIP:   req.ClientIP,
			EvidenceID: evID,
			TargetPath: relPath,
			HashAfter:  serverHash,
			Remarks:    fmt.Sprintf("证据即时固化完成（%d ms）。TSA 序列号 %s。", latency, token.SerialNumber),
			Meta: map[string]string{
				"sourceType":   req.SourceType,
				"tsaSerial":    token.SerialNumber,
				"tsaGenTime":   token.GenTime,
				"clientHash":   req.ClientHash,
				"serverHash":   serverHash,
			},
		})
	}

	// 10. 同步更新原 Evidence 结构
	s.store.mu.Lock()
	c.Evidences = append(c.Evidences, Evidence{
		ID:          evID,
		URL:         req.Meta["url"],
		CollectedAt: now,
		TextContent: "",
		Metadata:    req.Meta,
		ContentHash: serverHash,
	})
	c.UpdatedAt = now
	_ = s.store.saveCase(c)
	s.store.mu.Unlock()

	return rec, nil
}

// GetNotarized 读取固化记录
func (s *NotarizeService) GetNotarized(caseID, evidenceID string) (*NotarizedEvidence, error) {
	recPath := filepath.Join(s.store.dataDir, caseID, "notarized", evidenceID+".json")
	data, err := os.ReadFile(recPath)
	if err != nil {
		return nil, err
	}
	var rec NotarizedEvidence
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// ListNotarized 列出案件全部固化记录
func (s *NotarizeService) ListNotarized(caseID string) ([]*NotarizedEvidence, error) {
	dir := filepath.Join(s.store.dataDir, caseID, "notarized")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*NotarizedEvidence
	for _, e := range entries {
		name := e.Name()
		if len(name) < 6 || name[len(name)-5:] != ".json" {
			continue
		}
		// 跳过 .tsr.json
		if len(name) > 10 && name[len(name)-9:] == ".tsr.json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var rec NotarizedEvidence
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		out = append(out, &rec)
	}
	return out, nil
}

// VerifyIntegrity 重新校验证据完整性
// 返回：校验后的哈希链 + 原始记录（状态字段会被更新）
func (s *NotarizeService) VerifyIntegrity(caseID, evidenceID string) (*NotarizedEvidence, notary.VerifyResult, error) {
	rec, err := s.GetNotarized(caseID, evidenceID)
	if err != nil {
		return nil, notary.VerifyResult{}, err
	}

	absPath := filepath.Join(s.store.dataDir, caseID, rec.FilePath)
	nowHash, err := notary.ComputeFileHash(absPath)
	if err != nil {
		rec.IntegrityStatus = "compromised"
		rec.HashChain.ServerHash = "FILE_MISSING"
		return rec, notary.VerifyResult{Consistent: false, Reason: "文件丢失: " + err.Error()}, nil
	}

	// 更新 ServerHash 为当前实际哈希
	chain := rec.HashChain
	chain.ServerHash = nowHash
	result := chain.Verify()

	// TSA 独立验证
	if rec.TSAToken != nil {
		if err := s.tsa.VerifyTimestamp(rec.TSAToken); err != nil {
			result.Consistent = false
			if result.Reason != "" {
				result.Reason += "；"
			}
			result.Reason += "TSA 签名验证失败: " + err.Error()
		}
	}

	if result.Consistent {
		rec.IntegrityStatus = "verified"
	} else {
		rec.IntegrityStatus = "compromised"
	}

	// 追加监督链
	if log, _ := s.getCustody(caseID); log != nil {
		_ = log.Append(notary.CustodyEvent{
			EventType:  "verify",
			Actor:      "system",
			EvidenceID: evidenceID,
			TargetPath: rec.FilePath,
			HashBefore: rec.HashChain.ClientHash,
			HashAfter:  nowHash,
			Remarks:    result.Reason,
		})
	}

	// 持久化更新
	rec.HashChain.ServerHash = nowHash
	recPath := filepath.Join(s.store.dataDir, caseID, "notarized", evidenceID+".json")
	recBytes, _ := json.MarshalIndent(rec, "", "  ")
	_ = os.WriteFile(recPath, recBytes, 0644)

	return rec, result, nil
}

// AppendCustody 对外暴露追加监督链事件
func (s *NotarizeService) AppendCustody(caseID string, ev notary.CustodyEvent) error {
	log, err := s.getCustody(caseID)
	if err != nil {
		return err
	}
	return log.Append(ev)
}

// ListCustody 列出监督链事件
func (s *NotarizeService) ListCustody(caseID string) ([]notary.CustodyEvent, error) {
	log, err := s.getCustody(caseID)
	if err != nil {
		return nil, err
	}
	return log.List()
}

// VerifyCustodyChain 校验监督链本身未被篡改
func (s *NotarizeService) VerifyCustodyChain(caseID string) (bool, string, error) {
	log, err := s.getCustody(caseID)
	if err != nil {
		return false, "", err
	}
	return log.Verify()
}

// UpdateBlockchainInfo 更新证据的区块链上链信息（批次 3 Merkle 服务调用）
func (s *NotarizeService) UpdateBlockchainInfo(caseID, evidenceID, tx, merkleRoot string, proof []string) error {
	rec, err := s.GetNotarized(caseID, evidenceID)
	if err != nil {
		return err
	}
	rec.BlockchainTx = tx
	rec.MerkleRoot = merkleRoot
	rec.MerkleProof = proof
	rec.HashChain.BlockchainHash = rec.HashChain.ServerHash // 成功上链即锚定

	recPath := filepath.Join(s.store.dataDir, caseID, "notarized", evidenceID+".json")
	data, _ := json.MarshalIndent(rec, "", "  ")
	if err := os.WriteFile(recPath, data, 0644); err != nil {
		return err
	}

	if log, _ := s.getCustody(caseID); log != nil {
		_ = log.Append(notary.CustodyEvent{
			EventType:  "transfer",
			Actor:      "system",
			EvidenceID: evidenceID,
			Remarks:    "证据已通过 Merkle Tree 聚合上链至蚂蚁链",
			Meta: map[string]string{
				"blockchainTx": tx,
				"merkleRoot":   merkleRoot,
			},
		})
	}
	return nil
}

// DataDir 公开访问数据目录（供 MCP 工具使用）
func (s *NotarizeService) DataDir() string { return s.store.dataDir }

// TSA 暴露 TSA 服务（仅用于独立验证测试）
func (s *NotarizeService) TSA() *notary.TSAService { return s.tsa }

func sourceSubDir(sourceType string) string {
	switch sourceType {
	case "live_segment":
		return "live"
	case "short_video":
		return "short"
	case "web":
		return "web"
	case "document":
		return "documents"
	default:
		return "misc"
	}
}

// shouldSkipWatchEvent 过滤系统/元数据文件的自环事件
// 仅保留真正的证据文件 / notarized 记录以外位置的改动告警
func shouldSkipWatchEvent(slashPath string) bool {
	// 监督链、CA、区块链账本、案件元数据都是系统 own 的，不应作为告警
	skipSuffixes := []string{
		"/custody.log.jsonl",
		"/case.json",
		"/HEAD",
		"/evidence_manifest.json",
	}
	for _, s := range skipSuffixes {
		if hasSuffix(slashPath, s) {
			return true
		}
	}
	// .ca / .chain 目录下的一切
	skipContains := []string{
		"/.ca/",
		"/.chain/",
		"/notarized/",     // TSA 证书 + 固化记录本身就是系统写入
		"/reports/",       // 报告文件也是系统产物
		"/anchor_batches/",
	}
	for _, s := range skipContains {
		if contains(slashPath, s) {
			return true
		}
	}
	return false
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func shortRand(n int) string {
	return randomStr(n) // 复用 store.go 中的
}
