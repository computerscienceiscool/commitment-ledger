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
make status STATUS_ARGS='--exchange --json'
make report REPORT_ARGS='--promiser Alice'
make report REPORT_ARGS='--imports --json'
make inspect INSPECT_ARGS='--json COMMITMENT-...'
make verify VERIFY_ARGS='--json COMMITMENT-...'
make conformance VERSION=v0.1.0 SIGNER=commitment-ledger
make conformance-update VERSION=v0.1.0 SIGNER=commitment-ledger
make export EXPORT_ARGS='--out /tmp/bundle.json COMMITMENT-...'
make import IMPORT_ARGS='--in /tmp/bundle.json'
make provenance PROVENANCE_ARGS='--mode receive --json'
make reconcile RECONCILE_ARGS='--commitment COMMITMENT-... --json'
make send SEND_ARGS='--outbox /tmp/peer-outbox COMMITMENT-...'
make receive RECEIVE_ARGS='--inbox /tmp/peer-inbox --archive /tmp/peer-archive'
make doctor DOCTOR_ARGS='--repairable'
make repair REPAIR_ARGS='--records --protocol-cas --import-artifacts --import-support'
make identity IDENTITY_ARGS='list --json'
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
go run ./cmd/commitment-ledger status --exchange
go run ./cmd/commitment-ledger status --json
go run ./cmd/commitment-ledger status --exchange --json
```

Use this for repo-level operational summary:

- open TODOs
- open subtasks
- active commitments
- terminal commitment outcomes by repo/branch

Use `status --exchange` for import and exchange summary:

- total imports
- unique imported artifacts and source paths
- support installation count
- trusted vs untrusted imports under the current trust policy
- imported artifact signer state counts: active, archived, imported, or unknown
- per-mode counts such as `import` vs `receive`
- receipt-artifact counts, receipt signers, and how many imported artifacts have been acknowledged

Use `--json` when you need the repo-level or exchange-level status summary in a
stable machine-readable form.

### `report`

```bash
go run ./cmd/commitment-ledger report --promiser Alice
go run ./cmd/commitment-ledger report --repo alice-demo --branch main
go run ./cmd/commitment-ledger report --work alice-demo/main/TODO-ravud
go run ./cmd/commitment-ledger report --imports
go run ./cmd/commitment-ledger report --imports --json
```

Use `report` when you want filtered summaries by promiser, repo, or work
target.

Use `report --imports` when you want imported-artifact summaries grouped by
source path and annotated with the current trust-policy result.

That summary now also includes per-source receipt counts, receipt signers, and
locally resolved signer states for imported artifacts.

Use `--json` when you need machine-readable summaries for automation.

### `inspect`

```bash
go run ./cmd/commitment-ledger inspect COMMITMENT-...
go run ./cmd/commitment-ledger inspect EVIDENCE-...
go run ./cmd/commitment-ledger inspect ASSESSMENT-...
go run ./cmd/commitment-ledger inspect RECEIPT-...
go run ./cmd/commitment-ledger inspect bafy...
go run ./cmd/commitment-ledger inspect --json COMMITMENT-...
```

`inspect` resolves:

- commitment IDs
- evidence IDs
- assessment IDs
- receipt IDs
- artifact CIDs

It prints:

- artifact CID
- protocol name and `pCID`
- local frozen protocol doc path
- matching `CHANGELOG.md` conformance entries when the protocol is claimed there
- signer and signer key ID
- signer key state and matching local identity path when available
- payload and proof CIDs
- related local record path when one exists
- latest import provenance when the artifact entered this repo through `import` or `receive`
- current projected status or evidence details

Use `--json` when the same information needs to feed automation instead of a
human operator.

### `verify`

```bash
go run ./cmd/commitment-ledger verify COMMITMENT-...
go run ./cmd/commitment-ledger verify EVIDENCE-...
go run ./cmd/commitment-ledger verify ASSESSMENT-...
go run ./cmd/commitment-ledger verify RECEIPT-...
go run ./cmd/commitment-ledger verify bafy...
go run ./cmd/commitment-ledger verify --json COMMITMENT-...
```

`verify` resolves the same reference types as `inspect`, then checks:

- the artifact bytes can be loaded from local CAS
- the envelope decodes to the indexed protocol, payload, and proof
- the derived envelope, payload, and proof CIDs match the artifact index row
- the signature verifies over the carried protocol selector and payload
- the signer and key ID match active, archived, or imported local identity material

It also tells you whether the artifact's `protocol_pcid` matches a local frozen
protocol doc, whether the identity/protocol support came from built-in or
imported state, the latest recorded import provenance when applicable, and the
current local trust-policy judgment for signer, protocol, and import source. If
the artifact was signed before a local key rotation, `verify` will report the
archived signer key state instead of failing only because the active key has
changed.

Use `--json` when you need those verification results in a stable
machine-readable form.

### `config/trust-policy.json`

Optional local trust-policy file used by `verify`, `status --exchange`,
`report --imports`, and `reconcile`.

Current fields:

- `trust_built_in_signers`
- `trust_built_in_protocols`
- `trusted_signers`
- `trusted_protocol_pcids`
- `trusted_import_modes`
- `trusted_import_path_prefixes`

### `conformance`

```bash
go run ./cmd/commitment-ledger conformance --signer commitment-ledger --version v0.1.0
```

Use `conformance` when you want a machine-readable signed claim about the
protocol docs this implementation currently speaks.

Use `--write-changelog` or `make conformance-update` when you want the repo's
managed `CHANGELOG.md` conformance entries refreshed alongside the signed
artifact.

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

Every successful `import` also appends an import provenance row to
`data/imports.jsonl`.

### `provenance`

```bash
go run ./cmd/commitment-ledger provenance --mode receive
go run ./cmd/commitment-ledger provenance --artifact bafy...
go run ./cmd/commitment-ledger provenance --source /tmp/peer-inbox/bundle.json --json
go run ./cmd/commitment-ledger provenance --signer Mallory
go run ./cmd/commitment-ledger provenance --receipt-signer commitment-ledger --protocol-pcid bafy... --json
```

`provenance` is the direct history browser over local import and receive rows.

You can filter by:

- imported artifact CID
- source path
- artifact signer
- receipt signer
- protocol pCID
- mode such as `import` or `receive`

The output also shows any local receive-receipt artifacts that acknowledge the
imported artifact.

### `reconcile`

```bash
go run ./cmd/commitment-ledger reconcile --commitment COMMITMENT-...
go run ./cmd/commitment-ledger reconcile --artifact COMMITMENT-...
go run ./cmd/commitment-ledger reconcile --artifact bafy... --receipt-signer commitment-ledger
go run ./cmd/commitment-ledger reconcile --source /tmp/peer-inbox/bundle.json --json
go run ./cmd/commitment-ledger reconcile --mode receive --protocol-pcid bafy... --json
```

`reconcile` is the exchange-chain view built on top of raw provenance rows.

Use it when you need one direct answer for:

- which imported promise, evidence, and assessment artifacts belong to one
  commitment
- which bundle source paths introduced an artifact
- whether the same artifact was imported again through another path or mode
- which local receive receipts acknowledge that artifact
- what signer key state and trust outcome apply across the chain

Use `--commitment COMMITMENT-...` when you want the whole chain for a
commitment. That view summarizes the imported promise, imported evidence,
imported assessments, and receipt coverage together.

Each row shows:

- artifact CID, related local ID, kind, and protocol doc
- signer, signer key ID, signer key state, and identity path when resolvable
- import count, mode set, source-path set, and latest import
- trusted vs untrusted import counts under the current trust policy
- receipt count and receipt signers
- per-import lines so you can see repeated imports directly

Use `--json` when you want the same reconciliation chain in machine-readable
form for automation or audit scripts.

### `send`

```bash
go run ./cmd/commitment-ledger send --outbox /tmp/peer-outbox COMMITMENT-...
```

`send` is a convenience wrapper over `export` that writes a bundle file into a
peer-facing outbox directory with a generated filename.

### `receive`

```bash
go run ./cmd/commitment-ledger receive --inbox /tmp/peer-inbox
go run ./cmd/commitment-ledger receive --inbox /tmp/peer-inbox --archive /tmp/peer-archive
```

`receive` scans a local inbox directory for bundle files, imports them, and can
optionally archive the processed files after successful import.

By default, each successful `receive` also emits a signed local
`exchange_receipt` artifact acknowledging the imported bundle. Use
`--receipt-signer ''` to disable that behavior, or `--receipt-signer NAME` to
choose a different local receipt signer.

`import` and `receive` now fail fast on conflicts:

- same artifact CID with different indexed metadata
- same commitment/evidence/assessment ID with different projected content
- same imported signer or protocol support path with different bytes

Bundle files are also parsed strictly:

- unknown JSON fields are rejected
- missing required artifact fields are rejected before any local state changes

### `doctor`

```bash
go run ./cmd/commitment-ledger doctor
go run ./cmd/commitment-ledger doctor --json
go run ./cmd/commitment-ledger doctor --repairable
go run ./cmd/commitment-ledger doctor --strict
```

`doctor` checks:

- artifact index rows versus CAS presence and decodability
- indexed protocol, payload, and proof CIDs versus decoded envelope bytes
- primary, archived, and imported identity files can be parsed
- artifact signer/key pairs still resolve against current, archived, or imported identity material
- imported protocol metadata matches imported protocol document bytes

Treat a nonzero `doctor` result as a real local integrity problem until you
understand it.

Use `--repairable` when you want the current findings split into:

- issues the existing `repair` command may be able to address
- issues that still need operator investigation or manual recovery

Use `--strict` when warnings such as missing human-facing conformance files
should fail CI or audit runs instead of remaining informational.

### `repair`

```bash
go run ./cmd/commitment-ledger repair
go run ./cmd/commitment-ledger repair --records
go run ./cmd/commitment-ledger repair --protocol-cas
go run ./cmd/commitment-ledger repair --import-artifacts
go run ./cmd/commitment-ledger repair --import-support
go run ./cmd/commitment-ledger repair --identity-lineage
go run ./cmd/commitment-ledger repair --json --identity-lineage
```

`repair` is intentionally conservative. Today it can:

- rebuild commitment and assessment Markdown projection files from JSONL state
- restore built-in frozen protocol docs into local CAS
- restore missing imported artifact envelopes from previously recorded bundle source paths
- restore missing imported signer and protocol support files from previously recorded bundle source paths
- normalize archived identity filenames when the archived key material still exists locally under the wrong name

It does not recreate missing archived private keys that no longer exist anywhere
local. Those remain manual recovery cases.

It still does not resolve projection conflicts or synthesize bundle sources that
no longer exist locally.

When a saved bundle path is gone, `doctor` now treats that as a non-repairable
automation gap and tells you to recover the original bundle or re-import/export
it from another repo before retrying `repair`.

### `identity`

```bash
go run ./cmd/commitment-ledger identity list --json
go run ./cmd/commitment-ledger identity show Alice
go run ./cmd/commitment-ledger identity history Alice --json
go run ./cmd/commitment-ledger identity backup --include-imported-support --out /tmp/alice-identities.json Alice
go run ./cmd/commitment-ledger identity restore --in /tmp/alice-identities.json Alice
go run ./cmd/commitment-ledger identity rotate --name Alice
```

`identity` is the basic local signer lifecycle helper.

- `list` shows primary and imported identities
- `show` prints the current key ID, path, and public key for one name
- `history` shows the current key plus archived and imported key material for
  one signer name
- `backup` exports current plus archived local private identity material for one
  signer or for all local primary signers when no names are given; add
  `--include-imported-support` to also capture imported signer and protocol
  support material
- `restore` restores current and archived local private identity material from a
  backup file, restores bundled imported signer/protocol support when present,
  reports partial success, skips identical existing files, and flags explicit
  conflicts when local material differs from the backup
- `rotate` archives the old private key file under `config/identities/archive/`
  and writes a new keypair to the primary identity path

## Local State Layout

### `data/`

Append-only machine-readable projections:

- `data/work_items.jsonl`: latest-known and historical work observations
- `data/commitments.jsonl`: commitment projections
- `data/evidence.jsonl`: evidence projections
- `data/assessments.jsonl`: assessment projections
- `data/artifacts.jsonl`: local artifact index rows
- `data/imports.jsonl`: import and receive provenance rows
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

### `config/trust-policy.json`

Optional local trust-policy file controlling whether built-in or imported
signers, protocols, and import sources are treated as trusted by `verify`,
`status --exchange`, and `report --imports`.

### `docs/protocols/`

Frozen local protocol docs. The exact document bytes determine the local
`pCID`. Treat the artifact's `protocol_pcid` as authoritative when reading an
artifact.

### `CHANGELOG.md`

Repo-level conformance publication entries naming the exact frozen spec
doc-CIDs this implementation claims to speak. Read this together with emitted
`implementation_conformance` artifacts rather than as a replacement for them.

## Backup And Recovery

For a local backup, preserve these paths together:

- `data/`
- `records/`
- `config/identities/`
- `config/imported-identities/`
- `config/trust-policy.json` if present
- `docs/protocols/`
- `CHANGELOG.md`

For signer continuity, you can also export a dedicated identity backup file:

```bash
go run ./cmd/commitment-ledger identity backup --out /safe/path/identities.json
```

Add `--include-imported-support` if you also want the backup file to preserve
`config/imported-identities/` and `data/imported-protocols/`.

Recommended recovery flow:

1. Restore those paths together into a fresh checkout.
2. Run `make doctor`.
3. Run `make repair` if records, built-in protocol CAS objects, or imported artifact envelopes are missing.
4. Run `make status` and `make status STATUS_ARGS='--exchange'`.
5. Use `inspect` or `verify` on a few representative artifacts before resuming normal operation.

For a shorter command-oriented matrix, see `docs/recovery-checklist.md`.

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
- check `config/identities/archive/` if the signer rotated locally
- check `config/imported-identities/` if the artifact came from `import`
- use `inspect` to confirm the signer name carried in the proof

### `verify` says signer identity mismatch

Cause:

- the artifact proof was signed by a different key than the local identity file
- or the local identity file has changed since the artifact was emitted

What to do:

- inspect the artifact and signer identity carefully
- run `identity history NAME --json` to confirm whether the signing key is now
  archived or only present through imported support

### `doctor` reports archived identity filename mismatch

Cause:

- an archived key file still exists locally, but under the wrong filename

What to do:

- run `repair --identity-lineage`
- rerun `doctor` to confirm the mismatch is gone

### `doctor` reports missing archived key file for a historical signer

Cause:

- a historical private key expected under `config/identities/archive/` is gone
- or the only remaining copy is malformed and unreadable

What to do:

- first run `repair --identity-lineage` in case the key still exists under the
  wrong filename
- if that does not clear the finding, restore the archived identity file from
  backup
- use `identity history NAME --json` and `inspect` or `verify` on the affected
  artifact to confirm which key ID is missing

### `import` or `receive` says `... conflict ...`

Cause:

- the bundle is trying to introduce a local record or support file that already
  exists with different content

What to do:

- inspect the existing local artifact or record first
- inspect the incoming bundle in another checkout if needed
- decide which side is authoritative before retrying
- do not delete local state casually just to make the import pass
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
