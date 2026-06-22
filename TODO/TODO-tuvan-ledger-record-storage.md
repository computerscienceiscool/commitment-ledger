# TODO-tuvan: Store append-only ledger records in JSONL and Markdown

## Decision Intent Log

ID: DI-tuvan  
Date: 2026-06-22 11:02:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Track machine-readable and human-readable record storage together because the spec requires append-friendly JSONL plus auditable Markdown for commitments and assessments.  
Intent: Keep the ledger storage contract explicit while the core record types are implemented.  
Constraints: Use repo-local append-only files. Keep JSONL authoritative for machine use and Markdown readable without a database. Avoid destructive rewrites of source records.  
Affects: `TODO/TODO.md`, `TODO/TODO-tuvan-ledger-record-storage.md`, `internal/ledger/`, `data/`, `records/`

Goal: Implement durable local storage for work, commitment, evidence, and assessment records.

- [x] tuvan.1 Define JSON record types for work items, commitments, evidence, and assessments.
- [x] tuvan.2 Write append-only JSONL helpers for ledger data files.
- [x] tuvan.3 Render Markdown commitment and assessment records in `records/`.
- [x] tuvan.4 Preserve scan snapshots and derived work-item observations locally.
