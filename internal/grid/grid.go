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

type DecodedArtifact struct {
	ProtocolPCID string
	PayloadBytes []byte
	PayloadCID   string
	ProofBytes   []byte
	ProofCID     string
	Proof        Proof
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
	proof, err := ParseProof(proofBytes)
	if err != nil {
		return err
	}
	return VerifyWithProof(protocolPCID, payload, proof, proofBytes)
}

func VerifyWithProof(protocolPCID string, payload []byte, proof Proof, proofBytes []byte) error {
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

func ParseProof(proofBytes []byte) (Proof, error) {
	var proof Proof
	if err := json.Unmarshal(proofBytes, &proof); err != nil {
		return Proof{}, fmt.Errorf("unmarshal proof: %w", err)
	}
	return proof, nil
}

func DecodeEnvelope(envelope []byte) (DecodedArtifact, error) {
	offset := 0
	tag, err := decodeTag(envelope, &offset)
	if err != nil {
		return DecodedArtifact{}, err
	}
	if tag != gridTag {
		return DecodedArtifact{}, fmt.Errorf("unexpected outer tag %d", tag)
	}
	count, err := decodeArrayHeader(envelope, &offset)
	if err != nil {
		return DecodedArtifact{}, err
	}
	if count != 3 {
		return DecodedArtifact{}, fmt.Errorf("unexpected envelope item count %d", count)
	}
	selectorTag, err := decodeTag(envelope, &offset)
	if err != nil {
		return DecodedArtifact{}, err
	}
	if selectorTag != pcidTag {
		return DecodedArtifact{}, fmt.Errorf("unexpected protocol selector tag %d", selectorTag)
	}
	protocolPCID, err := decodeText(envelope, &offset)
	if err != nil {
		return DecodedArtifact{}, err
	}
	payload, err := decodeBytes(envelope, &offset)
	if err != nil {
		return DecodedArtifact{}, err
	}
	proofBytes, err := decodeBytes(envelope, &offset)
	if err != nil {
		return DecodedArtifact{}, err
	}
	if offset != len(envelope) {
		return DecodedArtifact{}, fmt.Errorf("unexpected trailing bytes in envelope")
	}
	proof, err := ParseProof(proofBytes)
	if err != nil {
		return DecodedArtifact{}, err
	}
	return DecodedArtifact{
		ProtocolPCID: protocolPCID,
		PayloadBytes: append([]byte(nil), payload...),
		PayloadCID:   cid.Sum(payload),
		ProofBytes:   append([]byte(nil), proofBytes...),
		ProofCID:     cid.Sum(proofBytes),
		Proof:        proof,
		Envelope:     append([]byte(nil), envelope...),
		EnvelopeCID:  cid.Sum(envelope),
	}, nil
}

func decodeTag(data []byte, offset *int) (uint64, error) {
	major, value, err := decodeHead(data, offset)
	if err != nil {
		return 0, err
	}
	if major != 6 {
		return 0, fmt.Errorf("expected tag major type, got %d", major)
	}
	return value, nil
}

func decodeArrayHeader(data []byte, offset *int) (uint64, error) {
	major, value, err := decodeHead(data, offset)
	if err != nil {
		return 0, err
	}
	if major != 4 {
		return 0, fmt.Errorf("expected array major type, got %d", major)
	}
	return value, nil
}

func decodeText(data []byte, offset *int) (string, error) {
	major, size, err := decodeHead(data, offset)
	if err != nil {
		return "", err
	}
	if major != 3 {
		return "", fmt.Errorf("expected text major type, got %d", major)
	}
	bytes, err := take(data, offset, size)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func decodeBytes(data []byte, offset *int) ([]byte, error) {
	major, size, err := decodeHead(data, offset)
	if err != nil {
		return nil, err
	}
	if major != 2 {
		return nil, fmt.Errorf("expected bytes major type, got %d", major)
	}
	return take(data, offset, size)
}

func decodeHead(data []byte, offset *int) (byte, uint64, error) {
	if *offset >= len(data) {
		return 0, 0, fmt.Errorf("unexpected end of data")
	}
	b := data[*offset]
	*offset = *offset + 1
	major := b >> 5
	additional := b & 0x1f
	switch {
	case additional <= 23:
		return major, uint64(additional), nil
	case additional == 24:
		bytes, err := take(data, offset, 1)
		if err != nil {
			return 0, 0, err
		}
		return major, uint64(bytes[0]), nil
	case additional == 25:
		bytes, err := take(data, offset, 2)
		if err != nil {
			return 0, 0, err
		}
		return major, uint64(bytes[0])<<8 | uint64(bytes[1]), nil
	case additional == 26:
		bytes, err := take(data, offset, 4)
		if err != nil {
			return 0, 0, err
		}
		return major, uint64(bytes[0])<<24 | uint64(bytes[1])<<16 | uint64(bytes[2])<<8 | uint64(bytes[3]), nil
	case additional == 27:
		bytes, err := take(data, offset, 8)
		if err != nil {
			return 0, 0, err
		}
		return major,
			uint64(bytes[0])<<56 | uint64(bytes[1])<<48 | uint64(bytes[2])<<40 | uint64(bytes[3])<<32 |
				uint64(bytes[4])<<24 | uint64(bytes[5])<<16 | uint64(bytes[6])<<8 | uint64(bytes[7]),
			nil
	default:
		return 0, 0, fmt.Errorf("unsupported additional info %d", additional)
	}
}

func take(data []byte, offset *int, size uint64) ([]byte, error) {
	end := *offset + int(size)
	if end < *offset || end > len(data) {
		return nil, fmt.Errorf("unexpected end of data")
	}
	out := append([]byte(nil), data[*offset:end]...)
	*offset = end
	return out, nil
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
