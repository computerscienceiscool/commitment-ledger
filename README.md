# commitment-ledger

Commitment Ledger is a CLI-first PromiseGrid app prototype for tracking
commitments made against existing repository TODO work, recording evidence, and
preserving later assessment as signed, CID-addressed protocol artifacts.

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

## Notes

- v0.1 observes local git clones only.
- Source repositories are not edited by this tool.
- Ledger records stay in this repository.
- Protocol docs under `docs/protocols/` define local pCIDs by exact document bytes.
- Commitments, evidence, assessments, and conformance claims are emitted as
  signed `grid([42(pCID), payload, proof])` artifacts stored in local CAS.
- JSONL and Markdown files are projections over those raw artifacts.
