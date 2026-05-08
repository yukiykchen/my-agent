package notary

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BlockchainTx 模拟区块链交易回执（蚂蚁链风格）
// 真实场景下是从蚂蚁链 SDK 返回的完整交易数据
type BlockchainTx struct {
	TxHash      string    `json:"txHash"`      // 交易哈希
	BlockHeight int64     `json:"blockHeight"` // 模拟区块高度
	BlockHash   string    `json:"blockHash"`   // 区块哈希
	MerkleRoot  string    `json:"merkleRoot"`  // 本次上链的 Merkle 根
	Timestamp   time.Time `json:"timestamp"`   // 上链时间
	ChainName   string    `json:"chainName"`   // 链标识
	ChainURL    string    `json:"chainUrl"`    // 浏览器 URL（mock）
	Signature   string    `json:"signature"`   // 验证者节点签名
	Validator   string    `json:"validator"`   // 验证者公钥 PEM
}

// MockChainService 模拟蚂蚁链存证服务
// 本地持久化一个"链文件"，任何写入的交易都是 append-only，
// 任何第三方只要拿到对应的 tx.json 都能重新验证签名与 Merkle 认证路径
type MockChainService struct {
	mu         sync.Mutex
	chainDir   string
	blockNum   int64
	prevBlock  string
	privateKey *ecdsa.PrivateKey
	certPEM    string
	chainName  string
}

// NewMockChainService 构造 Mock 蚂蚁链服务
func NewMockChainService(chainDir, chainName string) (*MockChainService, error) {
	if err := os.MkdirAll(chainDir, 0755); err != nil {
		return nil, err
	}

	// 复用 TSA CA 密钥体系（简化），实际可独立密钥
	tsa, err := NewTSAService(chainDir)
	if err != nil {
		return nil, err
	}

	svc := &MockChainService{
		chainDir:   chainDir,
		privateKey: tsa.privateKey,
		certPEM:    tsa.certPEM,
		chainName:  chainName,
	}
	svc.loadHead()
	return svc, nil
}

func (s *MockChainService) loadHead() {
	headFile := filepath.Join(s.chainDir, "HEAD")
	if data, err := os.ReadFile(headFile); err == nil {
		var head struct {
			BlockNum  int64  `json:"blockNum"`
			PrevBlock string `json:"prevBlock"`
		}
		if json.Unmarshal(data, &head) == nil {
			s.blockNum = head.BlockNum
			s.prevBlock = head.PrevBlock
		}
	}
}

func (s *MockChainService) saveHead() {
	headFile := filepath.Join(s.chainDir, "HEAD")
	data, _ := json.MarshalIndent(map[string]interface{}{
		"blockNum":  s.blockNum,
		"prevBlock": s.prevBlock,
	}, "", "  ")
	_ = os.WriteFile(headFile, data, 0644)
}

// SubmitMerkleRoot 提交 Merkle 根上链
func (s *MockChainService) SubmitMerkleRoot(merkleRoot string, meta map[string]string) (*BlockchainTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.blockNum++
	now := time.Now().UTC()

	// 模拟区块哈希 = sha256(prevBlock || merkleRoot || timestamp || blockNum)
	blockPayload := fmt.Sprintf("%s|%s|%s|%d", s.prevBlock, merkleRoot, now.Format(time.RFC3339Nano), s.blockNum)
	blockHashBytes := sha256.Sum256([]byte(blockPayload))
	blockHash := hex.EncodeToString(blockHashBytes[:])

	// 交易哈希 = sha256(blockHash || merkleRoot)
	txPayload := blockHash + "|" + merkleRoot
	txHashBytes := sha256.Sum256([]byte(txPayload))
	txHash := hex.EncodeToString(txHashBytes[:])

	// 对 txHash 签名
	sigR, sigS, err := ecdsa.Sign(rand.Reader, s.privateKey, txHashBytes[:])
	if err != nil {
		return nil, err
	}
	sigBytes, _ := asn1Encode(sigR, sigS)
	sig := base64.StdEncoding.EncodeToString(sigBytes)

	tx := &BlockchainTx{
		TxHash:      "0x" + txHash,
		BlockHeight: s.blockNum,
		BlockHash:   "0x" + blockHash,
		MerkleRoot:  merkleRoot,
		Timestamp:   now,
		ChainName:   s.chainName,
		ChainURL:    fmt.Sprintf("https://mock-antchain.local/tx/0x%s", txHash),
		Signature:   sig,
		Validator:   s.certPEM,
	}

	// 持久化交易
	txFile := filepath.Join(s.chainDir, fmt.Sprintf("block_%010d.json", s.blockNum))
	record := map[string]interface{}{
		"tx":   tx,
		"meta": meta,
	}
	data, _ := json.MarshalIndent(record, "", "  ")
	_ = os.WriteFile(txFile, data, 0444)

	s.prevBlock = "0x" + blockHash
	s.saveHead()

	return tx, nil
}

// QueryTx 通过交易哈希查询（模拟区块链浏览器查询）
func (s *MockChainService) QueryTx(txHash string) (*BlockchainTx, error) {
	entries, err := os.ReadDir(s.chainDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !startsWith(e.Name(), "block_") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.chainDir, e.Name()))
		if err != nil {
			continue
		}
		var rec struct {
			Tx BlockchainTx `json:"tx"`
		}
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		if rec.Tx.TxHash == txHash {
			return &rec.Tx, nil
		}
	}
	return nil, fmt.Errorf("交易不存在: %s", txHash)
}

// ChainStats 链统计
func (s *MockChainService) ChainStats() map[string]interface{} {
	return map[string]interface{}{
		"chainName":   s.chainName,
		"blockHeight": s.blockNum,
		"prevBlock":   s.prevBlock,
		"explorerURL": "https://mock-antchain.local",
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
