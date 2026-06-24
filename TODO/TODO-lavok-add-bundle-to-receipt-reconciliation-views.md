# TODO-lavok - Add Bundle-To-Receipt Reconciliation Views

## Summary

Add an operator-facing reconciliation view that joins bundle import provenance,
receive receipts, signer lineage, and trust evaluation into one artifact-level
chain.

## Why

The repo can already show:

- raw import provenance rows
- aggregate exchange/import summaries
- artifact-level inspect and verify views

What it still lacks is one direct answer for:

- which source bundle path introduced an artifact
- whether that artifact was imported more than once or from multiple sources
- which local exchange receipts acknowledge it
- what signer state and trust result apply across that chain

## Deliverables

- Add a `reconcile` CLI command with text and JSON output.
- Support filtering by artifact/reference, source path, signer, receipt signer,
  mode, and protocol pCID.
- Show import counts, sources, modes, receipt coverage, signer lineage, and
  trust results per imported artifact.
- Add end-to-end tests covering repeated imports and receive-receipt chains.
- Update the Makefile, README, and operator guide.
