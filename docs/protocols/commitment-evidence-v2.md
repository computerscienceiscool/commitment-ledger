# Commitment Ledger Protocol: commitment-evidence-v2

Status: local frozen v0.2 candidate  
Audience: Commitment Ledger peers and local projections

## Purpose

This protocol defines evidence-bearing artifacts about commitment-relevant
observations such as checked TODO state, observed commits, human review notes,
and local refusal or timeout observations.

## Protocol Identity

For v0.2 in this repo, the protocol's `pCID` is derived from the exact bytes of
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
  "kind": "commitment_evidence",
  "commitment_ref": "bafy...",
  "observer": "JJ",
  "repo": "wire-lab",
  "branch": "main",
  "source_commit": "def456",
  "target": "wire-lab/main/TODO-ravud/2.1",
  "evidence_kind": "todo_checked",
  "observed_at": "2026-06-24T09:00:00-07:00",
  "observed_bytes_cid": "bafy...",
  "notes": "Subtask checked off in detail file."
}
```

## Semantics

- `commitment_ref` points to a commitment artifact CID.
- `observer` is the local agent recording the observation.
- `repo`, `branch`, and `target` are expected to stay within the referenced commitment scope in local v0.2 validation.
- `observed_bytes_cid` may point to exact raw bytes preserved as supporting
  evidence when available.
- A timeout or silence observation is local evidence, not automatic proof of
  broken intent.

## Signable View

The signable bytes are the exact CBOR encoding of:

```text
[42(pCID), payload]
```

The proof bytes are an Ed25519 signature over that signable view in v0.2.
