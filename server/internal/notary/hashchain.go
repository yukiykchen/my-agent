package notary

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// HashChain 四值哈希链
// 对应论文 4.2.4：客户端、服务端、TSA、区块链 四值一致性
type HashChain struct {
	ClientHash     string `json:"clientHash"`     // 客户端 Web Crypto API 计算
	ServerHash     string `json:"serverHash"`     // 服务端落盘后重算
	TSAHash        string `json:"tsaHash"`        // TSA 时间戳证书中锚定的哈希
	BlockchainHash string `json:"blockchainHash"` // 区块链 Merkle Proof 推导出的哈希（可空，表示尚未上链）
}

// VerifyResult 校验结果
type VerifyResult struct {
	Consistent bool     `json:"consistent"`         // 是否一致
	Mismatches []string `json:"mismatches,omitempty"` // 不一致的环节
	Reason     string   `json:"reason,omitempty"`     // 简要说明
}

// Verify 校验四值一致性
// 若 BlockchainHash 为空，仅对比前三者
func (c HashChain) Verify() VerifyResult {
	vals := map[string]string{
		"client": strings.ToLower(c.ClientHash),
		"server": strings.ToLower(c.ServerHash),
		"tsa":    strings.ToLower(c.TSAHash),
	}
	if c.BlockchainHash != "" {
		vals["blockchain"] = strings.ToLower(c.BlockchainHash)
	}

	// 取第一个非空作为基准
	var base string
	var baseKey string
	for _, k := range []string{"client", "server", "tsa", "blockchain"} {
		if v := vals[k]; v != "" {
			base = v
			baseKey = k
			break
		}
	}

	if base == "" {
		return VerifyResult{Consistent: false, Reason: "所有哈希均为空"}
	}

	var mismatches []string
	for k, v := range vals {
		if v == "" {
			continue
		}
		if v != base {
			mismatches = append(mismatches, k)
		}
	}

	if len(mismatches) == 0 {
		return VerifyResult{Consistent: true, Reason: fmt.Sprintf("4 值哈希全部一致（基准来源：%s）", baseKey)}
	}
	return VerifyResult{
		Consistent: false,
		Mismatches: mismatches,
		Reason:     fmt.Sprintf("以下环节哈希与基准（%s）不一致：%v", baseKey, mismatches),
	}
}

// ComputeFileHash 计算文件 SHA-256（hex 小写）
func ComputeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return ComputeBytesHash(data), nil
}

// ComputeBytesHash 计算字节流 SHA-256
func ComputeBytesHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
