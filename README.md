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
commitment-ledger conformance --signer commitment-ledger --version v0.1.0
commitment-ledger expire
commitment-ledger status
commitment-ledger report --promiser JJ
```

## Config Contract

Each repo entry in `config/repos.json` currently supports these fields:

- `name`: stable repo name used in work targets and reports
- `local_path`: local git clone path to observe
- `branch`: expected checked-out branch; `scan` fails if the repo is on a different branch
- `todo_file`: path to the TODO index file inside the observed repo
- `enabled`: whether the repo participates in scans

The JSON shape also includes `provider` and `url`, but v0.1 is local-only and
does not use them yet.

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
- Assessment basis references must resolve to evidence artifacts for the same commitment.
- Protocol docs under `docs/protocols/` define local pCIDs by exact document bytes.
- Current emission stays on `commitment-promise-v1`, `implementation-conformance-v1`, `commitment-evidence-v2`, and `commitment-assessment-v2`; older frozen docs remain in-repo for historical pCID continuity.
- Commitments, evidence, assessments, and conformance claims are emitted as
  signed `grid([42(pCID), payload, proof])` artifacts stored in local CAS.
- JSONL and Markdown files are projections over those raw artifacts.
- Repo status summaries surface kept and non-kept terminal outcomes separately.

## Demo Docs

- `docs/demo-plan.md` lays out a real-repo demo using Alice, Bob, Dave, and Mallory roles.
- `docs/demo-script.md` is the spoken walkthrough with commands, files, and demo narration.
