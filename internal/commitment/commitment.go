package commitment

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"commitment-ledger/internal/ledger"
	"commitment-ledger/internal/model"
)

const dueDateLayout = "2006-01-02"

func Create(store *ledger.Store, promiser string, repo string, branch string, targets []string, dueDate string, promise string, now time.Time) (model.Commitment, error) {
	promiser = strings.TrimSpace(promiser)
	promise = strings.TrimSpace(promise)
	if promiser == "" {
		return model.Commitment{}, fmt.Errorf("promiser is required")
	}
	if repo == "" || branch == "" {
		return model.Commitment{}, fmt.Errorf("repo and branch are required")
	}
	if len(targets) == 0 {
		return model.Commitment{}, fmt.Errorf("at least one target is required")
	}
	if promise == "" {
		return model.Commitment{}, fmt.Errorf("promise text is required")
	}
	if _, err := time.Parse(dueDateLayout, dueDate); err != nil {
		return model.Commitment{}, fmt.Errorf("invalid due date %q: %w", dueDate, err)
	}

	workItems, err := store.LoadLatestWorkItems()
	if err != nil {
		return model.Commitment{}, err
	}
	for _, target := range targets {
		targetRepo, targetBranch, _, ok := model.SplitTarget(target)
		if !ok {
			return model.Commitment{}, fmt.Errorf("invalid target %q", target)
		}
		if targetRepo != repo || targetBranch != branch {
			return model.Commitment{}, fmt.Errorf("target %q does not match repo=%s branch=%s", target, repo, branch)
		}
		if _, ok := workItems[target]; !ok {
			return model.Commitment{}, fmt.Errorf("unknown target %q; run scan first", target)
		}
	}

	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return model.Commitment{}, err
	}

	id := nextCommitmentID(commitments, promiser, now)
	return model.Commitment{
		CommitmentID: id,
		Promiser:     promiser,
		Repo:         repo,
		Branch:       branch,
		Targets:      append([]string(nil), targets...),
		PromiseText:  promise,
		CreatedAt:    now.Format(time.RFC3339),
		DueDate:      dueDate,
		Status:       model.StatusOpen,
	}, nil
}

func ExpireDue(commitments map[string]model.Commitment, now time.Time) []model.Commitment {
	var updates []model.Commitment
	today := now.Format(dueDateLayout)
	for _, item := range commitments {
		if item.Status != model.StatusOpen {
			continue
		}
		if item.DueDate < today {
			item.Status = model.StatusExpiredUnassessed
			updates = append(updates, item)
		}
	}
	sort.Slice(updates, func(i, j int) bool {
		return updates[i].CommitmentID < updates[j].CommitmentID
	})
	return updates
}

func ValidAssessmentStatus(status string) bool {
	if status == model.StatusOpen {
		return false
	}
	_, ok := model.CommitmentStatuses[status]
	return ok
}

func nextCommitmentID(existing map[string]model.Commitment, promiser string, now time.Time) string {
	datePart := now.Format("20060102")
	promiserPart := slug(promiser)
	maxSeq := 0
	prefix := "COMMITMENT-" + datePart + "-" + promiserPart + "-"
	for id := range existing {
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		var seq int
		if _, err := fmt.Sscanf(id, prefix+"%03d", &seq); err == nil && seq > maxSeq {
			maxSeq = seq
		}
	}
	return fmt.Sprintf("%s%03d", prefix, maxSeq+1)
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "anon"
	}
	return b.String()
}
