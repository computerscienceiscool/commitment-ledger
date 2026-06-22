# TODO-mipog: Parse numeric and proquint TODOs with detail-file subtasks

## Decision Intent Log

ID: DI-mipog  
Date: 2026-06-22 11:02:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Track TODO parsing as its own slice because the spec requires mixed legacy and proquint formats plus inferred subtask targets from detail-file checkboxes.  
Intent: Preserve the parsing and target-identity rules before they are encoded into command behavior.  
Constraints: Support both numeric IDs and `TODO-<handle>` IDs. Parse checked and unchecked state. Infer subtask identifiers from checkbox numbering in detail files.  
Affects: `TODO/TODO.md`, `TODO/TODO-mipog-parse-todos-and-subtasks.md`, `internal/todo/parser.go`, `internal/todo/parser_test.go`

Goal: Build the work-item parser for top-level TODOs and subtasks.

- [x] mipog.1 Parse legacy numeric TODO entries from `TODO/TODO.md`.
- [x] mipog.2 Parse proquint TODO entries with linked detail files.
- [x] mipog.3 Discover checkbox subtasks inside detail files and infer subtask IDs.
- [x] mipog.4 Emit branch-qualified work targets for parent TODOs and subtasks.
