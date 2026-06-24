# TODO-togiv - Restore validation and conflict handling polish

Status: done

## Goal

Tighten the local recovery workflow around identity backups and restore paths so
operators can trust restore behavior and recovery tooling.

## Completed

- Added `identity restore --in ...` for current and archived local signer
  material.
- Added conflict-safe restore behavior so existing different local key material
  is not overwritten silently.
- Added `doctor --strict` for warning-fatal operation in CI or audits.
- Added multi-rotation lineage coverage so archived signer verification is
  tested beyond a single rotation.
- Added `docs/recovery-checklist.md` and aligned recovery-oriented operator
  documentation.

## Notes

- This TODO was added after implementation as part of repairing project TODO
  discipline.
- Future work should be entered in `TODO/TODO.md` before or during execution,
  not reconstructed afterward.
