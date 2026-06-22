# TODO-jafud: Bootstrap repo layout and config-driven CLI skeleton

## Decision Intent Log

ID: DI-jafud  
Date: 2026-06-22 11:02:00  
Author: codex@openai.com (Codex)  
Status: active  
Decision: Create the initial Go module, package layout, and repo-local storage directories described by the Commitment Ledger spec so later work can land in stable paths.  
Intent: Preserve the first implementation slice as explicit tracked work before code is added.  
Constraints: Stay CLI-first. Keep all ledger data in this repo. Do not add network dependencies, a web UI, or automatic source-repo edits.  
Affects: `TODO/TODO.md`, `TODO/TODO-jafud-bootstrap-layout-and-config.md`, `go.mod`, `cmd/`, `internal/`, `config/`, `data/`, `records/`, `README.md`

Goal: Bootstrap the repository structure needed for the v0.1 implementation.

- [x] jafud.1 Create the Go module and root command entrypoint.
- [x] jafud.2 Add the package directories named in the spec.
- [x] jafud.3 Create repo-local config, data, and record directories.
- [x] jafud.4 Add baseline documentation for the CLI-first prototype layout.
