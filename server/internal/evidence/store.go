package evidence

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Evidence 单条证据
type Evidence struct {
	ID             string            `json:"id"`
	URL            string            `json:"url"`
	CollectedAt    string            `json:"collectedAt"`
	TextContent    string            `json:"textContent"`
	ScreenshotPath string            `json:"screenshotPath,omitempty"`
	HTMLPath       string            `json:"htmlPath,omitempty"`
	Metadata       map[string]string `json:"metadata"`
	ContentHash    string            `json:"contentHash"`
}

// Case 案件
type Case struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	CreatedAt   string              `json:"createdAt"`
	UpdatedAt   string              `json:"updatedAt"`
	Status      string              `json:"status"` // collecting | analyzing | completed
	OriginalURL string              `json:"originalUrl,omitempty"`
	TargetURLs  []string            `json:"targetUrls"`
	Evidences   []Evidence          `json:"evidences"`
	Report      *InfringementReport `json:"report,omitempty"`
}

// InfringementReport 侵权分析报告
type InfringementReport struct {
	GeneratedAt     string          `json:"generatedAt"`
	Confidence      float64         `json:"confidence"`
	InfringementType string         `json:"infringementType"`
	CitedLaws       []string        `json:"citedLaws"`
	EvidenceSummary string          `json:"evidenceSummary"`
	LegalReasoning  LegalReasoning  `json:"legalReasoning"`
	Recommendations []string        `json:"recommendations"`
}

// LegalReasoning 法律三段论推理
type LegalReasoning struct {
	MajorPremise string `json:"majorPremise"`
	MinorPremise string `json:"minorPremise"`
	Conclusion   string `json:"conclusion"`
}

// Store 证据存储管理器
type Store struct {
	mu      sync.RWMutex
	dataDir string
	cases   map[string]*Case
}

// NewStore 创建证据存储
func NewStore(dataDir string) *Store {
	if dataDir == "" {
		dataDir = filepath.Join(".", "data", "evidence")
	}
	return &Store{
		dataDir: dataDir,
		cases:   make(map[string]*Case),
	}
}

// Init 初始化存储目录
func (s *Store) Init() error {
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}
	return s.loadCases()
}

// loadCases 从磁盘加载已有案件
func (s *Store) loadCases() error {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil // 目录不存在则忽略
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(s.dataDir, entry.Name(), "case.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var c Case
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		s.cases[c.ID] = &c
	}
	return nil
}

// CreateCase 创建新案件
func (s *Store) CreateCase(title, originalURL string, targetURLs []string) (*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("case_%d_%s", now.UnixMilli(), randomStr(6))

	c := &Case{
		ID:          id,
		Title:       title,
		CreatedAt:   now.Format(time.RFC3339),
		UpdatedAt:   now.Format(time.RFC3339),
		Status:      "collecting",
		OriginalURL: originalURL,
		TargetURLs:  targetURLs,
		Evidences:   []Evidence{},
	}

	caseDir := filepath.Join(s.dataDir, id)
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		return nil, err
	}

	s.cases[id] = c
	if err := s.saveCase(c); err != nil {
		return nil, err
	}
	return c, nil
}

// AddEvidence 添加证据
func (s *Store) AddEvidence(caseID, url, textContent string, screenshot []byte, html string, metadata map[string]string) (*Evidence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cases[caseID]
	if !ok {
		return nil, fmt.Errorf("案件不存在: %s", caseID)
	}

	now := time.Now()
	evidenceID := fmt.Sprintf("ev_%d_%s", now.UnixMilli(), randomStr(6))
	caseDir := filepath.Join(s.dataDir, caseID)

	hash := sha256.Sum256([]byte(textContent))
	contentHash := fmt.Sprintf("%x", hash)

	ev := Evidence{
		ID:          evidenceID,
		URL:         url,
		CollectedAt: now.Format(time.RFC3339),
		TextContent: textContent,
		Metadata:    metadata,
		ContentHash: contentHash,
	}

	// 保存截图
	if len(screenshot) > 0 {
		fname := evidenceID + "_screenshot.png"
		if err := os.WriteFile(filepath.Join(caseDir, fname), screenshot, 0644); err == nil {
			ev.ScreenshotPath = fname
		}
	}

	// 保存 HTML
	if html != "" {
		fname := evidenceID + "_page.html"
		if err := os.WriteFile(filepath.Join(caseDir, fname), []byte(html), 0644); err == nil {
			ev.HTMLPath = fname
		}
	}

	// 保存文本
	textFname := evidenceID + "_content.txt"
	_ = os.WriteFile(filepath.Join(caseDir, textFname), []byte(textContent), 0644)

	c.Evidences = append(c.Evidences, ev)
	c.UpdatedAt = now.Format(time.RFC3339)

	if err := s.saveCase(c); err != nil {
		return nil, err
	}
	_ = s.generateManifest(caseID)

	return &ev, nil
}

// SaveReport 保存分析报告
func (s *Store) SaveReport(caseID string, report *InfringementReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cases[caseID]
	if !ok {
		return fmt.Errorf("案件不存在: %s", caseID)
	}

	c.Report = report
	c.Status = "completed"
	c.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := s.saveCase(c); err != nil {
		return err
	}

	// 保存报告为独立 JSON
	reportPath := filepath.Join(s.dataDir, caseID, "report.json")
	data, _ := json.MarshalIndent(report, "", "  ")
	return os.WriteFile(reportPath, data, 0644)
}

// ListCases 获取案件列表
func (s *Store) ListCases() []*Case {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Case, 0, len(s.cases))
	for _, c := range s.cases {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt > list[j].UpdatedAt
	})
	return list
}

// GetCase 获取案件详情
func (s *Store) GetCase(caseID string) *Case {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cases[caseID]
}

// GetEvidenceFilePath 获取证据文件路径
func (s *Store) GetEvidenceFilePath(caseID, filename string) string {
	p := filepath.Join(s.dataDir, caseID, filename)
	// 安全检查：防止路径穿越
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	absData, _ := filepath.Abs(s.dataDir)
	if len(abs) < len(absData) || abs[:len(absData)] != absData {
		return ""
	}
	if _, err := os.Stat(abs); err != nil {
		return ""
	}
	return abs
}

func (s *Store) saveCase(c *Case) error {
	caseDir := filepath.Join(s.dataDir, c.ID)
	_ = os.MkdirAll(caseDir, 0755)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(caseDir, "case.json"), data, 0644)
}

func (s *Store) generateManifest(caseID string) error {
	c, ok := s.cases[caseID]
	if !ok {
		return nil
	}
	caseDir := filepath.Join(s.dataDir, caseID)

	files := make(map[string]string)
	for _, ev := range c.Evidences {
		files[ev.ID+"_content.txt"] = ev.ContentHash

		if ev.ScreenshotPath != "" {
			if data, err := os.ReadFile(filepath.Join(caseDir, ev.ScreenshotPath)); err == nil {
				h := sha256.Sum256(data)
				files[ev.ScreenshotPath] = fmt.Sprintf("%x", h)
			}
		}
		if ev.HTMLPath != "" {
			if data, err := os.ReadFile(filepath.Join(caseDir, ev.HTMLPath)); err == nil {
				h := sha256.Sum256(data)
				files[ev.HTMLPath] = fmt.Sprintf("%x", h)
			}
		}
	}

	manifest := map[string]interface{}{
		"caseId":      caseID,
		"generatedAt": time.Now().Format(time.RFC3339),
		"files":       files,
	}

	// 计算 manifest 自身哈希
	raw, _ := json.Marshal(manifest)
	mh := sha256.Sum256(raw)
	manifest["manifestHash"] = fmt.Sprintf("%x", mh)

	data, _ := json.MarshalIndent(manifest, "", "  ")
	return os.WriteFile(filepath.Join(caseDir, "evidence_manifest.json"), data, 0644)
}

func randomStr(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
