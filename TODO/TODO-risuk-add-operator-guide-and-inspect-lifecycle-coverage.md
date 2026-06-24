# TODO-risuk: Add operator guide and inspect lifecycle coverage

## Decision Intent Log

ID: DI-risuk  
Date: 2026-06-24 12:45:00  
Author: codex@openai.com (Codex)  
Status: done  
Decision: Add an operator-facing guide and extend end-to-end tests so the documented troubleshooting path is backed by a real assessed lifecycle.  
Intent: Make the repo easier to run and debug without requiring source-level knowledge or manual JSONL grepping.  
Constraints: Keep the guide aligned with current local-only behavior. Prefer examples that already match the fixture lifecycle and existing `Makefile` helpers.  
Affects: `TODO/TODO.md`, `TODO/TODO-risuk-add-operator-guide-and-inspect-lifecycle-coverage.md`, `docs/operator-guide.md`, `README.md`, `docs/implementation-status.md`, `cmd/commitment-ledger/main_test.go`

Goal: Document normal operation and troubleshooting, then verify `inspect` across a full assessed lifecycle.

- [x] risuk.1 Add an operator guide for scan, inspect, report, and local file layout.
- [x] risuk.2 Add troubleshooting notes for common command failures and state surprises.
- [x] risuk.3 Extend end-to-end tests to inspect assessed commitments and assessments.
