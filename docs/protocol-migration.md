# Protocol Migration Notes

## Purpose

This file explains the current local protocol-version story for Commitment
Ledger and how to read artifacts across the `v1` to `v2` evidence and
assessment transition.

## Current Emission Set

New artifacts are currently emitted under these frozen protocol docs:

- `commitment-promise-v1`
- `commitment-evidence-v2`
- `commitment-assessment-v2`
- `implementation-conformance-v1`

## Historical Frozen Docs

These frozen protocol docs remain in the repo for historical `pCID`
continuity:

- `commitment-evidence-v1`
- `commitment-assessment-v1`

They are still loaded into the local protocol registry so operators can keep
the exact frozen bytes tied to older local artifacts, but current commands do
not emit them for new evidence or assessment records.

## Reading Rule

When interpreting an artifact:

1. treat the artifact's carried `protocol_pcid` as authoritative
2. resolve that `pCID` against the matching frozen protocol doc bytes
3. interpret the payload using that exact frozen doc, not the latest similarly
   named document

In practical terms, `commitment-evidence-v1` and `commitment-evidence-v2` are
siblings, not in-place revisions of one live mutable spec.

## Why Evidence And Assessment Moved To v2

`v2` was introduced because the implementation tightened local semantics:

- manual evidence is validated against the referenced commitment scope
- assessment basis references must resolve to evidence artifacts for the same
  commitment
- assessment flow no longer silently rewrites already-finalized commitments

Those changes affect protocol-facing meaning, so the PromiseGrid-safe move was
to freeze new `v2` docs rather than rewriting the bytes of already-frozen `v1`
docs in place.

## Conformance Interpretation

The current `implementation-conformance-v1` payload now distinguishes three
sets:

- `claimed_protocol_pcids`: frozen protocol docs the implementation can
  interpret locally
- `emitted_protocol_pcids`: frozen protocol docs current commands emit for new
  artifacts
- `historical_protocol_pcids`: frozen older docs retained for historical local
  artifacts but not emitted by current commands

This keeps local historical continuity explicit without pretending every loaded
protocol doc is still part of the current emission path.
