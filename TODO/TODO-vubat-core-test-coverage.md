# TODO-vubat: Add deterministic tests for parsing, storage, and status transitions

## Decision Intent Log

ID: DI-vubat  
Date: 2026-06-22 11:02:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Track the initial verification pass explicitly so the v0.1 prototype does not rely on ad hoc manual checking for parser correctness or lifecycle behavior.  
Intent: Preserve test coverage as first-class work instead of a cleanup afterthought.  
Constraints: Use the standard `testing` package. Keep tests deterministic and local. Avoid network calls and avoid depending on mutable external repos beyond fixture content.  
Affects: `TODO/TODO.md`, `TODO/TODO-vubat-core-test-coverage.md`, `internal/*/*_test.go`

Goal: Cover the first implementation slices with deterministic tests.

- [x] vubat.1 Add parser tests for numeric, proquint, and subtask cases.
- [x] vubat.2 Add ledger storage tests for JSONL append and Markdown rendering.
- [x] vubat.3 Add lifecycle tests for commitment validation, expiration, and assessment.
- [ ] vubat.4 Verify the main repo scan and reporting flows against fixture repos.
