# TODO-babad: Add make-based local workflows and demo helpers

## Decision Intent Log

ID: DI-babad  
Date: 2026-06-24 11:10:00  
Author: codex@openai.com (Codex)  
Status: done  
Decision: Add a repo-native `Makefile` so common development, verification, and demo tasks remain discoverable and runnable without Codex access.  
Intent: Preserve operational continuity by moving recurring command knowledge into the repository itself.  
Constraints: Keep targets local-first and deterministic. Avoid hidden network dependence. Make demo helpers explicit about the paths and state they expect.  
Affects: `TODO/TODO.md`, `TODO/TODO-babad-add-make-based-local-workflows-and-demo-helpers.md`, `Makefile`, `README.md`, `docs/demo-plan.md`, `docs/demo-script.md`, `.gitignore`

Goal: Make the repo self-service for common local workflows and demos.

- [x] babad.1 Add a discoverable `Makefile` for build, test, formatting, and CLI command wrappers.
- [x] babad.2 Add demo-oriented make targets and generated demo config support.
- [x] babad.3 Update docs so the documented workflow matches the actual make-based entry points.
