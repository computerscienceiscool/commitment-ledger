package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"commitment-ledger/internal/protocol"
)

const (
	managedStart = "<!-- commitment-ledger:conformance:start -->"
	managedEnd   = "<!-- commitment-ledger:conformance:end -->"
)

type Entry struct {
	Claim          string
	Spec           string
	Scope          string
	BreakingChange string
	Notes          string
}

func Path(root string) string {
	return filepath.Join(root, "CHANGELOG.md")
}

func CurrentEntries(registry protocol.Registry, version string) []Entry {
	return []Entry{
		{
			Claim:          "implements",
			Spec:           registry.MustPCID(protocol.CommitmentPromise),
			Scope:          "full",
			BreakingChange: "false",
			Notes:          fmt.Sprintf("Current commitment-ledger %s emission for local frozen `%s`.", version, protocol.CommitmentPromise),
		},
		{
			Claim:          "implements",
			Spec:           registry.MustPCID(protocol.CommitmentEvidence),
			Scope:          "full",
			BreakingChange: "false",
			Notes:          fmt.Sprintf("Current commitment-ledger %s emission for local frozen `%s`.", version, protocol.CommitmentEvidence),
		},
		{
			Claim:          "implements",
			Spec:           registry.MustPCID(protocol.CommitmentAssessment),
			Scope:          "full",
			BreakingChange: "false",
			Notes:          fmt.Sprintf("Current commitment-ledger %s emission for local frozen `%s`.", version, protocol.CommitmentAssessment),
		},
		{
			Claim:          "implements",
			Spec:           registry.MustPCID(protocol.ImplementationConformance),
			Scope:          "full",
			BreakingChange: "false",
			Notes:          fmt.Sprintf("Current commitment-ledger %s emission for local frozen `%s`.", version, protocol.ImplementationConformance),
		},
		{
			Claim:          "partially-implements",
			Spec:           registry.MustPCID(protocol.CommitmentEvidenceV1),
			Scope:          "historical-read-only",
			BreakingChange: "false",
			Notes:          fmt.Sprintf("Retained in commitment-ledger %s for reading older local `%s` artifacts; not emitted by current commands.", version, protocol.CommitmentEvidenceV1),
		},
		{
			Claim:          "partially-implements",
			Spec:           registry.MustPCID(protocol.CommitmentAssessmentV1),
			Scope:          "historical-read-only",
			BreakingChange: "false",
			Notes:          fmt.Sprintf("Retained in commitment-ledger %s for reading older local `%s` artifacts; not emitted by current commands.", version, protocol.CommitmentAssessmentV1),
		},
	}
}

func RenderManagedSection(entries []Entry) string {
	var b strings.Builder
	b.WriteString(managedStart + "\n")
	b.WriteString("### Conformance\n\n")
	b.WriteString("These entries are the repo-level publication surface for implementation\n")
	b.WriteString("conformance claims. Until upstream PromiseGrid freezes one shared Commitment\n")
	b.WriteString("Ledger app contract, these entries name the exact local frozen spec doc-CIDs\n")
	b.WriteString("this implementation claims to speak. The signed `conformance` artifact is the\n")
	b.WriteString("machine-readable companion claim.\n\n")
	for i, entry := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("```changelog-entry\n")
		b.WriteString("claim:           " + entry.Claim + "\n")
		b.WriteString("spec:            " + entry.Spec + "\n")
		b.WriteString("scope:           " + entry.Scope + "\n")
		b.WriteString("breaking-change: " + entry.BreakingChange + "\n")
		b.WriteString("notes:           " + entry.Notes + "\n")
		b.WriteString("```\n")
	}
	b.WriteString(managedEnd + "\n")
	return b.String()
}

func WriteManaged(root string, registry protocol.Registry, version string) error {
	path := Path(root)
	entries := CurrentEntries(registry, version)
	managed := RenderManagedSection(entries)
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read changelog %q: %w", path, err)
	}
	out := replaceManagedSection(string(body), managed)
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write changelog %q: %w", path, err)
	}
	return nil
}

func MatchSpec(root string, spec string) ([]Entry, error) {
	body, err := os.ReadFile(Path(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read changelog: %w", err)
	}
	entries := ParseEntries(string(body))
	matches := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Spec == spec {
			matches = append(matches, entry)
		}
	}
	return matches, nil
}

func ParseEntries(body string) []Entry {
	lines := strings.Split(body, "\n")
	entries := []Entry{}
	inBlock := false
	current := Entry{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "```changelog-entry":
			inBlock = true
			current = Entry{}
			continue
		case "```":
			if inBlock {
				entries = append(entries, current)
			}
			inBlock = false
			continue
		}
		if !inBlock {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "claim":
			current.Claim = value
		case "spec":
			current.Spec = value
		case "scope":
			current.Scope = value
		case "breaking-change":
			current.BreakingChange = value
		case "notes":
			current.Notes = value
		}
	}
	return entries
}

func replaceManagedSection(existing string, managed string) string {
	if existing == "" {
		return "# CHANGELOG\n\n## Unreleased\n\n" + managed
	}
	start := strings.Index(existing, managedStart)
	end := strings.Index(existing, managedEnd)
	if start >= 0 && end >= 0 && end >= start {
		end += len(managedEnd)
		prefix := strings.TrimRight(existing[:start], "\n")
		suffix := strings.TrimLeft(existing[end:], "\n")
		out := prefix + "\n\n" + managed
		if suffix != "" {
			out += "\n" + suffix
		}
		return out
	}
	if strings.Contains(existing, "## Unreleased") {
		return strings.TrimRight(existing, "\n") + "\n\n" + managed
	}
	return strings.TrimRight(existing, "\n") + "\n\n## Unreleased\n\n" + managed
}
