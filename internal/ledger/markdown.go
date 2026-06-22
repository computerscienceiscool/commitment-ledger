package ledger

import (
	"fmt"
	"strings"

	"commitment-ledger/internal/model"
)

func CommitmentMarkdown(commitment model.Commitment, evidence []model.Evidence) string {
	var b strings.Builder
	b.WriteString("# " + commitment.CommitmentID + "\n\n")
	b.WriteString("Promiser: " + commitment.Promiser + "\n")
	b.WriteString("Repo: " + commitment.Repo + "\n")
	b.WriteString("Branch: " + commitment.Branch + "\n")
	if commitment.ArtifactCID != "" {
		b.WriteString("Artifact CID: " + commitment.ArtifactCID + "\n")
	}
	if commitment.ProtocolPCID != "" {
		b.WriteString("Protocol pCID: " + commitment.ProtocolPCID + "\n")
	}
	b.WriteString("Due Date: " + commitment.DueDate + "\n")
	b.WriteString("Status: " + commitment.Status + "\n\n")
	b.WriteString("## Promise\n\n")
	b.WriteString(commitment.PromiseText + "\n\n")
	b.WriteString("## Targets\n")
	for _, target := range commitment.Targets {
		b.WriteString("- " + target + "\n")
	}
	b.WriteString("\n## Evidence\n")
	if len(evidence) == 0 {
		b.WriteString("None yet.\n")
		return b.String()
	}
	for _, item := range evidence {
		line := fmt.Sprintf("- %s %s (%s)", item.EvidenceID, item.EvidenceType, item.ObservedAt)
		if item.Target != "" {
			line += " target=" + item.Target
		}
		if item.Notes != "" {
			line += " - " + item.Notes
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func AssessmentMarkdown(assessment model.Assessment, commitment model.Commitment) string {
	var b strings.Builder
	b.WriteString("# " + assessment.AssessmentID + "\n\n")
	b.WriteString("Commitment: " + assessment.CommitmentID + "\n")
	if assessment.ArtifactCID != "" {
		b.WriteString("Artifact CID: " + assessment.ArtifactCID + "\n")
	}
	if assessment.ProtocolPCID != "" {
		b.WriteString("Protocol pCID: " + assessment.ProtocolPCID + "\n")
	}
	b.WriteString("Assessor: " + assessment.Assessor + "\n")
	b.WriteString("Status: " + assessment.Status + "\n")
	b.WriteString("Assessed At: " + assessment.AssessedAt + "\n")
	b.WriteString("Promiser: " + commitment.Promiser + "\n\n")
	b.WriteString("## Basis\n")
	if len(assessment.Basis) == 0 {
		b.WriteString("None provided.\n")
	} else {
		for _, basis := range assessment.Basis {
			b.WriteString("- " + basis + "\n")
		}
	}
	b.WriteString("\n## Notes\n\n")
	if assessment.Notes == "" {
		b.WriteString("None.\n")
	} else {
		b.WriteString(assessment.Notes + "\n")
	}
	return b.String()
}
