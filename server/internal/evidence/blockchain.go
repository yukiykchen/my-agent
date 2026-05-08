package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"infringement-agent-server/internal/notary"
)

// AnchorBatch 一次批量上链记录（论文 4.3.3 Merkle Tree 批量聚合）
type AnchorBatch struct {
	BatchID     string                 `json:"batchId"`
	CaseID      string                 `json:"caseId"`
	AnchoredAt  string                 `json:"anchoredAt"`
	Evidences   []BatchEvidenceEntry   `json:"evidences"`
	MerkleRoot  string                 `json:"merkleRoot"`
	Tx          *notary.BlockchainTx   `json:"tx"`
	EvidenceCnt int                    `json:"evidenceCount"`
}

// BatchEvidenceEntry 批量中的一条证据记录
type BatchEvidenceEntry struct {
	EvidenceID  string                  `json:"evidenceId"`
	LeafHash    string                  `json:"leafHash"`
	LeafIndex   int                     `json:"leafIndex"`
	MerkleProof []notary.MerkleProofStep `json:"merkleProof"`
}

// ChainAnchor 链上锚定服务
type ChainAnchor struct {
	notarize *NotarizeService
	chain    *notary.MockChainService
	mu       sync.Mutex
}

// NewChainAnchor 构造链上锚定服务
func NewChainAnchor(notarize *NotarizeService) (*ChainAnchor, error) {
	chainDir := filepath.Join(notarize.DataDir(), ".chain")
	chain, err := notary.NewMockChainService(chainDir, "MockAntChain")
	if err != nil {
		return nil, err
	}
	return &ChainAnchor{
		notarize: notarize,
		chain:    chain,
	}, nil
}

// AnchorPendingEvidences 对一个案件所有尚未上链的证据进行批量 Merkle 上链
// 返回本次上链的批次
func (a *ChainAnchor) AnchorPendingEvidences(caseID string) (*AnchorBatch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	list, err := a.notarize.ListNotarized(caseID)
	if err != nil {
		return nil, err
	}

	// 过滤出 BlockchainHash 为空的证据
	var pending []*NotarizedEvidence
	for _, ev := range list {
		if ev.HashChain.BlockchainHash == "" {
			pending = append(pending, ev)
		}
	}
	if len(pending) == 0 {
		return nil, fmt.Errorf("当前案件无待上链证据")
	}

	return a.anchorEvidences(caseID, pending)
}

// AnchorSpecificEvidences 按指定 id 上链
func (a *ChainAnchor) AnchorSpecificEvidences(caseID string, evidenceIDs []string) (*AnchorBatch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var list []*NotarizedEvidence
	for _, id := range evidenceIDs {
		ev, err := a.notarize.GetNotarized(caseID, id)
		if err != nil {
			return nil, fmt.Errorf("证据 %s 读取失败: %w", id, err)
		}
		list = append(list, ev)
	}
	return a.anchorEvidences(caseID, list)
}

func (a *ChainAnchor) anchorEvidences(caseID string, evidences []*NotarizedEvidence) (*AnchorBatch, error) {
	if len(evidences) == 0 {
		return nil, fmt.Errorf("证据列表为空")
	}

	// 收集叶子哈希（使用 serverHash）
	leaves := make([]string, len(evidences))
	for i, ev := range evidences {
		leaves[i] = ev.HashChain.ServerHash
	}

	// 构建 Merkle 树
	tree, err := notary.BuildMerkleTree(leaves)
	if err != nil {
		return nil, err
	}
	root := tree.Root()

	// 提交到 Mock 链
	tx, err := a.chain.SubmitMerkleRoot(root, map[string]string{
		"caseId":        caseID,
		"evidenceCount": fmt.Sprintf("%d", len(evidences)),
	})
	if err != nil {
		return nil, err
	}

	// 更新每条证据的上链信息
	entries := make([]BatchEvidenceEntry, len(evidences))
	for i, ev := range evidences {
		proof, _ := tree.GenerateProof(i)
		proofStrs := make([]string, len(proof))
		for j, step := range proof {
			proofStrs[j] = step.Position + ":" + step.Sibling
		}
		if err := a.notarize.UpdateBlockchainInfo(caseID, ev.EvidenceID, tx.TxHash, root, proofStrs); err != nil {
			return nil, err
		}
		entries[i] = BatchEvidenceEntry{
			EvidenceID:  ev.EvidenceID,
			LeafHash:    ev.HashChain.ServerHash,
			LeafIndex:   i,
			MerkleProof: proof,
		}
	}

	batch := &AnchorBatch{
		BatchID:     fmt.Sprintf("batch_%d", time.Now().UnixMilli()),
		CaseID:      caseID,
		AnchoredAt:  time.Now().UTC().Format(time.RFC3339Nano),
		Evidences:   entries,
		MerkleRoot:  root,
		Tx:          tx,
		EvidenceCnt: len(evidences),
	}

	// 持久化批次
	batchDir := filepath.Join(a.notarize.DataDir(), caseID, "anchor_batches")
	_ = os.MkdirAll(batchDir, 0755)
	data, _ := json.MarshalIndent(batch, "", "  ")
	_ = os.WriteFile(filepath.Join(batchDir, batch.BatchID+".json"), data, 0644)

	return batch, nil
}

// VerifyEvidenceOnChain 独立验证某证据是否在链上（对应论文 4.2.4 四值哈希链中"chain"这一值的独立验证）
func (a *ChainAnchor) VerifyEvidenceOnChain(caseID, evidenceID string) (bool, string, error) {
	ev, err := a.notarize.GetNotarized(caseID, evidenceID)
	if err != nil {
		return false, "", err
	}
	if ev.BlockchainTx == "" {
		return false, "证据尚未上链", nil
	}

	// 1. 查询交易
	tx, err := a.chain.QueryTx(ev.BlockchainTx)
	if err != nil {
		return false, "交易查询失败: " + err.Error(), nil
	}

	// 2. Merkle 根一致性
	if tx.MerkleRoot != ev.MerkleRoot {
		return false, "链上 MerkleRoot 与本地记录不一致", nil
	}

	// 3. 重建 Merkle Proof，独立验证
	proof := make([]notary.MerkleProofStep, 0, len(ev.MerkleProof))
	for _, p := range ev.MerkleProof {
		// 格式："left:hash" 或 "right:hash"
		for idx := 0; idx < len(p); idx++ {
			if p[idx] == ':' {
				proof = append(proof, notary.MerkleProofStep{
					Position: p[:idx],
					Sibling:  p[idx+1:],
				})
				break
			}
		}
	}
	ok := notary.VerifyProof(ev.HashChain.ServerHash, tx.MerkleRoot, proof)
	if !ok {
		return false, "Merkle Proof 验证失败", nil
	}

	return true, fmt.Sprintf("链上验证通过（区块高度 %d，交易哈希 %s）", tx.BlockHeight, tx.TxHash), nil
}

// Stats 链统计
func (a *ChainAnchor) Stats() map[string]interface{} {
	return a.chain.ChainStats()
}
