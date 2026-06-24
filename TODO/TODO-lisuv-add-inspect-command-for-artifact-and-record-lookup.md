# TODO-lisuv: Add inspect command for artifact and record lookup

## Decision Intent Log

ID: DI-lisuv  
Date: 2026-06-24 12:20:00  
Author: codex@openai.com (Codex)  
Status: done  
Decision: Add a first-class `inspect` command so operators can resolve local IDs and artifact CIDs without grepping projection files by hand.  
Intent: Improve day-to-day operability by making artifact provenance, protocol version, and current projected state directly visible from the CLI.  
Constraints: Stay local-only. Reuse existing JSONL and Markdown projections. Make evidence inspection honest about the current lack of standalone Markdown evidence records.  
Affects: `TODO/TODO.md`, `TODO/TODO-lisuv-add-inspect-command-for-artifact-and-record-lookup.md`, `cmd/commitment-ledger/main.go`, `cmd/commitment-ledger/main_test.go`, `internal/protocol/protocol.go`, `README.md`, `Makefile`

Goal: Make it easy to inspect commitments, evidence, assessments, and artifact CIDs from the CLI.

- [x] lisuv.1 Add `inspect` command resolution and formatted output.
- [x] lisuv.2 Cover inspect behavior for IDs, artifact CIDs, and unknown references.
- [x] lisuv.3 Document and expose the new command in repo-native entry points.
