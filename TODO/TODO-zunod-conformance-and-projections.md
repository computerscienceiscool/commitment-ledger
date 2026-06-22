# TODO-zunod: Publish implementation conformance claims and projection rules

## Decision Intent Log

ID: DI-zunod  
Date: 2026-06-22 11:28:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Make the repo publish explicit conformance claims against its local protocol docs and treat JSONL/Markdown status views as local projections over artifact history rather than the protocol itself.  
Intent: Align the implementation story with the PromiseGrid guide's conformance-claim discipline.  
Constraints: Keep claims local and explicit. Do not pretend draft docs are global standards. Projection logic must remain additive and auditable.  
Affects: `TODO/TODO.md`, `TODO/TODO-zunod-conformance-and-projections.md`, `CHANGELOG.md`, `internal/report/`, `internal/ledger/`, `records/`

Goal: Make the app's implementation claims and local views explicit.

- [x] zunod.1 Add a conformance-claim protocol doc and emitted claim artifact.
- [x] zunod.2 Record which protocol docs this implementation claims to speak.
- [x] zunod.3 Treat JSONL and Markdown records as projections over artifact history.
- [x] zunod.4 Surface artifact CIDs and protocol pCIDs in reports and records.
