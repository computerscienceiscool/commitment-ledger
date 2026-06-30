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
- The newer upstream storage/session/message pressure now points more clearly
  toward PromiseGrid-native reference sets plus sparse CAS as durable canonical
  state, with Git-style workflows treated as bridge adapters rather than source
  of truth, but that direction is still evidence rather than a frozen
  Commitment Ledger app contract.

### Local-missing

- No peer-to-peer network transport yet.
- No shared remote import protocol yet.
- No richer shared trust-accounting view yet beyond local policy checks.
- No explicit migration story from these local protocol docs to any future
  upstream frozen specs yet.
- No portable reference-set artifact layer yet; the branch work only adds
  local structured reference-set files for operator-side state.
- No explicit Git-bridge mapping, loss, or refusal semantics yet for native
  CAS-first state.
- No chunk-manifest story yet for larger or directory-like logical objects.

Current local exchange support is bundle-based plus filesystem inbox/outbox
helpers: operators can `export`, `import`, `send`, and `receive` artifacts plus
support material, but there is still no shared network transport or peer
protocol layered over that.

Current local storage is now moving in a more CAS-first direction on the
`cas-commitment-ledger` branch, but it is still an app-local implementation
step rather than a claim that PromiseGrid has already frozen one shared
reference-set, sparse-CAS, or bridge-adapter contract for this app family.

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

Read those statements as this implementation's current app-side promise claims
about the protocol documents it can interpret and emit. They are not yet a
kernel-style minimum-port promise record, and they are not evidence that the
upstream guide has frozen the first required storage/session/message spec set
for this app family.

The current local conformance payload distinguishes between:

- frozen protocol docs the implementation can interpret locally
- frozen protocol docs current commands emit for new artifacts
- frozen historical docs retained for older local artifacts

That split is local version-accounting, not an upstream PromiseGrid-wide
migration contract.

Until upstream PromiseGrid freezes one shared Commitment Ledger app contract,
the repo-level `CHANGELOG.md` entries should be read as claims about these
local frozen docs, not as claims of universal upstream app-spec adoption.

The same caution now applies to CAS-first storage plans: local refs, indexes,
and future reference-set artifacts may become a good implementation direction,
but they should not be presented as if the upstream guide has already frozen
one mandatory reference-set or Git-bridge shape for all PromiseGrid apps.

## Current Operator Reminder

When inspecting `data/artifacts.jsonl`, remember that rows there are local
index entries over raw CAS objects. They are documented field-by-field in
`docs/promisegrid-app-design.md` and should not be mistaken for the protocol
artifact bytes themselves.

The same distinction applies to any CAS-first local refs or indexes: they are
operator-side working state and rebuildable local acceleration unless and until
the app explicitly promotes part of that state into named portable artifacts.

For day-to-day use, prefer the repo's operator-facing commands and guide
instead of reading projection files directly first:

- `status` for repo-level summary
- `report` for filtered views
- `inspect` for artifact and record lookup
- `verify` for local artifact and signer-material verification
- `docs/machine-readable-contracts.md` for the current JSON automation contract
- `docs/operator-guide.md` for normal workflow and troubleshooting
