package notary

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CustodyEvent 监督链事件（append-only）
// 对应论文 4.3.4 Chain of Custody
type CustodyEvent struct {
	EventID      string            `json:"eventId"`
	EventType    string            `json:"eventType"` // create | read | update | transfer | verify | alarm
	Timestamp    string            `json:"timestamp"` // RFC3339Nano
	Actor        string            `json:"actor"`     // 操作者（用户 ID 或系统）
	ClientIP     string            `json:"clientIp,omitempty"`
	EvidenceID   string            `json:"evidenceId,omitempty"`
	TargetPath   string            `json:"targetPath,omitempty"`
	HashBefore   string            `json:"hashBefore,omitempty"`
	HashAfter    string            `json:"hashAfter,omitempty"`
	Remarks      string            `json:"remarks,omitempty"`
	Meta         map[string]string `json:"meta,omitempty"`
	PrevLinkHash string            `json:"prevLinkHash"` // 链上前一事件的哈希（链式防篡改）
	LinkHash     string            `json:"linkHash"`     // 本事件自身链式哈希
}

// CustodyLog 监督链日志（每个案件一份）
type CustodyLog struct {
	mu       sync.Mutex
	path     string
	caseID   string
	lastHash string
	counter  int64
}

// NewCustodyLog 打开/创建案件监督链日志
func NewCustodyLog(caseDir, caseID string) (*CustodyLog, error) {
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(caseDir, "custody.log.jsonl")
	cl := &CustodyLog{path: path, caseID: caseID}
	// 重放找到最后一条链哈希
	if data, err := os.ReadFile(path); err == nil {
		lines := splitLines(data)
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			var ev CustodyEvent
			if err := json.Unmarshal(line, &ev); err == nil {
				cl.lastHash = ev.LinkHash
				cl.counter++
			}
		}
	}
	return cl, nil
}

// Append 追加事件
func (c *CustodyLog) Append(ev CustodyEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.counter++
	if ev.EventID == "" {
		ev.EventID = fmt.Sprintf("%s-%d", c.caseID, c.counter)
	}
	if ev.Timestamp == "" {
		ev.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	ev.PrevLinkHash = c.lastHash

	// 计算 LinkHash = sha256(prev || 规范化事件体)
	evClone := ev
	evClone.LinkHash = "" // 排除自身
	body, _ := json.Marshal(evClone)
	ev.LinkHash = ComputeBytesHash(append([]byte(ev.PrevLinkHash), body...))
	c.lastHash = ev.LinkHash

	line, _ := json.Marshal(ev)
	line = append(line, '\n')

	f, err := os.OpenFile(c.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

// Verify 验证整个监督链未被篡改（重放 link hash）
func (c *CustodyLog) Verify() (bool, string, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, "日志为空", nil
		}
		return false, "", err
	}
	lines := splitLines(data)
	prev := ""
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		var ev CustodyEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return false, fmt.Sprintf("第 %d 行解析失败", i+1), nil
		}
		if ev.PrevLinkHash != prev {
			return false, fmt.Sprintf("第 %d 行 PrevLinkHash 不匹配，监督链被篡改", i+1), nil
		}
		expected := ev.LinkHash
		ev.LinkHash = ""
		body, _ := json.Marshal(ev)
		recomputed := ComputeBytesHash(append([]byte(ev.PrevLinkHash), body...))
		if recomputed != expected {
			return false, fmt.Sprintf("第 %d 行 LinkHash 不匹配", i+1), nil
		}
		prev = expected
	}
	return true, fmt.Sprintf("监督链 %d 条记录完整", len(lines)), nil
}

// List 列出全部事件
func (c *CustodyLog) List() ([]CustodyEvent, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := splitLines(data)
	out := make([]CustodyEvent, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var ev CustodyEvent
		if err := json.Unmarshal(line, &ev); err == nil {
			out = append(out, ev)
		}
	}
	return out, nil
}

func splitLines(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			out = append(out, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out
}
