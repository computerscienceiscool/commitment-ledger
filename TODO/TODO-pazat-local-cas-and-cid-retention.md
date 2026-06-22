# TODO-pazat: Add local CAS and CID-addressed artifact retention

## Decision Intent Log

ID: DI-pazat  
Date: 2026-06-22 11:28:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Store primary protocol artifacts by content address in repo-local CAS so exact bytes remain durable, replayable evidence rather than being collapsed into mutable projections.  
Intent: Align storage with the PromiseGrid guide's content-addressing and evidence discipline.  
Constraints: Keep storage local to this repo in v0.1. Preserve exact bytes. Projection files may index or summarize artifacts but must not replace them.  
Affects: `TODO/TODO.md`, `TODO/TODO-pazat-local-cas-and-cid-retention.md`, `data/cas/`, `internal/cid/`, `internal/cas/`, `internal/ledger/`

Goal: Make raw protocol artifacts content-addressed and durable.

- [x] pazat.1 Implement local CID derivation for exact bytes.
- [x] pazat.2 Add a repo-local CAS layout for raw artifact storage.
- [x] pazat.3 Record artifact metadata and references without losing exact bytes.
- [x] pazat.4 Retain protocol docs and emitted envelopes as CID-addressed evidence.
