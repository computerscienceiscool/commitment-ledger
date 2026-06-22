# Commitment Ledger Protocol: implementation-conformance-v1

Status: local frozen v0.1 candidate  
Audience: Commitment Ledger operators and downstream readers

## Purpose

This protocol defines a conformance-claim artifact stating which local protocol
documents this implementation claims to speak and which local projection rules
it uses.

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
  "kind": "implementation_conformance",
  "implementation": "commitment-ledger",
  "version": "v0.1.0",
  "claimed_protocol_pcids": ["bafy...", "bafy...", "bafy..."],
  "projection_rules": [
    "JSONL files are append-only local indexes over artifact history.",
    "Markdown records are human-readable projections that retain artifact CIDs."
  ],
  "claimed_at": "2026-06-22T10:00:00-07:00"
}
```

## Semantics

- This claim is about what the implementation says it supports.
- Projection rules are informative local behavior notes and do not replace the
  raw artifacts or protocol docs.

## Signable View

The signable bytes are the exact CBOR encoding of:

```text
[42(pCID), payload]
```

The proof bytes are an Ed25519 signature over that signable view in v0.1.
