# commitment-ledger

Commitment Ledger is a CLI-first PromiseGrid app prototype for tracking
commitments made against existing repository TODO work, recording evidence, and
preserving later assessment as signed, CID-addressed protocol artifacts.

## Quick Start

1. Populate `config/repos.json` with the local git repos you want to observe.
   The tracked file in this repo ships empty on purpose.
2. Run `go run ./cmd/commitment-ledger scan --config config/repos.json` to
   discover work items from those repos.
3. Create commitments against discovered branch-qualified targets with
   `go run ./cmd/commitment-ledger commit ...`.
4. Re-run `scan` to derive local evidence, then use `assess` to record the
   final judgment.

## Layout

- `cmd/commitment-ledger`: CLI entrypoint
- `internal/config`: repo tracking configuration loader
- `internal/gitrepo`: local git observation helpers
- `internal/protocol`: local protocol docs and pCID loading
- `internal/cid`: local content-addressed identifier generation
- `internal/cas`: repo-local content-addressed storage
- `internal/grid`: signed grid-envelope encoding
- `internal/identity`: local signer identity material
- `internal/todo`: TODO and subtask parser
- `internal/ledger`: JSONL and Markdown storage
- `internal/commitment`: commitment validation and lifecycle updates
- `internal/evidence`: scan-derived and manual evidence helpers
- `internal/assessment`: manual assessment helpers
- `internal/report`: summary generation
- `config/repos.json`: tracked repo configuration
- `CHANGELOG.md`: repo-level conformance publication entries
- `docs/`: design notes, protocol docs, and the source spec PDF
- `docs/protocols/`: local protocol documents frozen by exact bytes
- `data/`: append-only machine-readable ledger files
- `data/cas/`: raw content-addressed protocol artifacts
- `records/`: human-readable commitment and assessment records

## Commands

```text
commitment-ledger scan --config config/repos.json
commitment-ledger commit --promiser JJ --repo repo --branch main --target repo/main/TODO-abcd --due 2026-06-30 --promise "I promise ..."
commitment-ledger evidence --commitment COMMITMENT-... --type manual_note --notes "Observed blocker"
commitment-ledger assess --commitment COMMITMENT-... --assessor JJ --status kept --notes "Completed on time"
commitment-ledger conformance --signer commitment-ledger --version v0.1.0 --write-changelog
commitment-ledger expire
commitment-ledger status
commitment-ledger status --exchange
commitment-ledger report --promiser JJ
commitment-ledger report --imports
commitment-ledger inspect COMMITMENT-...
commitment-ledger verify COMMITMENT-...
commitment-ledger export --out /tmp/bundle.json COMMITMENT-...
commitment-ledger import --in /tmp/bundle.json
commitment-ledger send --outbox /tmp/peer-outbox COMMITMENT-...
commitment-ledger receive --inbox /tmp/peer-inbox --archive /tmp/peer-archive
commitment-ledger doctor
```

## Make Targets

The repo includes a `Makefile` so routine local workflows do not depend on
Codex access.

Common targets:

- `make help`: list supported development, CLI, and demo targets
- `make fmt`: run `gofmt` across the repo
- `make test`: run `go test ./...`
- `make build`: build `bin/commitment-ledger`
- `make check`: run formatting, tests, and a local build
- `make cli ARGS='status'`: run arbitrary CLI commands through the standard local wrapper
- `make scan CONFIG=config/repos.json`: scan a configured repo set
- `make status STATUS_ARGS='--exchange'`: run the default repo summary or the exchange/import summary
- `make report REPORT_ARGS='--promiser Alice'`: run a filtered report
- `make report REPORT_ARGS='--imports'`: summarize imported artifacts by source path and trust result
- `make inspect INSPECT_ARGS='COMMITMENT-...'`: inspect a commitment ID, evidence ID, assessment ID, or artifact CID
- `make verify VERIFY_ARGS='COMMITMENT-...'`: verify a commitment ID, evidence ID, assessment ID, or artifact CID against local CAS bytes and signer material
- `make export EXPORT_ARGS='--out /tmp/bundle.json COMMITMENT-...'`: export an artifact bundle with related projection rows and support material
- `make import IMPORT_ARGS='--in /tmp/bundle.json'`: import an artifact bundle and optionally install bundled support material
- `make send SEND_ARGS='--outbox /tmp/peer-outbox COMMITMENT-...'`: write a bundle into a peer-facing outbox directory
- `make receive RECEIVE_ARGS='--inbox /tmp/peer-inbox --archive /tmp/peer-archive'`: import all bundle files from a peer inbox directory
- `make doctor`: verify local artifact, CAS, and imported support integrity
- `make conformance VERSION=v0.1.0 SIGNER=commitment-ledger`: emit a local conformance claim
- `make conformance-update VERSION=v0.1.0 SIGNER=commitment-ledger`: emit a conformance artifact and refresh the managed `CHANGELOG.md` entries

Demo-oriented targets:

- `make demo-setup`: create and seed demo repos under `$(HOME)/lab/commitment-ledger-demo` and write `config/repos.demo.json`
- `make demo-scan`: scan the generated demo config
- `make demo-status`: show the current local status summary
- `make demo-report REPORT_ARGS='--promiser Alice'`: run a demo-oriented report

## Config Contract

Each repo entry in `config/repos.json` currently supports these fields:

- `name`: stable repo name used in work targets and reports
- `local_path`: local git clone path to observe
- `branch`: expected checked-out branch; `scan` fails if the repo is on a different branch
- `todo_file`: path to the TODO index file inside the observed repo
- `enabled`: whether the repo participates in scans

The JSON shape also includes `provider` and `url`, but v0.1 is local-only and
does not use them yet.

## Trust Policy

If `config/trust-policy.json` exists, `verify`, `status --exchange`, and
`report --imports` use it for local trust evaluation.

Current supported fields:

- `trust_built_in_signers`: whether identities under `config/identities/` are trusted by default
- `trust_built_in_protocols`: whether built-in frozen docs under `docs/protocols/` are trusted by default
- `trusted_signers`: signer names trusted even when they come from imported support
- `trusted_protocol_pcids`: protocol pCIDs trusted even when they come from imported support
- `trusted_import_modes`: allowed import modes such as `import` or `receive`
- `trusted_import_path_prefixes`: path prefixes whose imported bundles are trusted locally

## TODO Parser Contract

The current parser recognizes:

- top-level items shaped like `001 - Title` or `TODO-ravud - Title`
- optional checkboxes on those lines, such as `- [x] TODO-ravud - Title`
- optional detail-file links in backticks, such as
  ``TODO-ravud - Title (`TODO/TODO-ravud-title.md`)``
- subtask lines in detail files shaped like `- [ ] 1. Do thing` or
  `- [x] 2.1 Do thing`

Observed work targets are always branch-qualified, for example
`repo/main/TODO-ravud/1`.

## Notes

- v0.1 observes local git clones only.
- Source repositories are not edited by this tool.
- Ledger records stay in this repository.
- Manual evidence must stay within the referenced commitment's repo, branch, and promised target scope.
- Assessments may move commitments from `open` or `expired_unassessed` into a terminal outcome, but they do not overwrite already-finalized commitments.
- `kept` is validated against the latest scanned work state; parent TODO promises require all discovered subtasks to be complete.
- Assessment basis references must resolve to evidence artifacts for the same commitment.
- Protocol docs under `docs/protocols/` define local pCIDs by exact document bytes.
- Current emission stays on `commitment-promise-v1`, `implementation-conformance-v1`, `commitment-evidence-v2`, and `commitment-assessment-v2`; older frozen docs remain in-repo for historical pCID continuity.
- Conformance is published in two forms: signed `implementation_conformance` artifacts and repo-level `CHANGELOG.md` entries naming exact frozen spec doc-CIDs.
- Commitments, evidence, assessments, and conformance claims are emitted as
  signed `grid([42(pCID), payload, proof])` artifacts stored in local CAS.
- JSONL and Markdown files are projections over those raw artifacts.
- Repo status summaries surface kept and non-kept terminal outcomes separately.
- `inspect` resolves commitment IDs, evidence IDs, assessment IDs, and artifact CIDs back to their local artifact metadata, frozen protocol docs, matching `CHANGELOG.md` conformance entries, and latest import provenance when present.
- `verify` checks local CAS bytes, envelope/payload/proof CIDs, the signature, matching local signer identity material, and optional local trust policy over signer, protocol, and import source.
- `export` writes a portable bundle containing the artifact index row, envelope bytes, related projection rows, and available signer/protocol support material.
- `import` loads that bundle back into local CAS and projections, can install bundled signer/protocol support material for later `inspect` and `verify` use, and records import provenance in `data/imports.jsonl`.
- `import` rejects conflicting commitment, evidence, assessment, signer-support, and protocol-support state instead of silently diverging local history.
- `send` and `receive` add a local filesystem inbox/outbox exchange path on top of the bundle format; they are still not network transport.
- `doctor` checks local artifact index entries against CAS bytes and validates imported support files.

## Backup And Recovery

For a reliable local backup, capture these together:

- `data/`
- `records/`
- `config/identities/`
- `config/imported-identities/`
- `config/trust-policy.json` if you use it
- `docs/protocols/` and `CHANGELOG.md`

After restoring, run `make doctor` before trusting the restored state.

## Demo Docs

- `docs/demo-plan.md` lays out a real-repo demo using Alice, Bob, Dave, and Mallory roles.
- `docs/demo-script.md` is the spoken walkthrough with commands, files, and demo narration.
- `docs/operator-guide.md` is the practical runbook for local operation, inspection, and troubleshooting.
- `docs/trust-and-verification.md` explains what local artifact verification proves and what it does not prove.

## Version Notes

- `docs/protocol-migration.md` explains the local `v1` to `v2` evidence and assessment transition and how conformance distinguishes claimed, emitted, and historical frozen protocol docs.
- `CHANGELOG.md` mirrors the current claimed, emitted, and historical protocol surface in the repo-level publication shape the PromiseGrid dev guide points App Devs toward.
