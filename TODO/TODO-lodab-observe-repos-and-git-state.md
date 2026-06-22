# TODO-lodab: Observe tracked repos and capture branch-specific git state

## Decision Intent Log

ID: DI-lodab  
Date: 2026-06-22 11:02:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Track the repo-observation slice separately so the app can scan local git clones without mutating them and can preserve repo, branch, and commit identity in evidence.  
Intent: Make the branch-specific source model explicit before command behavior is implemented.  
Constraints: Use local git clones only in v0.1. Treat provider metadata as informative only. Default behavior must not change branches.  
Affects: `TODO/TODO.md`, `TODO/TODO-lodab-observe-repos-and-git-state.md`, `internal/config/`, `internal/gitrepo/`, `config/repos.json`

Goal: Implement config-driven observation of tracked repositories.

- [x] lodab.1 Parse `config/repos.json` into repo source records.
- [x] lodab.2 Read current branch and commit hash from local git clones.
- [x] lodab.3 Respect configured branches without implicit checkout by default.
- [x] lodab.4 Surface repo scan summaries keyed by repo and branch.
