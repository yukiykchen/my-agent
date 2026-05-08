package notary

import (
	"encoding/asn1"
	"math/big"
)

// ecdsaSig ECDSA 签名 ASN.1 结构
type ecdsaSig struct {
	R, S *big.Int
}

// asn1Encode 将 ECDSA 签名 (r, s) 编码为 ASN.1 DER
func asn1Encode(r, s *big.Int) ([]byte, error) {
	return asn1.Marshal(ecdsaSig{R: r, S: s})
}

// asn1Decode 解码 ASN.1 DER 到 (r, s)
func asn1Decode(data []byte) (*big.Int, *big.Int, error) {
	var sig ecdsaSig
	if _, err := asn1.Unmarshal(data, &sig); err != nil {
		return nil, nil, err
	}
	return sig.R, sig.S, nil
}
