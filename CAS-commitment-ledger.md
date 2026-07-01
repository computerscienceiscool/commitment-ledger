# CAS Commitment Ledger

## Purpose

This note sketches how Commitment Ledger would look if it moved from
file-authoritative local storage to a CAS-first model more aligned with current
PromiseGrid guidance.

As of the June 29, 2026 PromiseGrid dev-guide refresh, that guidance now leans
more clearly toward:

- PromiseGrid-native reference sets as durable canonical state
- sparse CAS as the primary content store behind those reference sets
- Git import/export/push/pull as bridge adapters rather than source of truth
- chunk-manifest pressure for large-file or directory-like state

Since that refresh, this pressure has become concrete rather than purely
advisory. POC18 now ships an implementation-local protocol spec,
`version-control.md`, plus a first executable core slice,
`poc18-cas-git-replacement`, under `wire-lab`. That spec defines exactly the
model this note is reaching for: signed, versioned reference sets and sparse CAS
as the source of truth, Rabin content-defined chunk manifests for file content,
all POSIX inode types, and Git import/export/push/pull as bridge adapters. It is
still explicitly unfrozen evidence, not a settled storage or version-control
contract (`DR-davod` remains open), so Commitment Ledger should track it as a
reference point rather than a required target.

This note therefore treats refs and indexes not only as implementation
conveniences, but as the beginning of a more native reference-set model.

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
- local state is reconstructed from artifacts plus explicit local refs or
  reference sets
- printable or operator-facing files are projections rather than source of truth

That is a better fit for:

- PromiseGrid's `pCID`-first contract model
- unknown-`pCID` retention as evidence
- exchange of exact bytes between peers
- later reassessment from preserved artifacts
- cleaner portability across runtimes or storage backends
- the newer PromiseGrid direction that durable collaboration state may have its
  own PromiseGrid-native reference-set story instead of collapsing into live
  sync or Git history

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
- local refs, heads, or reference-set views
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

Under the newest dev-guide pressure, this option should be read as a stepping
stone toward a more native reference-set model:

- refs are not just mutable convenience pointers
- they are candidates for durable PromiseGrid-facing reference-set semantics
- one generic head file is too vague; named state-family sets are a better
  local approximation of the PromiseGrid direction
- Git-like branch or tag behavior, if ever needed, should be treated as a
  bridge adapter over native state rather than the canonical source

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
- it leaves room for the newer PromiseGrid-native reference-set direction
  without forcing Commitment Ledger to pretend Git history is canonical state

In other words:

- Option A is the cleaner theoretical endpoint
- Option B is the more credible implementation path

## What Would Change

If Commitment Ledger adopts Option B, the storage contract would become:

### Primary

- CAS objects containing exact protocol artifacts
- optional chunk-level CAS objects when larger artifact payloads or
  directory-like state need a named CAS profile
- optional chunk manifests when one artifact needs to name a structured set of
  smaller CAS objects

### Secondary but durable

- local refs or reference-set views naming important heads
- local state-family reference sets such as work observation heads, commitment
  state heads, and artifact exchange heads
- local indexes that can be rebuilt from CAS and refs
- bridge metadata when Git import/export/push/pull is used as an adapter rather
  than a source of truth

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
- reference-set artifact, if local refs need a portable artifact form later;
  POC18 models this as a signed, versioned `reference_set` promise with typed
  `previous_reference_set` parent links, so the local `*-heads` reference-set
  files are the operator-grade approximation of that native form
- chunk-manifest artifact, if one logical Commitment Ledger object grows beyond
  one convenient CAS object; POC18 uses Rabin content-defined chunking for all
  regular-file content, though Commitment Ledger's small signed artifacts do not
  need chunking yet

## Example Storage Layout

One possible local layout for Option B:

```text
data/
  cas/
    <implementation-local-cas-profile>/
      <chunk-cid>.bin
  refs/
    repos/<repo>/<branch>/latest-scan
    work/<repo>/<branch>/<work-id>
    commitments/<commitment-id>
    artifacts/<artifact-cid>
    reference-sets/work-observation-heads
    reference-sets/commitment-state-heads
    reference-sets/artifact-exchange-heads
  indexes/
    work-observation/<index-name>.json
    commitment-state/<index-name>.json
    artifact-exchange/<index-name>.json
  bridges/
    git/<bridge-name>.json
records/
  commitments/<commitment-id>.md
  assessments/<assessment-id>.md
```

Important discipline:

- `data/cas/` is primary durable content
- the current branch can use one implementation-local CAS profile name without
  claiming that upstream PromiseGrid has already frozen the final shared
  CAS-profile pCID for this app family
- `data/refs/` names current local heads and structured state-family reference
  sets
- `data/indexes/` is rebuildable local acceleration grouped by state family
- `data/bridges/` records adapter-specific state and should not quietly become
  canonical PromiseGrid state
- `records/` is human-facing projection only

## Migration Path

The safest migration would be incremental.

1. Keep writing current JSONL and Markdown projections.
2. Introduce explicit refs and artifact-linked indexes.
3. Make operator commands read indexes instead of raw JSONL where possible.
4. Add rebuild logic that regenerates indexes from CAS plus refs.
5. Reclassify JSONL files as compatibility projections or remove them once the
   index layer is stable.

If later PromiseGrid work hardens around native reference sets plus Git bridge
adapters, an additional follow-on step would be:

6. Promote selected local refs into explicit reference-set artifacts and treat
   any Git sync as import/export over those native sets rather than as the
   canonical state model. The POC18 `reference_set` promise is the concrete
   target shape here: a signed, versioned labeled set with typed
   `previous_reference_set` parent links, paired with a `git_bridge_mapping`
   promise that records explicit loss or non-commitment whenever Git cannot
   faithfully carry a native structure.

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
- Which local refs are merely app-local working memory, and which should become
  portable reference-set artifacts if the app later needs cross-peer durable
  collaboration state?
- If bridge adapters are used, what loss, refusal, or downgrade semantics apply
  when Git cannot faithfully carry a native Commitment Ledger reference-set or
  chunk-manifest structure?

## POC18 Reference Points

POC18's `version-control.md` does not settle these questions for Commitment
Ledger. It is a different protocol family (files, directories, branches) and is
still unfrozen. But it does give a concrete upstream shape for several of them.
The spec and slice live at:

- `wire-lab/implementations/poc18-cas-git-replacement/docs/protocols/version-control.md`
- `wire-lab/implementations/poc18-cas-git-replacement/` (executable core slice)
- supporting notes: `DN-rifir` (versioned reference-set design), `DN-dopod`
  (Tangled prior-art comparison), `TE-kopap` (native core plus Git bridge)

Mapping the open questions above onto the POC18 direction:

- *Indexes vs named artifacts.* POC18 keeps indexes as explicitly local derived
  state ("exact CAS bytes remain the source material") while promoting reference
  sets to signed, versioned artifacts. That confirms the split this note
  proposes: `data/indexes/` stays unsigned and rebuildable; `data/refs/`
  reference sets are the candidates for a portable signed form.
- *GC and retention.* POC18 makes retention a promise rather than a silent
  policy: `object_retention` and `object_availability` promise kinds let an
  agent publish what it will retain, serve, or collect instead of deleting
  objects it had promised to keep.
- *Refs after partial loss.* POC18 treats a partial store as normal sparse-DAG
  state; missing parents are requested through `sync_interest`, not read as bad
  faith. Locally, Commitment Ledger still rebuilds refs and indexes from CAS
  plus retained artifacts, which is the same posture.
- *Unknown-`pCID` retention.* POC18 receivers keep exact bytes and treat an
  unsupported promise kind as local non-commitment rather than failure, matching
  this note's intent to retain unknown-`pCID` artifacts as evidence.
- *Which refs become portable.* The signed `reference_set` promise is the
  portable form; a ref stays app-local working memory until it needs to be
  exchanged, at which point it becomes a `reference_set` artifact.
- *Git bridge loss semantics.* POC18's `git_bridge_mapping` requires explicit
  `loss_records` and one shared conversion core, and forbids making a Git
  remote, forge, or hidden ref the source of truth. That is the contract to copy
  if Commitment Ledger ever adds a Git bridge.

One envelope difference is worth noting deliberately. POC18's native envelope is
four slots, `grid([42(pCID), parents, payload, proof])`, with a first-class
typed `parents` slot carrying version and response ancestry. Commitment Ledger's
own pCIDs use three slots, `grid([42(pCID), payload, proof])`, and express
linkage (evidence to commitment, assessment to evidence) inside payloads and
projections instead. That is legitimate, since each `pCID` owns its own slot
grammar, but if the app later wants tighter POC18 alignment, elevating artifact
ancestry into explicit typed parent links is the direction to take.

Finally, POC18 does not define a production storage layout, and the dev guide
still does not freeze a universal storage profile or version-control contract.
That validates the discipline this note already follows: keep using an
implementation-local CAS profile name such as `local-artifact-cas-v1` without
claiming it is a shared frozen profile, and treat the whole POC18 surface as a
tracked direction rather than a required target.

## Practical Summary

If Commitment Ledger moves toward CAS-first storage, it should not replace
"files on disk" with some abstract storage slogan. It should make one concrete
architectural change:

- exact CAS artifacts become the source of truth
- local refs, reference sets, and indexes become the operator layer
- Markdown and similar files become projections

That is the clearest path to a more PromiseGrid-aligned Commitment Ledger.
