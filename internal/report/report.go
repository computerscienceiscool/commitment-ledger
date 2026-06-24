package report

import (
	"fmt"
	"sort"
	"strings"

	"commitment-ledger/internal/model"
)

type RepoSummary struct {
	Repo              string `json:"repo"`
	Branch            string `json:"branch"`
	OpenTODOs         int    `json:"open_todos"`
	OpenSubtasks      int    `json:"open_subtasks"`
	ActiveCommitments int    `json:"active_commitments"`
	Expired           int    `json:"expired"`
	Kept              int    `json:"kept"`
	PartiallyKept     int    `json:"partially_kept"`
	Broken            int    `json:"broken"`
	Refused           int    `json:"refused"`
	Delegated         int    `json:"delegated"`
	Superseded        int    `json:"superseded"`
	Extended          int    `json:"extended"`
}

type PersonSummary struct {
	Promiser          string `json:"promiser"`
	OpenCommitments   int    `json:"open_commitments"`
	Kept              int    `json:"kept"`
	PartiallyKept     int    `json:"partially_kept"`
	ExpiredUnassessed int    `json:"expired_unassessed"`
	Broken            int    `json:"broken"`
	Refused           int    `json:"refused"`
	Delegated         int    `json:"delegated"`
	Superseded        int    `json:"superseded"`
	Extended          int    `json:"extended"`
}

type WorkSummary struct {
	Target            string             `json:"target"`
	Status            string             `json:"status"`
	Subtasks          int                `json:"subtasks"`
	CompletedSubtasks int                `json:"completed_subtasks"`
	Commitments       []model.Commitment `json:"commitments"`
}

func RepoSummaries(workItems map[string]model.WorkItem, commitments map[string]model.Commitment) []RepoSummary {
	summaries := map[string]*RepoSummary{}
	for _, item := range workItems {
		key := item.Repo + "/" + item.Branch
		summary := summaries[key]
		if summary == nil {
			summary = &RepoSummary{Repo: item.Repo, Branch: item.Branch}
			summaries[key] = summary
		}
		if item.IsSubtask {
			if item.Status == "open" {
				summary.OpenSubtasks++
			}
			continue
		}
		if item.Status == "open" {
			summary.OpenTODOs++
		}
	}
	for _, item := range commitments {
		key := item.Repo + "/" + item.Branch
		summary := summaries[key]
		if summary == nil {
			summary = &RepoSummary{Repo: item.Repo, Branch: item.Branch}
			summaries[key] = summary
		}
		switch item.Status {
		case model.StatusOpen:
			summary.ActiveCommitments++
		case model.StatusExpiredUnassessed:
			summary.Expired++
		case model.StatusKept:
			summary.Kept++
		case model.StatusPartiallyKept:
			summary.PartiallyKept++
		case model.StatusBroken:
			summary.Broken++
		case model.StatusRefused:
			summary.Refused++
		case model.StatusDelegated:
			summary.Delegated++
		case model.StatusSuperseded:
			summary.Superseded++
		case model.StatusExtended:
			summary.Extended++
		}
	}
	out := make([]RepoSummary, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Repo == out[j].Repo {
			return out[i].Branch < out[j].Branch
		}
		return out[i].Repo < out[j].Repo
	})
	return out
}

func PersonSummaries(commitments map[string]model.Commitment) []PersonSummary {
	summaries := map[string]*PersonSummary{}
	for _, item := range commitments {
		summary := summaries[item.Promiser]
		if summary == nil {
			summary = &PersonSummary{Promiser: item.Promiser}
			summaries[item.Promiser] = summary
		}
		switch item.Status {
		case model.StatusOpen:
			summary.OpenCommitments++
		case model.StatusKept:
			summary.Kept++
		case model.StatusPartiallyKept:
			summary.PartiallyKept++
		case model.StatusExpiredUnassessed:
			summary.ExpiredUnassessed++
		case model.StatusBroken:
			summary.Broken++
		case model.StatusRefused:
			summary.Refused++
		case model.StatusDelegated:
			summary.Delegated++
		case model.StatusSuperseded:
			summary.Superseded++
		case model.StatusExtended:
			summary.Extended++
		}
	}
	out := make([]PersonSummary, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Promiser < out[j].Promiser })
	return out
}

func FindWorkSummary(target string, workItems map[string]model.WorkItem, commitments map[string]model.Commitment) (WorkSummary, error) {
	item, ok := workItems[target]
	if !ok {
		return WorkSummary{}, fmt.Errorf("unknown work target %q", target)
	}

	parentID := item.WorkID
	status := item.Status
	if item.IsSubtask {
		parentID = item.ParentWork
		if parent, ok := workItems[model.WorkTarget(item.Repo, item.Branch, parentID)]; ok {
			status = parent.Status
		}
	}

	summary := WorkSummary{
		Target: model.WorkTarget(item.Repo, item.Branch, parentID),
		Status: status,
	}
	for _, candidate := range workItems {
		if candidate.Repo != item.Repo || candidate.Branch != item.Branch || candidate.ParentWork != parentID {
			continue
		}
		summary.Subtasks++
		if candidate.Status == "complete" {
			summary.CompletedSubtasks++
		}
	}
	for _, candidate := range commitments {
		for _, commitmentTarget := range candidate.Targets {
			if commitmentTarget == summary.Target || strings.HasPrefix(commitmentTarget, summary.Target+"/") {
				summary.Commitments = append(summary.Commitments, candidate)
				break
			}
		}
	}
	sort.Slice(summary.Commitments, func(i, j int) bool {
		return summary.Commitments[i].CommitmentID < summary.Commitments[j].CommitmentID
	})
	return summary, nil
}
