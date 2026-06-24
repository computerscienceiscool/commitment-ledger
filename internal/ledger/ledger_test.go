package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"commitment-ledger/internal/model"
)

func TestStoreWritesJSONLAndMarkdown(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	commitment := model.Commitment{
		CommitmentID: "COMMITMENT-20260622-jj-001",
		Promiser:     "JJ",
		Repo:         "repo",
		Branch:       "main",
		Targets:      []string{"repo/main/TODO-ravud/1"},
		PromiseText:  "I promise to finish subtask 1.",
		CreatedAt:    "2026-06-22T11:00:00-07:00",
		DueDate:      "2026-06-28",
		Status:       model.StatusOpen,
	}
	if err := store.AppendCommitment(commitment); err != nil {
		t.Fatalf("append commitment: %v", err)
	}

	evidence := model.Evidence{
		EvidenceID:   "EVIDENCE-20260622-001",
		CommitmentID: commitment.CommitmentID,
		EvidenceType: model.EvidenceTypeTodoChecked,
		Repo:         "repo",
		Branch:       "main",
		Commit:       "abc123",
		Target:       "repo/main/TODO-ravud/1",
		ObservedAt:   "2026-06-22T12:00:00-07:00",
		Notes:        "Checked off in detail file.",
	}
	if err := store.AppendEvidence(evidence); err != nil {
		t.Fatalf("append evidence: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "records", "commitments", commitment.CommitmentID+".md"))
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "Checked off in detail file.") {
		t.Fatalf("markdown missing evidence note:\n%s", text)
	}

	items, err := store.LoadEvidenceForCommitment(commitment.CommitmentID)
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d evidence items, want 1", len(items))
	}
}

func TestLoadLatestWorkItemsDropsRemovedTargets(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	initial := []model.WorkItem{
		{Repo: "repo", Branch: "main", WorkID: "TODO-ravud", Status: "open"},
		{Repo: "repo", Branch: "main", WorkID: "TODO-ravud/1", ParentWork: "TODO-ravud", Status: "open", IsSubtask: true},
	}
	if err := store.AppendWorkItems(initial); err != nil {
		t.Fatalf("append initial work items: %v", err)
	}

	removed := []model.WorkItem{
		{Repo: "repo", Branch: "main", WorkID: "TODO-ravud/1", ParentWork: "TODO-ravud", Status: "open", IsSubtask: true, Removed: true},
	}
	if err := store.AppendWorkItems(removed); err != nil {
		t.Fatalf("append removed work item: %v", err)
	}

	items, err := store.LoadLatestWorkItems()
	if err != nil {
		t.Fatalf("load latest work items: %v", err)
	}
	if _, ok := items["repo/main/TODO-ravud/1"]; ok {
		t.Fatal("expected removed subtask to be absent from latest work items")
	}
	if _, ok := items["repo/main/TODO-ravud"]; !ok {
		t.Fatal("expected parent TODO to remain present")
	}
}
