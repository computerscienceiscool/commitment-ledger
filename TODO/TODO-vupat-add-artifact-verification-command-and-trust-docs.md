# TODO-vupat: Add artifact verification command and trust docs

## Decision Intent Log

ID: DI-vupat  
Date: 2026-06-24 13:05:00  
Author: codex@openai.com (Codex)  
Status: done  
Decision: Add a first-class `verify` command that validates local artifact envelopes, signature proofs, and protocol resolution, then document what that verification does and does not prove.  
Intent: Move the repo from “can inspect local artifacts” to “can verify local artifacts” without requiring manual CAS decoding or ad hoc scripting.  
Constraints: Stay local-only. Verify against local CAS bytes and local identity material. Be explicit about the difference between cryptographic verification and broader trust judgment.  
Affects: `TODO/TODO.md`, `TODO/TODO-vupat-add-artifact-verification-command-and-trust-docs.md`, `cmd/commitment-ledger/main.go`, `cmd/commitment-ledger/main_test.go`, `internal/grid/grid.go`, `internal/grid/grid_test.go`, `internal/identity/identity.go`, `Makefile`, `README.md`, `docs/operator-guide.md`, `docs/implementation-status.md`, `docs/trust-and-verification.md`

Goal: Let operators verify emitted artifacts from the CLI and understand the resulting trust model.

- [x] vupat.1 Add envelope parsing and artifact verification support.
- [x] vupat.2 Add `verify` command and end-to-end verification tests.
- [x] vupat.3 Document trust and verification semantics in operator-facing docs.
