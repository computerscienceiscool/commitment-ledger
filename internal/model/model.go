package model

import "strings"

const (
	StatusOpen                = "open"
	StatusKept                = "kept"
	StatusPartiallyKept       = "partially_kept"
	StatusBroken              = "broken"
	StatusRefused             = "refused"
	StatusDelegated           = "delegated"
	StatusSuperseded          = "superseded"
	StatusExtended            = "extended"
	StatusExpiredUnassessed   = "expired_unassessed"
	EvidenceTypeTodoChecked   = "todo_checked"
	EvidenceTypeTodoUnchecked = "todo_unchecked"
	EvidenceTypeCommitSeen    = "commit_seen"
	EvidenceTypeManualNote    = "manual_note"
	EvidenceTypeHumanReview   = "human_review"
	EvidenceTypeLLMReview     = "llm_review"
	EvidenceTypeAssessment    = "assessment"
)

var CommitmentStatuses = map[string]struct{}{
	StatusOpen:              {},
	StatusKept:              {},
	StatusPartiallyKept:     {},
	StatusBroken:            {},
	StatusRefused:           {},
	StatusDelegated:         {},
	StatusSuperseded:        {},
	StatusExtended:          {},
	StatusExpiredUnassessed: {},
}

type WorkItem struct {
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	Commit     string `json:"commit"`
	WorkID     string `json:"work_id"`
	ParentWork string `json:"parent_work_id,omitempty"`
	Title      string `json:"title"`
	DetailFile string `json:"detail_file,omitempty"`
	Status     string `json:"status"`
	SourceFile string `json:"source_file"`
	FirstSeen  string `json:"first_seen"`
	LastSeen   string `json:"last_seen"`
	IsSubtask  bool   `json:"is_subtask,omitempty"`
	Removed    bool   `json:"removed,omitempty"`
}

type Commitment struct {
	CommitmentID string   `json:"commitment_id"`
	ArtifactCID  string   `json:"artifact_cid,omitempty"`
	ProtocolPCID string   `json:"protocol_pcid,omitempty"`
	Signer       string   `json:"signer,omitempty"`
	SignerKeyID  string   `json:"signer_key_id,omitempty"`
	Promiser     string   `json:"promiser"`
	Repo         string   `json:"repo"`
	Branch       string   `json:"branch"`
	Targets      []string `json:"targets"`
	PromiseText  string   `json:"promise_text"`
	CreatedAt    string   `json:"created_at"`
	DueDate      string   `json:"due_date"`
	Status       string   `json:"status"`
}

type Evidence struct {
	EvidenceID       string `json:"evidence_id"`
	ArtifactCID      string `json:"artifact_cid,omitempty"`
	ProtocolPCID     string `json:"protocol_pcid,omitempty"`
	Signer           string `json:"signer,omitempty"`
	SignerKeyID      string `json:"signer_key_id,omitempty"`
	CommitmentID     string `json:"commitment_id"`
	CommitmentRef    string `json:"commitment_ref,omitempty"`
	EvidenceType     string `json:"evidence_type"`
	Repo             string `json:"repo"`
	Branch           string `json:"branch"`
	Commit           string `json:"commit"`
	Target           string `json:"target,omitempty"`
	ObservedAt       string `json:"observed_at"`
	ObservedBytesCID string `json:"observed_bytes_cid,omitempty"`
	Notes            string `json:"notes"`
}

type Assessment struct {
	AssessmentID  string   `json:"assessment_id"`
	ArtifactCID   string   `json:"artifact_cid,omitempty"`
	ProtocolPCID  string   `json:"protocol_pcid,omitempty"`
	Signer        string   `json:"signer,omitempty"`
	SignerKeyID   string   `json:"signer_key_id,omitempty"`
	CommitmentID  string   `json:"commitment_id"`
	CommitmentRef string   `json:"commitment_ref,omitempty"`
	Assessor      string   `json:"assessor"`
	Status        string   `json:"status"`
	AssessedAt    string   `json:"assessed_at"`
	Basis         []string `json:"basis"`
	Notes         string   `json:"notes"`
}

type ArtifactRecord struct {
	ArtifactCID  string `json:"artifact_cid"`
	ProtocolPCID string `json:"protocol_pcid"`
	Kind         string `json:"kind"`
	Signer       string `json:"signer"`
	SignerKeyID  string `json:"signer_key_id"`
	PayloadCID   string `json:"payload_cid"`
	ProofCID     string `json:"proof_cid"`
	ObservedAt   string `json:"observed_at"`
	RelatedID    string `json:"related_id,omitempty"`
	RelatedCID   string `json:"related_cid,omitempty"`
}

type Snapshot struct {
	Repo               string `json:"repo"`
	Branch             string `json:"branch"`
	Commit             string `json:"commit"`
	ObservedAt         string `json:"observed_at"`
	OpenWork           int    `json:"open_work"`
	CompletedWork      int    `json:"completed_work"`
	SubtasksDiscovered int    `json:"subtasks_discovered"`
}

func WorkTarget(repo, branch, workID string) string {
	return repo + "/" + branch + "/" + workID
}

func SplitTarget(target string) (repo string, branch string, workID string, ok bool) {
	parts := strings.SplitN(target, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func IsTerminalStatus(status string) bool {
	switch status {
	case StatusKept, StatusPartiallyKept, StatusBroken, StatusRefused, StatusDelegated, StatusSuperseded:
		return true
	default:
		return false
	}
}
