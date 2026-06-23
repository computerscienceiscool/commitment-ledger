package assessment

import (
	"fmt"
	"time"

	"commitment-ledger/internal/commitment"
	"commitment-ledger/internal/model"
)

func Create(existing []model.Assessment, current model.Commitment, assessor string, status string, basis []string, notes string, now time.Time) (model.Assessment, model.Commitment, error) {
	if assessor == "" {
		return model.Assessment{}, model.Commitment{}, fmt.Errorf("assessor is required")
	}
	if !commitment.ValidAssessmentStatus(status) {
		return model.Assessment{}, model.Commitment{}, fmt.Errorf("invalid assessment status %q", status)
	}
	if current.Status != model.StatusOpen && current.Status != model.StatusExpiredUnassessed {
		return model.Assessment{}, model.Commitment{}, fmt.Errorf("cannot assess commitment %q in status %q", current.CommitmentID, current.Status)
	}

	assessment := model.Assessment{
		AssessmentID: nextAssessmentID(existing, now),
		CommitmentID: current.CommitmentID,
		Assessor:     assessor,
		Status:       status,
		AssessedAt:   now.Format(time.RFC3339),
		Basis:        append([]string(nil), basis...),
		Notes:        notes,
	}
	current.Status = status
	return assessment, current, nil
}

func nextAssessmentID(existing []model.Assessment, now time.Time) string {
	prefix := "ASSESSMENT-" + now.Format("20060102") + "-"
	maxSeq := 0
	for _, item := range existing {
		var seq int
		if _, err := fmt.Sscanf(item.AssessmentID, prefix+"%03d", &seq); err == nil && seq > maxSeq {
			maxSeq = seq
		}
	}
	return fmt.Sprintf("%s%03d", prefix, maxSeq+1)
}
