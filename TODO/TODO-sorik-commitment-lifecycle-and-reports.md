# TODO-sorik: Implement commitment lifecycle commands and reports

## Decision Intent Log

ID: DI-sorik  
Date: 2026-06-22 11:02:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Track the command surface and status lifecycle together so v0.1 can create self-authored commitments, record evidence, expire overdue promises, assess outcomes, and summarize results.  
Intent: Preserve the user-visible contract before the CLI behavior is completed.  
Constraints: Only the promiser may create a commitment for themselves. Due dates are required. Expiration must not automatically mean broken. Reports stay local and CLI-readable.  
Affects: `TODO/TODO.md`, `TODO/TODO-sorik-commitment-lifecycle-and-reports.md`, `cmd/commitment-ledger/main.go`, `internal/commitment/`, `internal/evidence/`, `internal/assessment/`, `internal/report/`

Goal: Implement the promise lifecycle and the main CLI workflows.

- [x] sorik.1 Create commitments with required due dates and validated work targets.
- [x] sorik.2 Record scan-derived evidence for checked TODOs, checked subtasks, and observed commits.
- [x] sorik.3 Expire overdue open commitments into `expired_unassessed`.
- [x] sorik.4 Add manual assessment and human-readable repo, person, and work summaries.
