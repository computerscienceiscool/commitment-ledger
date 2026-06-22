# TODO-rovik: Emit signed grid-envelope artifacts for commitments and evidence

## Decision Intent Log

ID: DI-rovik  
Date: 2026-06-22 11:28:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Emit commitments, evidence, and assessments as signed `grid([42(pCID), payload, proof])` artifacts so the app's primary records are PromiseGrid-style protocol messages rather than bare local database rows.  
Intent: Move the user-visible lifecycle onto actual PromiseGrid-shaped artifacts.  
Constraints: Keep the universal envelope small. Let payload meaning stay pCID-owned. Keep signature handling local and explicit.  
Affects: `TODO/TODO.md`, `TODO/TODO-rovik-signed-grid-artifacts.md`, `internal/grid/`, `internal/identity/`, `internal/commitment/`, `internal/evidence/`, `internal/assessment/`

Goal: Make the lifecycle commands emit signed grid artifacts.

- [x] rovik.1 Define payload shapes for commitment, evidence, and assessment artifacts.
- [x] rovik.2 Implement signed grid-envelope encoding and verification.
- [x] rovik.3 Emit raw artifacts from `commit`, `evidence`, `assess`, and scan-derived evidence flows.
- [x] rovik.4 Preserve signer identity, proof bytes, and artifact references in projections.
