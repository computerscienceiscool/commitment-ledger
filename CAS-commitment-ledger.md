# CAS Commitment Ledger

## Purpose

This note sketches how Commitment Ledger would look if it moved from
file-authoritative local storage to a CAS-first model more aligned with current
PromiseGrid guidance.

The current repo already keeps signed artifact envelopes in local CAS, but the
primary operational state still lives in JSONL files and Markdown records under
`data/` and `records/`. In a CAS-first design, immutable artifacts become the
primary durable state and local files become derived views, local refs, or
rebuildable caches.

## Current Shape

Today the storage model is effectively:

- append local JSONL rows for work, commitments, evidence, assessments, imports,
  and snapshots
- write Markdown projections for commitment and assessment records
- store signed artifact envelope bytes in local CAS
- join across those files to answer `status`, `inspect`, `report`, `verify`,
  and `reconcile`

This is workable, but it leaves the local projections more central than the
artifact bytes themselves.

## Design Goal

The intended direction is:

- protocol artifacts are primary
- exact bytes are retained by content address
- local state is reconstructed from artifacts plus explicit local refs
- printable or operator-facing files are projections rather than source of truth

That is a better fit for:

- PromiseGrid's `pCID`-first contract model
- unknown-`pCID` retention as evidence
- exchange of exact bytes between peers
- later reassessment from preserved artifacts
- cleaner portability across runtimes or storage backends

## Shared Baseline

Both CAS-first options below assume the same baseline.

### Primary durable objects

The following durable records should exist first as immutable CAS artifacts:

- commitment promise artifacts
- evidence artifacts
- assessment artifacts
- exchange receipt artifacts
- implementation promise claim / conformance artifacts
- optionally repo-observation artifacts for scan output
- optionally import provenance artifacts when provenance needs to travel

### Local-only state

The following should stay local by default unless a protocol explicitly says
otherwise:

- trust policy
- local signer key material and file paths
- repair hints
- UI and report caches
- temporary ingest bookkeeping
- demo-only setup state

### Ongoing need for local files

CAS-first does not mean no files on disk. It still needs:

- a local CAS object store
- local refs or heads
- index files or index chunks
- trust and identity configuration
- optional projection caches

The real shift is from file-ledger-first to artifact-first.

## Option A: Pure Artifact Graph

In this model, every meaningful durable record is an immutable artifact in CAS.
State is computed by traversing CID links between artifacts.

### Shape

- each commitment is one artifact
- each evidence record is one artifact linked to a commitment artifact CID
- each assessment is one artifact linked to a commitment artifact CID and one
  or more evidence artifact CIDs
- each exchange receipt links to imported artifact CIDs and local receipt
  context
- each repo observation or scan artifact links to the observed repo, branch,
  commit, and discovered work items

### Local state access

The runtime reconstructs views by walking the graph:

- `status` finds the latest relevant artifacts for each repo and commitment
- `inspect` resolves one CID or local alias and expands linked artifacts
- `report` aggregates graph slices by promiser, repo, branch, or work target
- `reconcile` follows import, receipt, and signer/protocol support links

### Strengths

- strongest fit with PromiseGrid artifact discipline
- no ambiguity about source of truth
- excellent portability between implementations
- easier peer exchange because the exchanged object is already the primary
  object
- clean provenance and reassessment story

### Weaknesses

- expensive queries if the graph grows large
- more work to answer operator-style questions quickly
- more local complexity for "what is current?" calculations
- likely needs indexes anyway once the repo gets bigger

### Best use

Choose this if the repo is optimizing hardest for protocol purity, portability,
and artifact auditability even at the cost of heavier query-time computation.

## Option B: Artifact Graph Plus Local Refs and Indexes

In this model, immutable artifacts in CAS are still the durable source of
truth, but the app also maintains explicit local refs and indexes for operator
workflows.

### Shape

Artifacts stay the same as in Option A. The difference is the addition of
small local structures such as:

- `refs/commitments/<commitment-id>` -> latest commitment-related artifact CID
- `refs/work/<repo>/<branch>/<work-id>` -> latest known work-observation CID
- `refs/repos/<repo>/<branch>/latest-scan` -> latest scan artifact CID
- `indexes/by-artifact/<cid>.json` -> local summary of linked records
- `indexes/by-commitment/<commitment-id>.json` -> cached artifact set and
  current interpreted state
- `indexes/by-repo/<repo>/<branch>.json` -> cached work and commitment summary

Those indexes are explicitly rebuildable from CAS plus local refs.

### Local state access

The runtime usually answers operator commands from indexes first:

- `status` reads repo and work indexes
- `inspect` expands one artifact CID plus cached linked metadata
- `report` aggregates from filtered indexes
- `doctor` verifies index-to-CAS consistency and can rebuild indexes if needed

### Strengths

- best operational performance for local CLI usage
- keeps artifacts primary while preserving simple operator UX
- easier migration path from the current repo shape
- simpler to add repair and rebuild workflows
- allows partial indexing and lazy rebuilds

### Weaknesses

- introduces another layer that can drift if not verified
- requires explicit rebuild rules and consistency checks
- still needs clear discipline so indexes do not quietly become authoritative

### Best use

Choose this if the repo is optimizing for a real working app while still moving
to CAS-first semantics.

## Recommendation

Option B is the better next step for Commitment Ledger.

Reason:

- the repo already behaves like an operator-facing local app
- commands like `status`, `report`, `inspect`, `doctor`, and `repair` want fast
  local answers
- the current JSONL and Markdown model can be migrated into rebuildable refs
  and indexes without losing usability
- it preserves the PromiseGrid direction that exact artifacts are primary

In other words:

- Option A is the cleaner theoretical endpoint
- Option B is the more credible implementation path

## What Would Change

If Commitment Ledger adopts Option B, the storage contract would become:

### Primary

- CAS objects containing exact protocol artifacts
- optional chunk-level CAS objects if larger artifact payloads need a named CAS
  profile later

### Secondary but durable

- local refs naming important heads
- local indexes that can be rebuilt from CAS and refs

### Disposable projections

- Markdown records
- JSON summaries for humans or automation
- imported support summaries
- report caches

## Likely Artifact Families

The current families would remain, but they would become more central:

- `commitment-promise-v1`
- `commitment-evidence-v2`
- `commitment-assessment-v2`
- `implementation-conformance-v1`
- `exchange-receipt-v1`

Likely new app-level families:

- repo work observation artifact
- import provenance artifact
- index checkpoint artifact, if the app ever wants signed or exchangeable
  summaries rather than purely local indexes

## Example Storage Layout

One possible local layout for Option B:

```text
data/
  cas/
    <cas-profile-pcid>/
      <chunk-cid>.bin
  refs/
    repos/<repo>/<branch>/latest-scan
    work/<repo>/<branch>/<work-id>
    commitments/<commitment-id>
    artifacts/<artifact-cid>
  indexes/
    by-commitment/<commitment-id>.json
    by-artifact/<artifact-cid>.json
    by-repo/<repo>/<branch>.json
records/
  commitments/<commitment-id>.md
  assessments/<assessment-id>.md
```

Important discipline:

- `data/cas/` is primary durable content
- `data/refs/` names current local heads
- `data/indexes/` is rebuildable local acceleration
- `records/` is human-facing projection only

## Migration Path

The safest migration would be incremental.

1. Keep writing current JSONL and Markdown projections.
2. Introduce explicit refs and artifact-linked indexes.
3. Make operator commands read indexes instead of raw JSONL where possible.
4. Add rebuild logic that regenerates indexes from CAS plus refs.
5. Reclassify JSONL files as compatibility projections or remove them once the
   index layer is stable.

That lets the app move toward CAS-first storage without freezing feature work
for a full rewrite.

## Open Questions

Before implementing, the repo needs explicit answers for:

- What is the protocol shape for repo observation artifacts?
- Which records stay purely local and which should be exchangeable?
- Are indexes always unsigned local cache, or do some become named artifacts?
- What is the GC and retention promise for local CAS?
- How are refs rebuilt after partial local loss?
- How should import provenance link to support material, receipts, and repaired
  state?
- When unknown-`pCID` artifacts arrive, what minimum local indexing promise
  should still be kept?

## Practical Summary

If Commitment Ledger moves toward CAS-first storage, it should not replace
"files on disk" with some abstract storage slogan. It should make one concrete
architectural change:

- exact CAS artifacts become the source of truth
- local refs and indexes become the operator layer
- Markdown and similar files become projections

That is the clearest path to a more PromiseGrid-aligned Commitment Ledger.
