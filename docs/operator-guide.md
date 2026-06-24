# Operator Guide

## Purpose

This file is the practical runbook for operating Commitment Ledger locally.

Use it when you need to:

- run the normal scan -> commit -> evidence -> assess flow
- inspect what the ledger believes about a commitment or artifact
- understand where local state lives
- diagnose common operator errors

## Fast Path

For routine local work from the repo root:

```bash
make help
make test
make scan CONFIG=config/repos.json
make status
make report REPORT_ARGS='--promiser Alice'
make inspect INSPECT_ARGS='COMMITMENT-...'
make verify VERIFY_ARGS='COMMITMENT-...'
make conformance VERSION=v0.1.0 SIGNER=commitment-ledger
make export EXPORT_ARGS='--out /tmp/bundle.json COMMITMENT-...'
make import IMPORT_ARGS='--in /tmp/bundle.json'
```

For the seeded demo workflow:

```bash
make demo-setup
make demo-scan
make demo-status
make demo-report REPORT_ARGS='--promiser Alice'
```

## Core Commands

### `scan`

```bash
go run ./cmd/commitment-ledger scan --config config/repos.json
```

What it does:

- observes each enabled local git repo
- verifies the checked-out branch matches config
- parses TODO work and subtasks
- retires removed targets from the latest projected work state
- derives local scan evidence for open or expired-unassessed commitments

Use `scan` again after the source repo changes. The ledger does not infer new
repo state until you rescan.

### `commit`

```bash
go run ./cmd/commitment-ledger commit \
  --promiser Alice \
  --repo repo \
  --branch main \
  --target repo/main/TODO-ravud/1 \
  --due 2026-07-01 \
  --promise "I promise to complete subtask 1."
```

What it requires:

- a previously scanned target
- repo and branch matching that target
- a valid due date
- non-empty promise text

### `evidence`

```bash
go run ./cmd/commitment-ledger evidence \
  --commitment COMMITMENT-... \
  --type manual_note \
  --notes "Observed blocker"
```

Manual evidence must stay within the commitment's repo, branch, and promised
target scope.

### `assess`

```bash
go run ./cmd/commitment-ledger assess \
  --commitment COMMITMENT-... \
  --assessor Alice \
  --status kept \
  --basis EVIDENCE-... \
  --notes "Completed before the due date."
```

Important current rules:

- already-finalized commitments cannot be silently reassessed
- basis references must resolve to evidence for the same commitment
- `kept` is checked against the latest scanned work state
- for parent TODO targets, all discovered subtasks must be complete

### `status`

```bash
go run ./cmd/commitment-ledger status
```

Use this for repo-level operational summary:

- open TODOs
- open subtasks
- active commitments
- terminal commitment outcomes by repo/branch

### `report`

```bash
go run ./cmd/commitment-ledger report --promiser Alice
go run ./cmd/commitment-ledger report --repo alice-demo --branch main
go run ./cmd/commitment-ledger report --work alice-demo/main/TODO-ravud
```

Use `report` when you want filtered summaries by promiser, repo, or work
target.

### `inspect`

```bash
go run ./cmd/commitment-ledger inspect COMMITMENT-...
go run ./cmd/commitment-ledger inspect EVIDENCE-...
go run ./cmd/commitment-ledger inspect ASSESSMENT-...
go run ./cmd/commitment-ledger inspect bafy...
```

`inspect` resolves:

- commitment IDs
- evidence IDs
- assessment IDs
- artifact CIDs

It prints:

- artifact CID
- protocol name and `pCID`
- local frozen protocol doc path
- signer and signer key ID
- payload and proof CIDs
- related local record path when one exists
- current projected status or evidence details

### `verify`

```bash
go run ./cmd/commitment-ledger verify COMMITMENT-...
go run ./cmd/commitment-ledger verify EVIDENCE-...
go run ./cmd/commitment-ledger verify ASSESSMENT-...
go run ./cmd/commitment-ledger verify bafy...
```

`verify` resolves the same reference types as `inspect`, then checks:

- the artifact bytes can be loaded from local CAS
- the envelope decodes to the indexed protocol, payload, and proof
- the derived envelope, payload, and proof CIDs match the artifact index row
- the signature verifies over the carried protocol selector and payload
- the signer and key ID match local identity material under `config/identities/`

It also tells you whether the artifact's `protocol_pcid` matches a local frozen
protocol doc.

### `conformance`

```bash
go run ./cmd/commitment-ledger conformance --signer commitment-ledger --version v0.1.0
```

Use `conformance` when you want a machine-readable signed claim about the
protocol docs this implementation currently speaks.

After emitting a new conformance artifact, update `CHANGELOG.md` so the repo
also publishes the same claim in the human-facing shape the PromiseGrid dev
guide points App Devs toward.

### `export`

```bash
go run ./cmd/commitment-ledger export --out /tmp/bundle.json COMMITMENT-...
go run ./cmd/commitment-ledger export --out /tmp/bundle.json EVIDENCE-...
go run ./cmd/commitment-ledger export --out /tmp/bundle.json ASSESSMENT-...
go run ./cmd/commitment-ledger export --out /tmp/bundle.json bafy...
```

`export` writes a bundle containing:

- the artifact index row
- the raw envelope bytes
- the related commitment, evidence, or assessment projection when available
- the related commitment projection for evidence and assessment bundles
- available protocol and signer support material

### `import`

```bash
go run ./cmd/commitment-ledger import --in /tmp/bundle.json
go run ./cmd/commitment-ledger import --in /tmp/bundle.json --install-support=false
```

`import` restores the bundle into local CAS and append-only projections.

By default it also installs bundled support material into:

- `data/imported-protocols/`
- `config/imported-identities/`

Use `--install-support=false` when you want to import the artifact but
deliberately keep signer/protocol support separate.

## Local State Layout

### `data/`

Append-only machine-readable projections:

- `data/work_items.jsonl`: latest-known and historical work observations
- `data/commitments.jsonl`: commitment projections
- `data/evidence.jsonl`: evidence projections
- `data/assessments.jsonl`: assessment projections
- `data/artifacts.jsonl`: local artifact index rows
- `data/snapshots.jsonl`: per-scan repo summaries

### `data/cas/`

Raw content-addressed bytes for emitted artifacts and frozen protocol docs.

### `data/imported-protocols/`

Imported protocol docs and metadata used when a bundle carries a protocol doc
that is not already part of the repo's built-in frozen set.

### `records/`

Human-readable Markdown projections:

- `records/commitments/`
- `records/assessments/`

Evidence does not currently get its own standalone Markdown record.

### `config/imported-identities/`

Imported public signer material used by `verify` when an artifact signer is not
present in the primary local identity store.

### `docs/protocols/`

Frozen local protocol docs. The exact document bytes determine the local
`pCID`. Treat the artifact's `protocol_pcid` as authoritative when reading an
artifact.

### `CHANGELOG.md`

Repo-level conformance publication entries naming the exact frozen spec
doc-CIDs this implementation claims to speak. Read this together with emitted
`implementation_conformance` artifacts rather than as a replacement for them.

## Troubleshooting

### `scan` says the repo is on the wrong branch

Cause:

- the repo's current checked-out branch does not match `config/repos.json`

What to do:

- switch the repo to the configured branch
- or update the repo config if the intended observed branch changed

### `commit` says `unknown target`

Cause:

- the target has not been scanned yet
- or the target disappeared from the latest work state

What to do:

- run `scan` first
- use `report --work ...` or inspect `data/work_items.jsonl` to confirm the
  current target exists

### `assess` says `unknown basis reference`

Cause:

- the supplied `--basis` value is neither a known evidence ID nor an evidence
  artifact CID

What to do:

- inspect the intended evidence with `inspect EVIDENCE-...`
- use the evidence ID or the emitted artifact CID exactly

### `assess` says basis evidence belongs to another commitment

Cause:

- the referenced evidence exists, but it was recorded for a different
  commitment

What to do:

- inspect both the evidence and the commitment
- choose basis evidence from the same commitment only

### `assess --status kept` is rejected

Common causes:

- the promised subtask is not complete yet
- the promised parent TODO still has incomplete discovered subtasks
- the target no longer exists in the latest scanned state

What to do:

- update the source repo
- commit those TODO changes in the source repo
- run `scan` again
- inspect the commitment and relevant evidence before retrying `assess`

### A TODO disappeared after scan

This is now expected behavior when the source repo removes or renames the
target. The latest projected work state retires missing targets on the next
scan instead of keeping them alive forever.

### `inspect` shows no record path for evidence

This is expected today. Evidence is projected into `data/evidence.jsonl` and
the associated commitment Markdown record, but not into a standalone
`records/evidence/` tree yet.

### `verify` fails to load signer identity

Cause:

- the artifact signer exists in the proof, but there is no matching local
  identity file under `config/identities/`

What to do:

- confirm you are in the repo that originally emitted the artifact
- check `config/identities/`
- check `config/imported-identities/` if the artifact came from `import`
- use `inspect` to confirm the signer name carried in the proof

### `verify` says signer identity mismatch

Cause:

- the artifact proof was signed by a different key than the local identity file
- or the local identity file has changed since the artifact was emitted

What to do:

- inspect the artifact and signer identity carefully
- treat this as a real trust/integrity problem, not a display issue
- see `docs/trust-and-verification.md` for the trust model limits

### `verify` says local protocol match is `no`

Cause:

- the artifact references a protocol `pCID` the repo does not know locally
- or an imported bundle was loaded without its protocol support

What to do:

- inspect the artifact
- check `data/imported-protocols/`
- re-import the bundle with support material if appropriate

### `import` fails with a protocol support mismatch

Cause:

- the bundled protocol bytes do not hash to the bundled `protocol_pcid`

What to do:

- treat the bundle as malformed or tampered with
- do not trust that support material without an explanation
