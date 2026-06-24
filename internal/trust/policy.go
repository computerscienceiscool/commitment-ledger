package trust

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"commitment-ledger/internal/model"
)

const DefaultPath = "config/trust-policy.json"

type Policy struct {
	Path                  string   `json:"-"`
	Found                 bool     `json:"-"`
	TrustBuiltInSigners   bool     `json:"trust_built_in_signers"`
	TrustBuiltInProtocols bool     `json:"trust_built_in_protocols"`
	TrustedSigners        []string `json:"trusted_signers,omitempty"`
	TrustedProtocolPCIDs  []string `json:"trusted_protocol_pcids,omitempty"`
	TrustedImportModes    []string `json:"trusted_import_modes,omitempty"`
	TrustedImportPrefixes []string `json:"trusted_import_path_prefixes,omitempty"`
}

type Evaluation struct {
	PolicyPath      string
	PolicyFound     bool
	SignerTrusted   bool
	ProtocolTrusted bool
	ImportTrusted   bool
	ImportApplies   bool
	OverallTrusted  bool
	SignerReason    string
	ProtocolReason  string
	ImportReason    string
}

func Load(root string) (Policy, error) {
	path := filepath.Join(root, DefaultPath)
	policy := Policy{
		Path:                  path,
		TrustBuiltInSigners:   true,
		TrustBuiltInProtocols: true,
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return policy, nil
	}
	if err != nil {
		return Policy{}, fmt.Errorf("read trust policy %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &policy); err != nil {
		return Policy{}, fmt.Errorf("parse trust policy %q: %w", path, err)
	}
	policy.Path = path
	policy.Found = true
	return policy, nil
}

func Evaluate(policy Policy, signer string, signerBuiltIn bool, protocolPCID string, protocolBuiltIn bool, latestImport *model.ImportRecord) Evaluation {
	out := Evaluation{
		PolicyPath:  policy.Path,
		PolicyFound: policy.Found,
	}

	switch {
	case signerBuiltIn && policy.TrustBuiltInSigners:
		out.SignerTrusted = true
		out.SignerReason = "built-in local identity"
	case contains(policy.TrustedSigners, signer):
		out.SignerTrusted = true
		out.SignerReason = "listed in trust policy"
	default:
		out.SignerReason = "not trusted by current policy"
	}

	switch {
	case protocolBuiltIn && policy.TrustBuiltInProtocols:
		out.ProtocolTrusted = true
		out.ProtocolReason = "built-in frozen protocol doc"
	case contains(policy.TrustedProtocolPCIDs, protocolPCID):
		out.ProtocolTrusted = true
		out.ProtocolReason = "listed in trust policy"
	default:
		out.ProtocolReason = "not trusted by current policy"
	}

	if latestImport == nil {
		out.ImportTrusted = true
		out.ImportReason = "no import provenance for this artifact"
	} else {
		out.ImportApplies = true
		modeAllowed := len(policy.TrustedImportModes) == 0 || contains(policy.TrustedImportModes, latestImport.Mode)
		pathAllowed := hasTrustedPrefix(policy.TrustedImportPrefixes, latestImport.SourcePath)
		if modeAllowed && pathAllowed {
			out.ImportTrusted = true
			out.ImportReason = "source path matched trust policy"
		} else if !modeAllowed {
			out.ImportReason = "import mode not trusted by current policy"
		} else {
			out.ImportReason = "import source path not trusted by current policy"
		}
	}

	out.OverallTrusted = out.SignerTrusted && out.ProtocolTrusted && out.ImportTrusted
	return out
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func hasTrustedPrefix(prefixes []string, path string) bool {
	if len(prefixes) == 0 {
		return false
	}
	cleaned := filepath.Clean(path)
	for _, prefix := range prefixes {
		if strings.HasPrefix(cleaned, filepath.Clean(prefix)) {
			return true
		}
	}
	return false
}
