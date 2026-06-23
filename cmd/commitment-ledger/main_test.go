package main

import (
	"strings"
	"testing"

	"commitment-ledger/internal/ledger"
	"commitment-ledger/internal/model"
)

func TestResolveBasisMapsAndValidatesCommitmentEvidence(t *testing.T) {
	evidenceItems := []model.Evidence{
		{
			EvidenceID:   "EVIDENCE-20260623-001",
			ArtifactCID:  "bafy-e1",
			CommitmentID: "COMMITMENT-1",
		},
		{
			EvidenceID:   "EVIDENCE-20260623-002",
			ArtifactCID:  "bafy-e2",
			CommitmentID: "COMMITMENT-2",
		},
	}

	got, err := resolveBasis([]string{"EVIDENCE-20260623-001", "bafy-e1"}, evidenceItems, "COMMITMENT-1")
	if err != nil {
		t.Fatalf("resolveBasis: %v", err)
	}
	if len(got) != 1 || got[0] != "bafy-e1" {
		t.Fatalf("resolved basis = %#v, want [\"bafy-e1\"]", got)
	}

	_, err = resolveBasis([]string{"EVIDENCE-20260623-002"}, evidenceItems, "COMMITMENT-1")
	if err == nil || !strings.Contains(err.Error(), "belongs to commitment") {
		t.Fatalf("foreign evidence error = %v, want commitment mismatch", err)
	}

	_, err = resolveBasis([]string{"does-not-exist"}, evidenceItems, "COMMITMENT-1")
	if err == nil || !strings.Contains(err.Error(), "unknown basis reference") {
		t.Fatalf("unknown basis error = %v, want unknown basis reference", err)
	}
}

func TestValidateEvidenceInputRejectsMismatches(t *testing.T) {
	store := ledger.NewStore(t.TempDir())
	workItems := []model.WorkItem{
		{Repo: "repo", Branch: "main", WorkID: "TODO-ravud", Status: model.StatusOpen},
		{Repo: "repo", Branch: "main", WorkID: "TODO-ravud/1", ParentWork: "TODO-ravud", Status: model.StatusOpen, IsSubtask: true},
	}
	if err := store.AppendWorkItems(workItems); err != nil {
		t.Fatalf("append work items: %v", err)
	}

	current := model.Commitment{
		CommitmentID: "COMMITMENT-1",
		Repo:         "repo",
		Branch:       "main",
		Targets:      []string{"repo/main/TODO-ravud"},
	}

	repo, branch, err := validateEvidenceInput(store, current, "", "", "repo/main/TODO-ravud/1")
	if err != nil {
		t.Fatalf("validateEvidenceInput accepted descendant target: %v", err)
	}
	if repo != "repo" || branch != "main" {
		t.Fatalf("repo/branch = %s/%s, want repo/main", repo, branch)
	}

	_, _, err = validateEvidenceInput(store, current, "other", "main", "")
	if err == nil || !strings.Contains(err.Error(), "must match commitment") {
		t.Fatalf("repo mismatch error = %v, want repo/branch mismatch", err)
	}

	_, _, err = validateEvidenceInput(store, current, "", "", "repo/main/TODO-other")
	if err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("unknown target error = %v, want unknown target", err)
	}
}
