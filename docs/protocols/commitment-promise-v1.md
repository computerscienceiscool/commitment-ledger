# Commitment Ledger Protocol: commitment-promise-v1

Status: local frozen v0.1 candidate  
Audience: Commitment Ledger peers and local projections

## Purpose

This protocol defines a promise-bearing artifact for one promiser to make a
commitment against one or more branch-qualified work targets discovered from a
tracked repository.

## Protocol Identity

For v0.1 in this repo, the protocol's `pCID` is derived from the exact bytes of
this frozen document. The implementation computes that content-addressed
identity locally and uses it as the protocol selector in grid-envelope slot `0`.

## Envelope

The primary artifact is a signed grid envelope:

```text
grid([42(pCID), payload, proof])
```

- slot `0`: `42(pCID)` selecting this protocol document
- slot `1`: opaque payload bytes defined by this protocol
- slot `2`: proof bytes carrying a signature over the exact signable view

## Payload

The payload bytes are UTF-8 JSON with this shape:

```json
{
  "kind": "commitment_promise",
  "promiser": "JJ",
  "promisee": "team/project" ,
  "repo": "wire-lab",
  "branch": "main",
  "targets": ["wire-lab/main/TODO-ravud/2.1"],
  "promise_text": "I promise to complete TODO-ravud subtask 2.1.",
  "due_date": "2026-06-28",
  "created_at": "2026-06-22T10:00:00-07:00",
  "supersedes": [],
  "metadata": {
    "source_commit": "abc123"
  }
}
```

## Semantics

- The promiser is the agent making the promise.
- The app records only self-authored promises; the signer is expected to match
  the promiser's local durable identity.
- `targets` are branch-qualified work targets.
- `due_date` creates expiration pressure but does not by itself prove breakage.
- `supersedes` lists prior commitment artifact CIDs this promise replaces.

## Signable View

The signable bytes are the exact CBOR encoding of the two-slot array:

```text
[42(pCID), payload]
```

The proof bytes are an Ed25519 signature over that exact signable view in v0.1.

## Local Projection Rules

Implementations may project these artifacts into local indexes, Markdown
records, or reports, but those projections are derived views over the raw
artifact bytes and must retain the source artifact CID.
