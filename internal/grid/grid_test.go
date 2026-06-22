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
