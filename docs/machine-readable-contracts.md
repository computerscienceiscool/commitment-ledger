# Machine-Readable Contracts

## Purpose

This document defines the current JSON automation surface for Commitment
Ledger.

It is intentionally separate from the frozen protocol docs under
`docs/protocols/`:

- protocol docs define artifact meaning and protocol identity by `pCID`
- this file defines local CLI automation outputs and file formats

Those are related, but they are not the same contract.

## Current Contract Version

The current local CLI JSON contract version is:

- `cli-json-v1`

This version name covers:

- JSON emitted to stdout by CLI commands using `--json`
- JSON files written by local operator workflows such as `export` and
  `identity backup`

It does not redefine:

- signed artifact payload formats
- frozen protocol document bytes
- upstream PromiseGrid-wide conformance

## Compatibility Rules

Within `cli-json-v1`:

- existing top-level keys should not be renamed or removed
- existing scalar fields should keep the same meaning
- existing array element shapes should keep the same meaning
- new optional fields may be added
- new commands may add `--json` support
- text output may change without affecting this contract

Breaking changes require:

1. a new contract version name such as `cli-json-v2`
2. an update to this document
3. a repo-level note in `CHANGELOG.md`
4. matching operator documentation updates

## Stdout JSON Commands

These commands currently emit JSON to stdout:

- `status --json`
  - repo summary array keyed by repo and branch
- `status --exchange --json`
  - exchange/import summary object
- `report --json`
  - repo summary array, promiser summary object, work view object, or import
    summary object depending on flags
- `inspect --json`
  - artifact or record inspection object
- `verify --json`
  - artifact verification object
- `provenance --json`
  - import provenance row array
- `reconcile --json`
  - artifact-row array or commitment-chain object depending on flags
- `doctor --json`
  - local integrity summary object
- `repair --json`
  - local repair result object
  - includes whether `local_state` repair was applied and how many local state
    index surfaces were rebuilt
- `identity list --json`
  - identity summary array
- `identity show --json`
  - one local identity object
- `identity history NAME --json`
  - signer lineage object
- `identity backup --json`
  - backup export object when writing to stdout
- `identity restore --json`
  - restore result object
- `identity rotate --json`
  - rotate result object

## JSON File Formats

These local workflows also produce or consume JSON files:

- `export --out ...`
  - bundle file consumed by `import` and `receive`
- `send --outbox ...`
  - bundle file written to an exchange directory
- `identity backup --out ...`
  - backup file consumed by `identity restore --in ...`
- `config/trust-policy.json`
  - local trust-policy configuration

These file formats are local implementation contracts, not frozen protocol
specs.

Bundle files and `config/trust-policy.json` are parsed strictly:

- unknown JSON fields are rejected
- incomplete required sections are rejected

## Shape Notes

The automation surface intentionally uses a mix of arrays and objects:

- arrays are used for naturally repeated record views such as repo status rows,
  provenance rows, and identity lists
- objects are used for focused single-result views such as `verify`,
  `inspect`, `doctor`, `repair`, and identity history

Consumers should not assume every `--json` command returns an object.
They should select the command-specific shape documented here and in the
operator guide.

## Source Of Truth

The practical source of truth for `cli-json-v1` is:

1. this document
2. regression coverage in `cmd/commitment-ledger/main_test.go`
3. the implementation in `cmd/commitment-ledger/main.go`

If those drift, fix the code or docs together in one change.

## Relationship To Protocol Versioning

JSON automation versioning and protocol versioning move independently:

- adding a new local JSON field does not create a new protocol `pCID`
- adding a new frozen protocol doc version does not automatically change this
  JSON contract version

Only bump `cli-json-v1` when operator-facing JSON compatibility actually
changes.
