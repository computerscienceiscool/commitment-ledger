# TODO-nopud - Harden Identity Restore Conflict And Partial-Restore Behavior

## Summary

Make `identity restore` more defensive and more explicit about partial success.

## Deliverables

- Detect and classify current-vs-archived restore conflicts clearly.
- Preserve per-identity partial results instead of failing without context.
- Surface restore conflicts and skipped items in text and JSON output.
- Add focused restore-path tests.
