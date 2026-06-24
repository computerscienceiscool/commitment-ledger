package trust

import "testing"

func TestValidatePolicyRejectsUnsupportedMode(t *testing.T) {
	err := validatePolicy(Policy{TrustedImportModes: []string{"ftp"}})
	if err == nil {
		t.Fatal("expected unsupported mode error")
	}
}

func TestParsePolicyRejectsUnknownField(t *testing.T) {
	var policy Policy
	err := parsePolicy([]byte(`{"trust_built_in_signers":true,"unexpected":1}`), &policy)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}
