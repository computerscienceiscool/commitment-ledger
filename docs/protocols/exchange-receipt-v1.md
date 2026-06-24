# Commitment Ledger Protocol: exchange-receipt-v1

Status: local frozen v0.1 candidate  
Audience: Commitment Ledger operators and downstream readers

## Purpose

This protocol defines a signed receipt artifact saying that this local
Commitment Ledger instance received an artifact bundle and recorded local import
provenance for it.

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
  "kind": "exchange_receipt",
  "receipt_id": "RECEIPT-20260624-001",
  "received_artifact_cid": "bafy...",
  "related_id": "COMMITMENT-20260624-alice-001",
  "source_path": "/tmp/peer-inbox/20260625T030200Z-COMMITMENT-....json",
  "receiver": "commitment-ledger",
  "received_at": "2026-06-24T10:00:00-07:00",
  "support_installed": true,
  "installed_protocol_pcid": "bafy...",
  "installed_signer_identity": "Mallory"
}
```

## Semantics

- `receipt_id` is a local stable identifier for the receipt artifact.
- `received_artifact_cid` names the artifact envelope that was received.
- `related_id` may carry the local record ID associated with that artifact.
- `source_path` is the local filesystem path used for the received bundle.
- `receiver` identifies the local operator or service that recorded the receipt.
- `support_installed` says whether bundled signer/protocol support was installed
  during receipt handling.
- `installed_protocol_pcid` and `installed_signer_identity` are optional local
  notes about support material installed during the receive step.

## Signable View

The signable bytes are the exact CBOR encoding of:

```text
[42(pCID), payload]
```

The proof bytes are an Ed25519 signature over that signable view in v0.1.
