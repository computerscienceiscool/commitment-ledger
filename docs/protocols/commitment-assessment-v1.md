# Commitment Ledger Protocol: commitment-assessment-v1

Status: local frozen v0.1 candidate  
Audience: Commitment Ledger peers and local projections

## Purpose

This protocol defines a local assessment artifact stating how an assessor judges
the outcome of a previously emitted commitment.

## Protocol Identity

For v0.1 in this repo, the protocol's `pCID` is derived from the exact bytes of
this frozen document.

## Envelope

The primary artifact is:

```text
grid([42(pCID), payload, proof])
```

## Payload

The payload bytes are UTF-8 JSON with this shape:

```json
{
  "kind": "commitment_assessment",
  "commitment_ref": "bafy...",
  "assessor": "Steve",
  "status": "kept",
  "assessed_at": "2026-06-28T15:00:00-07:00",
  "basis": ["bafy...", "bafy..."],
  "notes": "Both promised subtasks were checked off before the due date."
}
```

## Semantics

- `commitment_ref` points to a commitment artifact CID.
- `basis` lists supporting evidence artifact CIDs.
- Assessment is local and explicit; there is no global automatic verdict.

## Signable View

The signable bytes are the exact CBOR encoding of:

```text
[42(pCID), payload]
```

The proof bytes are an Ed25519 signature over that signable view in v0.1.
