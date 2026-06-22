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

- No peer-to-peer transport or artifact exchange yet.
- No import path for remote evidence or remote conformance claims yet.
- No richer local trust-accounting view yet.
- No explicit migration story from these local protocol docs to any future
  upstream frozen specs yet.

## Conformance Claim Path

The `conformance` command emits a signed artifact saying which local protocol
documents this implementation claims to speak. It is an explicit local
statement of contract support, not a claim that upstream PromiseGrid has frozen
those exact docs for everyone else.

## Current Operator Reminder

When inspecting `data/artifacts.jsonl`, remember that rows there are local
index entries over raw CAS objects. They are documented field-by-field in
`docs/promisegrid-app-design.md` and should not be mistaken for the protocol
artifact bytes themselves.
