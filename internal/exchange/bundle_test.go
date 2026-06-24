package exchange

import "testing"

func TestParseBundleRejectsUnknownField(t *testing.T) {
	_, err := ParseBundle([]byte(`{"version":"commitment-ledger-bundle-v1","unexpected":1}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestParseBundleRejectsMissingArtifactFields(t *testing.T) {
	_, err := ParseBundle([]byte(`{"version":"commitment-ledger-bundle-v1","artifact":{"artifact_cid":"bafy"},"envelope_base64":"YWJj"}`))
	if err == nil {
		t.Fatal("expected missing field error")
	}
}
