// Package notary 实现证据可信固化相关服务：
// 1. Mock TSA（RFC 3161 风格）可信时间戳
// 2. 四值哈希链一致性校验
// 3. 监督链（Chain of Custody）审计日志
package notary

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TimestampToken 模拟 RFC 3161 TimeStampToken（.tsr 的简化 JSON 结构）
// 真实场景下应为 DER 编码的 CMS SignedData，这里以 JSON 方便调试展示
type TimestampToken struct {
	Version       int    `json:"version"`        // 1
	Policy        string `json:"policy"`         // OID 1.2.840.113549.1.9.16.2.14（timestamp-token-policy）
	HashAlgorithm string `json:"hashAlgorithm"`  // sha-256
	HashedMessage string `json:"hashedMessage"`  // 被签哈希（hex）
	SerialNumber  string `json:"serialNumber"`   // 时间戳序列号
	GenTime       string `json:"genTime"`        // 精确到毫秒的 UTC 时间
	TSAName       string `json:"tsaName"`        // 时间戳机构名称
	Signature     string `json:"signature"`      // ECDSA-P256 对 (hash||genTime) 的签名（base64）
	PublicKey     string `json:"publicKey"`      // 签名公钥（PEM，用于验证）
	CertSubject   string `json:"certSubject"`    // 证书主体
}

// TSAService Mock TSA 服务
type TSAService struct {
	mu         sync.Mutex
	privateKey *ecdsa.PrivateKey
	certPEM    string
	subject    string
	tsaName    string
	serial     int64
	certPath   string
}

// NewTSAService 构造 TSA 服务，自动加载或生成 CA
// certDir：CA 证书持久化目录（生成后后续启动复用，保证签名可被历史证据验证）
func NewTSAService(certDir string) (*TSAService, error) {
	_ = os.MkdirAll(certDir, 0755)
	t := &TSAService{
		subject:  "CN=Mock TSA CA, O=Evidence Chain System, C=CN",
		tsaName:  "Mock联合信任时间戳服务中心",
		certPath: certDir,
	}
	if err := t.loadOrCreateCA(); err != nil {
		return nil, err
	}
	return t, nil
}

// loadOrCreateCA 加载或生成 CA 密钥
func (t *TSAService) loadOrCreateCA() error {
	keyFile := filepath.Join(t.certPath, "tsa_ca.key")
	certFile := filepath.Join(t.certPath, "tsa_ca.crt")

	if keyPEM, err := os.ReadFile(keyFile); err == nil {
		block, _ := pem.Decode(keyPEM)
		if block != nil {
			if priv, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
				t.privateKey = priv
				if certBytes, err := os.ReadFile(certFile); err == nil {
					t.certPEM = string(certBytes)
					return nil
				}
			}
		}
	}

	// 生成 P-256 ECDSA 密钥
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	t.privateKey = priv

	// 构造自签 CA 证书
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   "Mock TSA CA",
			Organization: []string{"Evidence Chain System"},
			Country:      []string{"CN"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	t.certPEM = string(certPEM)

	// 持久化
	keyDER, _ := x509.MarshalECPrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	_ = os.WriteFile(keyFile, keyPEM, 0600)
	_ = os.WriteFile(certFile, certPEM, 0644)
	return nil
}

// ApplyTimestamp 对给定的 SHA-256 哈希申请时间戳
func (t *TSAService) ApplyTimestamp(hashHex string) (*TimestampToken, error) {
	if len(hashHex) != 64 {
		return nil, fmt.Errorf("invalid sha-256 hex length: %d (expected 64)", len(hashHex))
	}

	t.mu.Lock()
	t.serial++
	serial := t.serial
	t.mu.Unlock()

	genTime := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	payload := hashHex + "|" + genTime

	h := sha256.Sum256([]byte(payload))
	r, s, err := ecdsa.Sign(rand.Reader, t.privateKey, h[:])
	if err != nil {
		return nil, err
	}
	sigBytes, _ := asn1Encode(r, s)

	pubDER, _ := x509.MarshalPKIXPublicKey(&t.privateKey.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	return &TimestampToken{
		Version:       1,
		Policy:        "1.2.840.113549.1.9.16.2.14",
		HashAlgorithm: "sha-256",
		HashedMessage: hashHex,
		SerialNumber:  fmt.Sprintf("TSA-MOCK-%019d", serial),
		GenTime:       genTime,
		TSAName:       t.tsaName,
		Signature:     base64.StdEncoding.EncodeToString(sigBytes),
		PublicKey:     string(pubPEM),
		CertSubject:   t.subject,
	}, nil
}

// VerifyTimestamp 独立验证时间戳（可被第三方调用）
// 任何持有该 token 的一方均可通过公开的 CA 公钥验证其真伪
func (t *TSAService) VerifyTimestamp(token *TimestampToken) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	// 1. 解析公钥
	block, _ := pem.Decode([]byte(token.PublicKey))
	if block == nil {
		return fmt.Errorf("invalid public key PEM")
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}
	pub, ok := pubAny.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not ECDSA")
	}

	// 2. 重构签名 payload
	payload := token.HashedMessage + "|" + token.GenTime
	h := sha256.Sum256([]byte(payload))

	// 3. 解码签名
	sigBytes, err := base64.StdEncoding.DecodeString(token.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	r, s, err := asn1Decode(sigBytes)
	if err != nil {
		return fmt.Errorf("parse signature: %w", err)
	}

	// 4. 验证签名
	if !ecdsa.Verify(pub, h[:], r, s) {
		return fmt.Errorf("ECDSA signature verification failed")
	}
	return nil
}

// SaveTokenFile 将时间戳持久化为 .tsr 文件（此处使用 JSON 便于检验，真实场景为 DER）
func (t *TSAService) SaveTokenFile(token *TimestampToken, path string) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0444) // 时间戳证书本身也设置为只读
}

// LoadTokenFile 从文件加载
func LoadTokenFile(path string) (*TimestampToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tk TimestampToken
	if err := json.Unmarshal(data, &tk); err != nil {
		return nil, err
	}
	return &tk, nil
}
