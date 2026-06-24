package grid

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestBuildAndVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	artifact, err := Build("bafyprotocol", []byte(`{"kind":"commitment"}`), "JJ", "jj-key", pub, priv)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if artifact.EnvelopeCID == "" || artifact.PayloadCID == "" || artifact.ProofCID == "" {
		t.Fatal("expected non-empty cids")
	}
	if err := Verify("bafyprotocol", artifact.PayloadBytes, artifact.ProofBytes); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestDecodeEnvelopeRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	artifact, err := Build("bafyprotocol", []byte(`{"kind":"commitment"}`), "JJ", "jj-key", pub, priv)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	decoded, err := DecodeEnvelope(artifact.Envelope)
	if err != nil {
		t.Fatalf("DecodeEnvelope: %v", err)
	}
	if decoded.ProtocolPCID != artifact.ProtocolPCID {
		t.Fatalf("protocol pCID = %q, want %q", decoded.ProtocolPCID, artifact.ProtocolPCID)
	}
	if decoded.EnvelopeCID != artifact.EnvelopeCID || decoded.PayloadCID != artifact.PayloadCID || decoded.ProofCID != artifact.ProofCID {
		t.Fatalf("decoded CIDs = %+v, want envelope=%q payload=%q proof=%q", decoded, artifact.EnvelopeCID, artifact.PayloadCID, artifact.ProofCID)
	}
	if err := Verify(decoded.ProtocolPCID, decoded.PayloadBytes, decoded.ProofBytes); err != nil {
		t.Fatalf("Verify decoded: %v", err)
	}
}

func TestVerifyFailsForTamperedPayload(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	artifact, err := Build("bafyprotocol", []byte(`{"kind":"commitment"}`), "JJ", "jj-key", pub, priv)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	decoded, err := DecodeEnvelope(artifact.Envelope)
	if err != nil {
		t.Fatalf("DecodeEnvelope: %v", err)
	}
	decoded.PayloadBytes[0] ^= 0x01
	if err := Verify(decoded.ProtocolPCID, decoded.PayloadBytes, decoded.ProofBytes); err == nil {
		t.Fatal("expected signature verification failure for tampered payload")
	}
}
