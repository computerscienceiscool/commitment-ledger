# TODO-karud: Freeze initial Commitment Ledger protocol docs and pCID workflow

## Decision Intent Log

ID: DI-karud  
Date: 2026-06-22 11:28:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Add explicit Commitment Ledger protocol documents and derive protocol identity from their exact bytes so the app can claim named PromiseGrid contracts instead of speaking unnamed local structs.  
Intent: Correct the architecture from PromiseGrid-inspired local bookkeeping to an actual grid-app contract surface.  
Constraints: Keep the first pass local and CLI-first. Define the app's own protocols in this repo. Use content-addressed identity for the protocol docs and treat projections as secondary views.  
Affects: `TODO/TODO.md`, `TODO/TODO-karud-freeze-protocol-docs-and-pcid-workflow.md`, `docs/protocols/`, `README.md`, `internal/protocol/`

Goal: Give Commitment Ledger explicit protocol docs and a local pCID workflow.

- [x] karud.1 Write initial protocol docs for commitments, evidence, assessments, and conformance claims.
- [x] karud.2 Define how local `pCID` and doc-CID are derived from exact frozen doc bytes.
- [x] karud.3 Add code that loads protocol docs and computes their identities.
- [x] karud.4 Surface protocol identities in emitted artifacts and local records.
