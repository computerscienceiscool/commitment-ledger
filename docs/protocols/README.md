# Commitment Ledger Protocols

This directory defines the local PromiseGrid-facing protocol documents for
Commitment Ledger.

The working discipline in this repo is:

- each protocol document is frozen by exact bytes
- the content-addressed identity of that frozen document is its local protocol
  identifier (`pCID`) for v0.1
- emitted app artifacts carry that `pCID` in grid-envelope slot `0`
- local JSONL and Markdown files are projections over CID-addressed protocol
  artifacts, not the primary protocol bytes

The initial protocol set is:

- `commitment-promise-v1.md`
- `commitment-evidence-v1.md`
- `commitment-assessment-v1.md`
- `implementation-conformance-v1.md`
