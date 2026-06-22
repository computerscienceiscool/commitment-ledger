# Commitment Ledger PromiseGrid Design

## Direction

Commitment Ledger is being upgraded from a local bookkeeping CLI to a real
PromiseGrid-style app.

The primary corrections are:

- protocol docs and explicit `pCID`s define the app contract
- commitments, evidence, assessments, and conformance claims are emitted as
  signed grid artifacts
- exact artifact bytes are retained in local CAS by CID
- JSONL and Markdown files remain local projections over artifact history

## Provisional Note

This repo's protocol docs are local frozen docs for this implementation, not a
claim that upstream PromiseGrid has already frozen the same application
contracts globally. See `docs/implementation-status.md` for the current split
between upstream-open areas and local missing pieces.

## Initial Artifact Families

- `commitment-promise-v1`
- `commitment-evidence-v1`
- `commitment-assessment-v1`
- `implementation-conformance-v1`

## Initial Local Choices

- exact protocol-doc bytes define local `pCID`s in v0.1
- payload bytes are UTF-8 JSON owned by each protocol doc
- the grid envelope is `grid([42(pCID), payload, proof])`
- proof bytes are Ed25519 signatures over exact CBOR of `[42(pCID), payload]`
- artifacts are stored in repo-local CAS under `data/cas/`

## Projection Discipline

- raw artifacts are primary
- JSONL rows index and summarize artifact history
- Markdown records stay human-readable and retain artifact CID plus protocol
  `pCID`
