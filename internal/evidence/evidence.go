package evidence

import (
	"fmt"
	"time"

	"commitment-ledger/internal/model"
)

func Derive(repo string, branch string, commit string, workItems map[string]model.WorkItem, commitments map[string]model.Commitment, existing []model.Evidence, now time.Time) []model.Evidence {
	seen := existingKeys(existing)
	nextSeq := nextEvidenceSeq(existing, now)
	observedAt := now.Format(time.RFC3339)

	var derived []model.Evidence
	for _, commitment := range commitments {
		if commitment.Repo != repo || commitment.Branch != branch {
			continue
		}
		if commitment.Status != model.StatusOpen && commitment.Status != model.StatusExpiredUnassessed {
			continue
		}

		commitKey := evidenceKey(commitment.CommitmentID, model.EvidenceTypeCommitSeen, repo, branch, commit, "", "Observed repo commit during scan.")
		if _, ok := seen[commitKey]; !ok {
			item := model.Evidence{
				EvidenceID:   fmt.Sprintf("EVIDENCE-%s-%03d", now.Format("20060102"), nextSeq),
				CommitmentID: commitment.CommitmentID,
				EvidenceType: model.EvidenceTypeCommitSeen,
				Repo:         repo,
				Branch:       branch,
				Commit:       commit,
				ObservedAt:   observedAt,
				Notes:        "Observed repo commit during scan.",
			}
			nextSeq++
			derived = append(derived, item)
			seen[commitKey] = struct{}{}
		}

		for _, target := range commitment.Targets {
			item, ok := workItems[target]
			if !ok || item.Status != "complete" {
				continue
			}
			note := "Target marked complete in observed TODO state."
			key := evidenceKey(commitment.CommitmentID, model.EvidenceTypeTodoChecked, repo, branch, commit, target, note)
			if _, ok := seen[key]; ok {
				continue
			}
			derived = append(derived, model.Evidence{
				EvidenceID:   fmt.Sprintf("EVIDENCE-%s-%03d", now.Format("20060102"), nextSeq),
				CommitmentID: commitment.CommitmentID,
				EvidenceType: model.EvidenceTypeTodoChecked,
				Repo:         repo,
				Branch:       branch,
				Commit:       commit,
				Target:       target,
				ObservedAt:   observedAt,
				Notes:        note,
			})
			nextSeq++
			seen[key] = struct{}{}
		}
	}
	return derived
}

func NewManual(existing []model.Evidence, commitmentID string, evidenceType string, repo string, branch string, commit string, target string, notes string, now time.Time) model.Evidence {
	seq := nextEvidenceSeq(existing, now)
	return model.Evidence{
		EvidenceID:   fmt.Sprintf("EVIDENCE-%s-%03d", now.Format("20060102"), seq),
		CommitmentID: commitmentID,
		EvidenceType: evidenceType,
		Repo:         repo,
		Branch:       branch,
		Commit:       commit,
		Target:       target,
		ObservedAt:   now.Format(time.RFC3339),
		Notes:        notes,
	}
}

func nextEvidenceSeq(existing []model.Evidence, now time.Time) int {
	prefix := "EVIDENCE-" + now.Format("20060102") + "-"
	maxSeq := 0
	for _, item := range existing {
		var seq int
		if _, err := fmt.Sscanf(item.EvidenceID, prefix+"%03d", &seq); err == nil && seq > maxSeq {
			maxSeq = seq
		}
	}
	return maxSeq + 1
}

func existingKeys(items []model.Evidence) map[string]struct{} {
	keys := make(map[string]struct{}, len(items))
	for _, item := range items {
		keys[evidenceKey(item.CommitmentID, item.EvidenceType, item.Repo, item.Branch, item.Commit, item.Target, item.Notes)] = struct{}{}
	}
	return keys
}

func evidenceKey(commitmentID string, evidenceType string, repo string, branch string, commit string, target string, notes string) string {
	return commitmentID + "|" + evidenceType + "|" + repo + "|" + branch + "|" + commit + "|" + target + "|" + notes
}
