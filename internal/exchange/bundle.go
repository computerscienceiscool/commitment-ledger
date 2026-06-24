package exchange

import "commitment-ledger/internal/model"

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
