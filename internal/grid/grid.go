package grid

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"commitment-ledger/internal/cid"
)

const (
	gridTag = 1735551332
	pcidTag = 42
)

type Proof struct {
	Alg       string `json:"alg"`
	Signer    string `json:"signer"`
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

type Artifact struct {
	ProtocolPCID string
	PayloadBytes []byte
	PayloadCID   string
	ProofBytes   []byte
	ProofCID     string
	Envelope     []byte
	EnvelopeCID  string
}

func Build(protocolPCID string, payload []byte, signer string, keyID string, publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey) (Artifact, error) {
	signable, err := encodeSignable(protocolPCID, payload)
	if err != nil {
		return Artifact{}, err
	}
	sig := ed25519.Sign(privateKey, signable)
	proofPayload, err := json.Marshal(Proof{
		Alg:       "Ed25519",
		Signer:    signer,
		KeyID:     keyID,
		PublicKey: base64.StdEncoding.EncodeToString(publicKey),
		Signature: base64.StdEncoding.EncodeToString(sig),
	})
	if err != nil {
		return Artifact{}, fmt.Errorf("marshal proof: %w", err)
	}
	envelope, err := encodeEnvelope(protocolPCID, payload, proofPayload)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{
		ProtocolPCID: protocolPCID,
		PayloadBytes: append([]byte(nil), payload...),
		PayloadCID:   cid.Sum(payload),
		ProofBytes:   proofPayload,
		ProofCID:     cid.Sum(proofPayload),
		Envelope:     envelope,
		EnvelopeCID:  cid.Sum(envelope),
	}, nil
}

func Verify(protocolPCID string, payload []byte, proofBytes []byte) error {
	var proof Proof
	if err := json.Unmarshal(proofBytes, &proof); err != nil {
		return fmt.Errorf("unmarshal proof: %w", err)
	}
	publicKey, err := base64.StdEncoding.DecodeString(proof.PublicKey)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	signature, err := base64.StdEncoding.DecodeString(proof.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	signable, err := encodeSignable(protocolPCID, payload)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), signable, signature) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

func encodeEnvelope(protocolPCID string, payload []byte, proof []byte) ([]byte, error) {
	selector, err := encodeTag(pcidTag, encodeText(protocolPCID))
	if err != nil {
		return nil, err
	}
	body := encodeArray([][]byte{
		selector,
		encodeBytes(payload),
		encodeBytes(proof),
	})
	return encodeTag(gridTag, body)
}

func encodeSignable(protocolPCID string, payload []byte) ([]byte, error) {
	selector, err := encodeTag(pcidTag, encodeText(protocolPCID))
	if err != nil {
		return nil, err
	}
	return encodeArray([][]byte{
		selector,
		encodeBytes(payload),
	}), nil
}

func encodeArray(items [][]byte) []byte {
	head := encodeHead(4, uint64(len(items)))
	size := len(head)
	for _, item := range items {
		size += len(item)
	}
	out := make([]byte, 0, size)
	out = append(out, head...)
	for _, item := range items {
		out = append(out, item...)
	}
	return out
}

func encodeTag(tag uint64, content []byte) ([]byte, error) {
	head := encodeHead(6, tag)
	out := make([]byte, 0, len(head)+len(content))
	out = append(out, head...)
	out = append(out, content...)
	return out, nil
}

func encodeText(s string) []byte {
	head := encodeHead(3, uint64(len(s)))
	out := make([]byte, 0, len(head)+len(s))
	out = append(out, head...)
	out = append(out, s...)
	return out
}

func encodeBytes(b []byte) []byte {
	head := encodeHead(2, uint64(len(b)))
	out := make([]byte, 0, len(head)+len(b))
	out = append(out, head...)
	out = append(out, b...)
	return out
}

func encodeHead(major byte, n uint64) []byte {
	switch {
	case n <= 23:
		return []byte{major<<5 | byte(n)}
	case n <= 0xff:
		return []byte{major<<5 | 24, byte(n)}
	case n <= 0xffff:
		return []byte{major<<5 | 25, byte(n >> 8), byte(n)}
	case n <= 0xffffffff:
		return []byte{major<<5 | 26, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	default:
		return []byte{major<<5 | 27, byte(n >> 56), byte(n >> 48), byte(n >> 40), byte(n >> 32), byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	}
}
