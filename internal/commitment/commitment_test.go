package commitment

import (
	"testing"
	"time"

	"commitment-ledger/internal/ledger"
	"commitment-ledger/internal/model"
)

func TestExpireDueUpdatesOnlyOverdueOpenCommitments(t *testing.T) {
	commitments := map[string]model.Commitment{
		"a": {CommitmentID: "a", DueDate: "2026-06-20", Status: model.StatusOpen},
		"b": {CommitmentID: "b", DueDate: "2026-06-22", Status: model.StatusOpen},
		"c": {CommitmentID: "c", DueDate: "2026-06-19", Status: model.StatusKept},
	}

	updates := ExpireDue(commitments, time.Date(2026, 6, 22, 11, 0, 0, 0, time.FixedZone("PDT", -7*3600)))
	if len(updates) != 1 {
		t.Fatalf("got %d updates, want 1", len(updates))
	}
	if updates[0].CommitmentID != "a" || updates[0].Status != model.StatusExpiredUnassessed {
		t.Fatalf("unexpected update: %+v", updates[0])
	}
}

func TestCreateRejectsUnknownTarget(t *testing.T) {
	store := ledger.NewStore(t.TempDir())
	_, err := Create(store, "JJ", "repo", "main", []string{"repo/main/TODO-binap"}, "2026-06-28", "I promise.", time.Now())
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}
