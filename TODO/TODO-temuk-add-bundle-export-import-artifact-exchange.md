# TODO-temuk: Add bundle export/import artifact exchange

## Decision Intent Log

ID: DI-temuk  
Date: 2026-06-24 13:40:00  
Author: codex@openai.com (Codex)  
Status: done  
Decision: Add bundle-based `export` and `import` commands so artifacts, related projection rows, and optional support material can move between local Commitment Ledger repos.  
Intent: Bridge the gap between purely local operation and future peer exchange by making artifacts portable now, while keeping the transport model explicit and manual.  
Constraints: Stay local-first and deterministic. Keep imported protocol and signer support separate from the primary frozen docs and signer store. Make verification fail clearly when support material is missing or mismatched.  
Affects: `TODO/TODO.md`, `TODO/TODO-temuk-add-bundle-export-import-artifact-exchange.md`, `cmd/commitment-ledger/main.go`, `cmd/commitment-ledger/main_test.go`, `internal/exchange/bundle.go`, `internal/grid/grid.go`, `internal/grid/grid_test.go`, `internal/identity/identity.go`, `internal/protocol/protocol.go`, `Makefile`, `README.md`, `docs/operator-guide.md`, `docs/implementation-status.md`, `docs/demo-plan.md`, `docs/trust-and-verification.md`

Goal: Support manual artifact exchange with bundled support material and verification-aware import behavior.

- [x] temuk.1 Add export/import bundle commands and support-material storage.
- [x] temuk.2 Add end-to-end tests for round-trip import plus missing/mismatched support failures.
- [x] temuk.3 Update operator and status docs so the exchange story matches the implemented commands.
