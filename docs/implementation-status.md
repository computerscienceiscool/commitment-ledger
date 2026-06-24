# Implementation Status

## Provisional Grid

Commitment Ledger is being built against a provisional PromiseGrid landscape.
That means two different kinds of incompleteness need to stay separate:

- `Upstream-open`: PromiseGrid itself has not frozen one shared contract or
  settled design in that area yet.
- `Local-missing`: Commitment Ledger could implement something useful now, but
  this repo does not have it yet.

## Current Split

### Upstream-open

- One shared upstream frozen Commitment Ledger protocol does not exist yet.
- Final cross-project signing/envelope rules are still under active upstream
  comparison.
- Shared peer exchange, trust-accounting, CAS replication, and related app
  patterns are still being explored upstream rather than frozen as one final
  contract.

### Local-missing

- No peer-to-peer network transport yet.
- No shared remote import protocol yet.
- No richer shared trust-accounting view yet beyond local policy checks.
- No explicit migration story from these local protocol docs to any future
  upstream frozen specs yet.

Current local exchange support is bundle-based plus filesystem inbox/outbox
helpers: operators can `export`, `import`, `send`, and `receive` artifacts plus
support material, but there is still no shared network transport or peer
protocol layered over that.

Current local trust support is policy-based only: `verify`, `status --exchange`,
and `report --imports` can apply a local `config/trust-policy.json`, but that is
still operator-local trust classification rather than shared cross-peer trust.

Current local integrity checking is operator-side only: `doctor` can validate
artifact/CAS/support consistency inside one checkout, but there is still no
shared remote repair or quorum mechanism.

Current local repair support is intentionally narrow: `repair` can rebuild
Markdown projections, restore built-in protocol docs to CAS, and restore
missing imported artifact envelopes when the original bundle source paths are
still available locally, but it still does not resolve conflicting imported
state automatically.

Current local automation output is documented separately from protocol docs in
`docs/machine-readable-contracts.md`. That file currently publishes the local
CLI JSON compatibility contract as `cli-json-v1`.

## Conformance Claim Path

This repo now publishes conformance in two aligned ways:

- the `conformance` command emits a signed artifact saying which local protocol
  documents this implementation claims to speak
- `CHANGELOG.md` publishes repo-level conformance entries naming the exact
  frozen spec doc-CIDs in a human-facing audit trail

Both are explicit local statements of contract support, not a claim that
upstream PromiseGrid has frozen those exact docs for everyone else.

The current local conformance payload distinguishes between:

- frozen protocol docs the implementation can interpret locally
- frozen protocol docs current commands emit for new artifacts
- frozen historical docs retained for older local artifacts

That split is local version-accounting, not an upstream PromiseGrid-wide
migration contract.

Until upstream PromiseGrid freezes one shared Commitment Ledger app contract,
the repo-level `CHANGELOG.md` entries should be read as claims about these
local frozen docs, not as claims of universal upstream app-spec adoption.

## Current Operator Reminder

When inspecting `data/artifacts.jsonl`, remember that rows there are local
index entries over raw CAS objects. They are documented field-by-field in
`docs/promisegrid-app-design.md` and should not be mistaken for the protocol
artifact bytes themselves.

For day-to-day use, prefer the repo's operator-facing commands and guide
instead of reading projection files directly first:

- `status` for repo-level summary
- `report` for filtered views
- `inspect` for artifact and record lookup
- `verify` for local artifact and signer-material verification
- `docs/machine-readable-contracts.md` for the current JSON automation contract
- `docs/operator-guide.md` for normal workflow and troubleshooting
