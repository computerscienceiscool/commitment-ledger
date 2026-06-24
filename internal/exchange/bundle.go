package exchange

import (
	"encoding/json"
	"fmt"
	"strings"

	"commitment-ledger/internal/model"
)

const BundleVersion = "commitment-ledger-bundle-v1"

type ProtocolSupport struct {
	Name          string `json:"name"`
	ProtocolPCID  string `json:"protocol_pcid"`
	DocCID        string `json:"doc_cid"`
	DocumentBytes string `json:"document_bytes_base64"`
}

type SignerSupport struct {
	Name      string `json:"name"`
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
}

type Bundle struct {
	Version    string               `json:"version"`
	ExportedAt string               `json:"exported_at"`
	Artifact   model.ArtifactRecord `json:"artifact"`
	Envelope   string               `json:"envelope_base64"`
	Commitment *model.Commitment    `json:"commitment,omitempty"`
	Evidence   *model.Evidence      `json:"evidence,omitempty"`
	Assessment *model.Assessment    `json:"assessment,omitempty"`
	Protocol   *ProtocolSupport     `json:"protocol,omitempty"`
	Signer     *SignerSupport       `json:"signer,omitempty"`
}

func ParseBundle(data []byte) (Bundle, error) {
	var bundle Bundle
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bundle); err != nil {
		return Bundle{}, fmt.Errorf("decode bundle: %w", err)
	}
	if err := validateBundle(bundle); err != nil {
		return Bundle{}, err
	}
	return bundle, nil
}

func validateBundle(bundle Bundle) error {
	if bundle.Version == "" {
		return fmt.Errorf("bundle missing version")
	}
	if bundle.Artifact.ArtifactCID == "" {
		return fmt.Errorf("bundle missing artifact.artifact_cid")
	}
	if bundle.Artifact.ProtocolPCID == "" {
		return fmt.Errorf("bundle missing artifact.protocol_pcid")
	}
	if bundle.Artifact.Kind == "" {
		return fmt.Errorf("bundle missing artifact.kind")
	}
	if bundle.Artifact.PayloadCID == "" {
		return fmt.Errorf("bundle missing artifact.payload_cid")
	}
	if bundle.Artifact.ProofCID == "" {
		return fmt.Errorf("bundle missing artifact.proof_cid")
	}
	if bundle.Envelope == "" {
		return fmt.Errorf("bundle missing envelope_base64")
	}
	if bundle.Protocol != nil {
		if bundle.Protocol.ProtocolPCID == "" || bundle.Protocol.DocumentBytes == "" {
			return fmt.Errorf("bundle protocol support incomplete")
		}
	}
	if bundle.Signer != nil {
		if bundle.Signer.Name == "" || bundle.Signer.KeyID == "" || bundle.Signer.PublicKey == "" {
			return fmt.Errorf("bundle signer support incomplete")
		}
	}
	return nil
}
