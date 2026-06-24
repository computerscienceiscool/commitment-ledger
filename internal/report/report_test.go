package report

import (
	"testing"

	"commitment-ledger/internal/model"
)

func TestRepoSummariesCountNonKeptOutcomes(t *testing.T) {
	workItems := map[string]model.WorkItem{
		"repo/main/TODO-ravud": {Repo: "repo", Branch: "main", WorkID: "TODO-ravud", Status: "open"},
	}
	commitments := map[string]model.Commitment{
		"a": {Repo: "repo", Branch: "main", Status: model.StatusKept},
		"b": {Repo: "repo", Branch: "main", Status: model.StatusPartiallyKept},
		"c": {Repo: "repo", Branch: "main", Status: model.StatusBroken},
		"d": {Repo: "repo", Branch: "main", Status: model.StatusRefused},
		"e": {Repo: "repo", Branch: "main", Status: model.StatusDelegated},
		"f": {Repo: "repo", Branch: "main", Status: model.StatusSuperseded},
		"g": {Repo: "repo", Branch: "main", Status: model.StatusExtended},
	}

	summaries := RepoSummaries(workItems, commitments)
	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}
	got := summaries[0]
	if got.Kept != 1 || got.PartiallyKept != 1 || got.Broken != 1 || got.Refused != 1 || got.Delegated != 1 || got.Superseded != 1 || got.Extended != 1 {
		t.Fatalf("unexpected repo summary counts: %+v", got)
	}
}

func TestFindWorkSummaryUsesParentStatusForSubtaskQueries(t *testing.T) {
	workItems := map[string]model.WorkItem{
		"repo/main/TODO-ravud": {
			Repo:   "repo",
			Branch: "main",
			WorkID: "TODO-ravud",
			Status: "open",
		},
		"repo/main/TODO-ravud/1": {
			Repo:       "repo",
			Branch:     "main",
			WorkID:     "TODO-ravud/1",
			ParentWork: "TODO-ravud",
			Status:     "complete",
			IsSubtask:  true,
		},
	}

	summary, err := FindWorkSummary("repo/main/TODO-ravud/1", workItems, nil)
	if err != nil {
		t.Fatalf("FindWorkSummary: %v", err)
	}
	if summary.Target != "repo/main/TODO-ravud" {
		t.Fatalf("target = %q, want parent target", summary.Target)
	}
	if summary.Status != "open" {
		t.Fatalf("status = %q, want parent status open", summary.Status)
	}
}

func TestPersonSummariesCountAllTerminalOutcomes(t *testing.T) {
	commitments := map[string]model.Commitment{
		"a": {Promiser: "Alice", Status: model.StatusOpen},
		"b": {Promiser: "Alice", Status: model.StatusKept},
		"c": {Promiser: "Alice", Status: model.StatusPartiallyKept},
		"d": {Promiser: "Alice", Status: model.StatusExpiredUnassessed},
		"e": {Promiser: "Alice", Status: model.StatusBroken},
		"f": {Promiser: "Alice", Status: model.StatusRefused},
		"g": {Promiser: "Alice", Status: model.StatusDelegated},
		"h": {Promiser: "Alice", Status: model.StatusSuperseded},
		"i": {Promiser: "Alice", Status: model.StatusExtended},
	}

	summaries := PersonSummaries(commitments)
	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}
	got := summaries[0]
	if got.OpenCommitments != 1 || got.Kept != 1 || got.PartiallyKept != 1 || got.ExpiredUnassessed != 1 || got.Broken != 1 || got.Refused != 1 || got.Delegated != 1 || got.Superseded != 1 || got.Extended != 1 {
		t.Fatalf("unexpected person summary counts: %+v", got)
	}
}
