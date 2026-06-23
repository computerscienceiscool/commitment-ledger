# TODO-babab: Harden assessment, evidence, and reporting invariants

## Decision Intent Log

ID: DI-babab  
Date: 2026-06-23 13:05:00  
Author: codex@openai.com (Codex)  
Status: completed  
Decision: Close the review findings by enforcing commitment lifecycle invariants, validating evidence and assessment references, and making reports reflect all materially relevant outcomes.  
Intent: Preserve the repo's ledger semantics so signed local artifacts do not silently encode contradictory or malformed operator assertions.  
Constraints: Keep the CLI local-first, deterministic, and append-only. Do not introduce network requirements. Add tests alongside behavior changes.  
Affects: `TODO/TODO.md`, `TODO/TODO-babab-harden-assessment-evidence-and-reporting-invariants.md`, `cmd/commitment-ledger/main.go`, `internal/assessment/assessment.go`, `internal/report/report.go`, `internal/*/*_test.go`

Goal: Enforce tighter invariants around assessment, evidence, and reports.

- [x] babab.1 Reject assessments that overwrite already-finalized commitments.
- [x] babab.2 Require assessment basis references to resolve to evidence artifacts for the same commitment.
- [x] babab.3 Validate manual evidence repo, branch, and target metadata against the referenced commitment.
- [x] babab.4 Surface non-kept terminal outcomes in repo summaries and keep work summaries internally consistent.
