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

## Artifact Index Anatomy

The local artifact index in `data/artifacts.jsonl` is a projection over raw CAS
objects. One row currently looks like this:

```json
{
  "artifact_cid": "bafkreidfay3cjjnzdvi57vayxuywwnzv7fivggfycqvhjwcwyen3udqxhe",
  "protocol_pcid": "bafkreibbmx4swcujke52q5ak7bx2p5so6caupcxh7trb25owz5bxmj5dnq",
  "kind": "commitment_promise",
  "signer": "Alice",
  "signer_key_id": "alice-ed25519-v1",
  "payload_cid": "bafkreidzsnd5xhe4lnbohcr2ghcd7i3qo7kw5ce2lleeob4wg74p2tk5ei",
  "proof_cid": "bafkreiefoonk5wfdh36swm6ikhxkjphpo5nelem4n5vnn4tpielsmfgmni",
  "observed_at": "2026-06-22T12:23:40-07:00",
  "related_id": "COMMITMENT-20260622-alice-001"
}
```

Field meaning:

- `artifact_cid`: CID of the full emitted grid-envelope bytes stored in local CAS
- `protocol_pcid`: content-addressed identity of the protocol doc that owns the payload meaning
- `kind`: local artifact family such as `commitment_promise`, `commitment_evidence`, `commitment_assessment`, or `implementation_conformance`
- `signer`: human-facing signer name used by the local identity store
- `signer_key_id`: local durable key label used for the signing key material
- `payload_cid`: CID of the raw payload bytes before proof wrapping
- `proof_cid`: CID of the raw proof bytes containing signature material
- `observed_at`: local timestamp when this implementation emitted or indexed the artifact
- `related_id`: local projection ID tied to this artifact, such as a `COMMITMENT-*`, `EVIDENCE-*`, or `ASSESSMENT-*`
- `related_cid`: optional CID reference to another artifact this row points at, such as the commitment artifact referenced by evidence or assessment

Important distinction:

- `artifact_cid` is the primary content-addressed identity for the emitted message bytes
- `related_id` is only a local projection handle for reports and operator workflows
- `protocol_pcid` identifies which frozen local protocol doc explains how to interpret the payload
