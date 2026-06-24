# TODO-fesob - Add Bundle-Path Loss Recovery

## Summary

Improve recovery when recorded bundle source paths no longer exist.

## Deliverables

- Detect and report missing bundle-source files more clearly.
- Add fallback recovery guidance or alternative recovery paths where possible.
- Tighten `doctor` and `repair` messaging around unrecoverable source-path loss.
- Add focused tests for missing saved bundle paths.
