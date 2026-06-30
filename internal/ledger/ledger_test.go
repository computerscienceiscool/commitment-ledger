package ledger

import (
	"encoding/json"
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

func TestCommitmentIndexIsRecoverableFromReferenceSet(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	commitment := model.Commitment{
		CommitmentID: "COMMITMENT-20260629-jj-001",
		Promiser:     "JJ",
		Repo:         "repo",
		Branch:       "main",
		Targets:      []string{"repo/main/TODO-ravud/1"},
		PromiseText:  "I promise to finish subtask 1.",
		CreatedAt:    "2026-06-29T11:00:00-07:00",
		DueDate:      "2026-07-05",
		Status:       model.StatusOpen,
	}
	if err := store.AppendCommitment(commitment); err != nil {
		t.Fatalf("append commitment: %v", err)
	}

	setPath := filepath.Join(root, "data", "refs", "reference-sets", commitmentStateReferenceSet+".json")
	body, err := os.ReadFile(setPath)
	if err != nil {
		t.Fatalf("read reference set cache: %v", err)
	}
	var set referenceSet
	if err := json.Unmarshal(body, &set); err != nil {
		t.Fatalf("decode reference set cache: %v", err)
	}
	entry, ok := set.Entries["commitments-latest"]
	if !ok || entry.CID == "" {
		t.Fatalf("reference set missing commitments-latest entry: %+v", set.Entries)
	}

	if err := os.Remove(filepath.Join(root, "data", "refs", "commitments-latest.ref")); err != nil {
		t.Fatalf("remove loose ref: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "data", "indexes", "commitment-state", "commitments-latest.json")); err != nil {
		t.Fatalf("remove structured cache: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "data", "indexes", "commitments-latest.json")); err != nil {
		t.Fatalf("remove legacy cache: %v", err)
	}

	items, err := store.LoadLatestCommitments()
	if err != nil {
		t.Fatalf("LoadLatestCommitments: %v", err)
	}
	if got, ok := items[commitment.CommitmentID]; !ok || got.PromiseText != commitment.PromiseText {
		t.Fatalf("reference-set recovery failed, got=%+v", items)
	}
}

func TestWorkItemsIndexUsesWorkObservationReferenceSet(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	items := []model.WorkItem{
		{Repo: "repo", Branch: "main", WorkID: "TODO-ravud", Status: "open"},
	}
	if err := store.AppendWorkItems(items); err != nil {
		t.Fatalf("append work items: %v", err)
	}

	workSetPath := filepath.Join(root, "data", "refs", "reference-sets", workObservationReferenceSet+".json")
	body, err := os.ReadFile(workSetPath)
	if err != nil {
		t.Fatalf("read work observation reference set cache: %v", err)
	}
	var workSet referenceSet
	if err := json.Unmarshal(body, &workSet); err != nil {
		t.Fatalf("decode work observation reference set cache: %v", err)
	}
	if entry, ok := workSet.Entries["work-items-latest"]; !ok || entry.CID == "" {
		t.Fatalf("work observation reference set missing work-items-latest entry: %+v", workSet.Entries)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "indexes", "work-observation", "work-items-latest.json")); err != nil {
		t.Fatalf("expected structured work observation cache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "indexes", "work-items-latest.json")); err != nil {
		t.Fatalf("expected legacy work item cache: %v", err)
	}

	legacySetPath := filepath.Join(root, "data", "refs", "reference-sets", legacyReferenceSetName+".json")
	if _, err := os.Stat(legacySetPath); !os.IsNotExist(err) {
		t.Fatalf("expected no legacy reference set cache, stat err=%v", err)
	}
}

func TestLoadLatestCommitmentsFallsBackToLegacyReferenceSet(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	snapshot := casIndexSnapshot[model.Commitment]{
		Version: "cas-index-v1",
		Items: []model.Commitment{
			{
				CommitmentID: "COMMITMENT-20260629-jj-legacy",
				Promiser:     "JJ",
				Repo:         "repo",
				Branch:       "main",
				Targets:      []string{"repo/main/TODO-ravud/1"},
				PromiseText:  "I promise to finish subtask 1.",
				CreatedAt:    "2026-06-29T12:00:00-07:00",
				DueDate:      "2026-07-05",
				Status:       model.StatusOpen,
			},
		},
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	id, err := store.CAS.Put(body)
	if err != nil {
		t.Fatalf("store snapshot in cas: %v", err)
	}
	legacySet := referenceSet{
		Version: "reference-set-v1",
		Name:    legacyReferenceSetName,
		Entries: map[string]referenceEntry{
			"commitments-latest": {CID: id},
		},
	}
	if err := store.persistReferenceSet(legacyReferenceSetName, legacySet); err != nil {
		t.Fatalf("persist legacy reference set: %v", err)
	}

	items, err := store.LoadLatestCommitments()
	if err != nil {
		t.Fatalf("LoadLatestCommitments: %v", err)
	}
	got, ok := items["COMMITMENT-20260629-jj-legacy"]
	if !ok || got.PromiseText != "I promise to finish subtask 1." {
		t.Fatalf("legacy reference-set fallback failed, got=%+v", items)
	}
}
