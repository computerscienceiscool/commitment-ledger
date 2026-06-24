package protocol

type CommitmentPromisePayload struct {
	Kind        string            `json:"kind"`
	Promiser    string            `json:"promiser"`
	Promisee    string            `json:"promisee,omitempty"`
	Repo        string            `json:"repo"`
	Branch      string            `json:"branch"`
	Targets     []string          `json:"targets"`
	PromiseText string            `json:"promise_text"`
	DueDate     string            `json:"due_date"`
	CreatedAt   string            `json:"created_at"`
	Supersedes  []string          `json:"supersedes,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type CommitmentEvidencePayload struct {
	Kind             string `json:"kind"`
	CommitmentRef    string `json:"commitment_ref"`
	Observer         string `json:"observer"`
	Repo             string `json:"repo"`
	Branch           string `json:"branch"`
	SourceCommit     string `json:"source_commit,omitempty"`
	Target           string `json:"target,omitempty"`
	EvidenceKind     string `json:"evidence_kind"`
	ObservedAt       string `json:"observed_at"`
	ObservedBytesCID string `json:"observed_bytes_cid,omitempty"`
	Notes            string `json:"notes,omitempty"`
}

type CommitmentAssessmentPayload struct {
	Kind          string   `json:"kind"`
	CommitmentRef string   `json:"commitment_ref"`
	Assessor      string   `json:"assessor"`
	Status        string   `json:"status"`
	AssessedAt    string   `json:"assessed_at"`
	Basis         []string `json:"basis,omitempty"`
	Notes         string   `json:"notes,omitempty"`
}

type ExchangeReceiptPayload struct {
	Kind                    string `json:"kind"`
	ReceiptID               string `json:"receipt_id"`
	ReceivedArtifactCID     string `json:"received_artifact_cid"`
	RelatedID               string `json:"related_id,omitempty"`
	SourcePath              string `json:"source_path"`
	Receiver                string `json:"receiver"`
	ReceivedAt              string `json:"received_at"`
	SupportInstalled        bool   `json:"support_installed"`
	InstalledProtocolPCID   string `json:"installed_protocol_pcid,omitempty"`
	InstalledSignerIdentity string `json:"installed_signer_identity,omitempty"`
}

type ImplementationConformancePayload struct {
	Kind                    string   `json:"kind"`
	Implementation          string   `json:"implementation"`
	Version                 string   `json:"version"`
	ClaimedProtocolPCIDs    []string `json:"claimed_protocol_pcids"`
	EmittedProtocolPCIDs    []string `json:"emitted_protocol_pcids,omitempty"`
	HistoricalProtocolPCIDs []string `json:"historical_protocol_pcids,omitempty"`
	ProjectionRules         []string `json:"projection_rules"`
	ClaimedAt               string   `json:"claimed_at"`
}
