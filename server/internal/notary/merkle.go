package notary

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// MerkleTree 简化版 Merkle 二叉树
// 叶节点为证据 SHA-256（hex），内部节点为 sha256(left || right) 的 hex
// 若节点数为奇数，最右节点向上复制
type MerkleTree struct {
	leaves []string
	levels [][]string // levels[0]=leaves, levels[n]=root
}

// BuildMerkleTree 从叶子构造 Merkle 树
func BuildMerkleTree(leafHashes []string) (*MerkleTree, error) {
	if len(leafHashes) == 0 {
		return nil, fmt.Errorf("叶节点为空")
	}
	normed := make([]string, len(leafHashes))
	for i, h := range leafHashes {
		normed[i] = strings.ToLower(h)
	}
	levels := [][]string{normed}

	current := normed
	for len(current) > 1 {
		var next []string
		for i := 0; i < len(current); i += 2 {
			left := current[i]
			var right string
			if i+1 < len(current) {
				right = current[i+1]
			} else {
				right = left // 奇数补齐
			}
			next = append(next, hashPair(left, right))
		}
		levels = append(levels, next)
		current = next
	}

	return &MerkleTree{leaves: normed, levels: levels}, nil
}

// Root 根哈希
func (m *MerkleTree) Root() string {
	return m.levels[len(m.levels)-1][0]
}

// Leaves 叶节点列表
func (m *MerkleTree) Leaves() []string {
	cp := make([]string, len(m.leaves))
	copy(cp, m.leaves)
	return cp
}

// MerkleProofStep 认证路径中的一步（兄弟节点 + 位置）
type MerkleProofStep struct {
	Sibling  string `json:"sibling"`
	Position string `json:"position"` // "left" 或 "right"，表示 sibling 在当前节点的哪一侧
}

// GenerateProof 生成第 index 个叶节点到 root 的认证路径
func (m *MerkleTree) GenerateProof(index int) ([]MerkleProofStep, error) {
	if index < 0 || index >= len(m.leaves) {
		return nil, fmt.Errorf("index 越界")
	}
	var proof []MerkleProofStep
	idx := index
	for lvl := 0; lvl < len(m.levels)-1; lvl++ {
		nodes := m.levels[lvl]
		var siblingIdx int
		var position string
		if idx%2 == 0 {
			siblingIdx = idx + 1
			position = "right"
		} else {
			siblingIdx = idx - 1
			position = "left"
		}
		if siblingIdx >= len(nodes) {
			siblingIdx = idx // 孤点复制
		}
		proof = append(proof, MerkleProofStep{
			Sibling:  nodes[siblingIdx],
			Position: position,
		})
		idx /= 2
	}
	return proof, nil
}

// VerifyProof 独立验证某叶节点是否属于给定 root
// 真实场景下，任何持有 leaf + root + proof 的一方均可独立执行
func VerifyProof(leaf, root string, proof []MerkleProofStep) bool {
	current := strings.ToLower(leaf)
	for _, step := range proof {
		if step.Position == "right" {
			current = hashPair(current, step.Sibling)
		} else {
			current = hashPair(step.Sibling, current)
		}
	}
	return current == strings.ToLower(root)
}

func hashPair(left, right string) string {
	h := sha256.New()
	// 拼接的是 hex 字符串的字节
	h.Write([]byte(left))
	h.Write([]byte(right))
	return hex.EncodeToString(h.Sum(nil))
}
