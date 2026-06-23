package assessment

import (
	"strings"
	"testing"
	"time"

	"commitment-ledger/internal/model"
)

func TestCreateRejectsAssessmentOfFinalizedCommitment(t *testing.T) {
	current := model.Commitment{
		CommitmentID: "COMMITMENT-20260623-jj-001",
		Status:       model.StatusKept,
	}

	_, _, err := Create(nil, current, "JJ", model.StatusBroken, nil, "reassess", time.Now())
	if err == nil || !strings.Contains(err.Error(), "cannot assess commitment") {
		t.Fatalf("Create error = %v, want finalized commitment rejection", err)
	}
}
