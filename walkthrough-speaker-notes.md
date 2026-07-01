# Walkthrough Speaker Notes

Speaker notes for driving `./walkthrough.sh` live. Keep this open on a second
screen. Run the script **without** `AUTO=1` so it pauses between steps — talk
during each pause, then press Enter to advance.

Format per step:
- **SHOW** — what's on screen.
- **SAY** — quotable talking points (say them in your own words).
- **WHY** — the point the step is really making.
- **IF ASKED** — likely audience questions and crisp answers.

> **The one sentence to land, up front and again at the end:**
> *"The readable files are just projections. The signed, content-addressed
> artifacts are the source of truth — and that's what makes this portable,
> auditable, and reassessable later."*

CAS (content-addressed storage) is the star of this talk. Every act quietly
builds toward the **Storage Tour (Act 6)**. Don't blow the whole CAS story in
Act 2 — plant it, then pay it off at the end.

---

## Opening framing (before you run anything)

**SAY:**
- "Commitment Ledger watches ordinary TODO-driven git repos and adds one thing
  they don't have: a record of *promises* made against that work, the *evidence*
  that the work happened, and a later *assessment* of the outcome."
- "It never edits the repos it observes. It only reads them."
- "The important mental shift: a task board tells you the *current* checkbox
  state. This tells you **who promised what, whether it happened, and how we
  judged it** — and it keeps the exact bytes of every one of those statements."
- "Everything you're about to see runs in a throwaway sandbox. I can run it ten
  times and never touch our real data. When it finishes it prints the sandbox
  path so you can poke at the files yourself."

**WHY:** Set expectations: this is *not* a task tracker, *not* editing repos,
and the storage model is deliberate. Those three claims get tested by the demo.

---

## SETUP — build + seed

**SHOW:** The build, the sandbox creation, and the fake "work" repo (`TODO.md`
plus a detail file with three unchecked subtasks).

**SAY:**
- "This is just an ordinary git repo with an ordinary TODO list. Nothing has
  been promised yet — this is only *available work*."
- "One design choice worth flagging now: the tool resolves all its state
  relative to where it runs, so I can point it at a sandbox and it stays fully
  isolated. Same reason it's safe to run against real repos later — it's a
  read-only observer of the source."

**WHY:** Establish the baseline: real repo, real TODO, zero promises. Everything
after this is the *layer the ledger adds*.

**IF ASKED:** *"Does it need a special repo format?"* → "Just a TODO index with
items like `TODO-ravud - Title` and subtasks like `- [ ] 1. Do thing`. That's
the whole contract."

---

## ACT 1 — WORK: `scan`

**SHOW:** `scan` output (open work / subtasks discovered / commit hash), then the
work items list: `demo/main/TODO-ravud`, `.../1`, `.../2`, `.../3`.

**SAY:**
- "The scan reads the repo's TODO files and turns them into machine-readable
  work items. It isn't inventing work and it isn't keeping a shadow task list by
  hand — it's *deriving* from what's in the repo."
- "Notice every target is **branch-qualified**: `demo/main/TODO-ravud/1`. There
  is no pretense of one global state independent of branch. A promise is always
  against a specific repo, branch, and target."

**WHY:** Work is *observed*, not authored. And identity is branch-scoped — this
matters later for honest assessment.

**IF ASKED:** *"Where did that come from?"* → "It's a projection into
`data/work_items.jsonl`. Hold that word — *projection* — it comes back at the
end."

---

## ACT 2 — PROMISE: `commit`  *(first CAS seed)*

**SHOW:** The `commit` command, the returned `COMMITMENT-… <CID>`, the
human-readable record, the artifact index row, the `.bin` file under `data/cas/`,
and `inspect --json`.

**SAY:**
- "This is the first real *promise*. Before this line there was only work; now
  Alice has committed to finishing subtask 1 by a due date."
- "Look at what came back: **two identifiers**. A human-facing `COMMITMENT-` id
  for operators, and a **CID** — a content-addressed identifier — for the exact
  signed artifact bytes."
- "The readable record and the index row both point *back* to that CID and to
  the protocol `pCID`. So the friendly Markdown is never the only copy — it's a
  view onto the real bytes."
- *(plant the CAS idea)* "And here's the actual artifact on disk:
  `data/cas/local-artifact-cas-v1/bafkre/<cid>.bin`. That file **is** the promise
  — signed, immutable, addressed by the hash of its own content. We'll come back
  to why that's the whole point."

**WHY:** Introduce content addressing gently: a promise is a *signed artifact*,
and everything human-facing is a pointer to it. Plant; don't over-explain yet.

**IF ASKED:**
- *"What's a pCID vs a CID?"* → "The **pCID** is the content address of the
  *protocol spec* — the rulebook the payload follows. The **CID** is the content
  address of *this specific artifact's* bytes. One says 'here are the rules,' the
  other says 'here is the exact message.'"
- *"Who signed it?"* → `inspect` shows `signer: Alice`, `signer_key_state:
  active`. "Each actor has a local ed25519 key; the artifact carries a signature
  over its bytes."

---

## ACT 3 — EVIDENCE: do the work, re-`scan`

**SHOW:** Alice checks subtask 1's box and commits it in the source repo; the
second `scan`; the new evidence row with `evidence_type: todo_checked`.

**SAY:**
- "Now Alice actually does the work — she ticks the box and commits it like any
  normal repo change. Crucial detail: **the evidence lives in the source repo**,
  not in something we typed into the ledger."
- "The second scan notices the promised box flipped and records that as
  **evidence**. Evidence is an *observation* — it is not yet a verdict."
- "That gap is deliberate. 'The box is checked' is not the same statement as
  'the promise was kept.' We refuse to collapse those two."

**WHY:** Evidence is observed fact, derived from the real repo — and it is
explicitly *separate* from judgment. This is the honesty of the model.

**IF ASKED:** *"Couldn't someone just check the box to fake it?"* → "They could
check a box — but that only produces *evidence*. Someone still has to make the
*assessment*, and assessment is a separate, signed, accountable act. See the
next step."

---

## ACT 4 — ASSESSMENT: `assess … --status kept`

**SHOW:** The `assess` command citing the evidence id; the assessment record.

**SAY:**
- "Now the separate, deliberate act: recording the outcome as **kept**, citing
  the evidence we just saw as the basis."
- "And the tool won't let you lie casually: `kept` is **validated against the
  latest scanned work state**. If the subtask weren't actually complete, this
  would be refused."
- "The assessment is itself another signed artifact — same treatment as the
  promise. Work, promise, evidence, assessment: four distinct, preserved
  statements."

**WHY:** Judgment is a first-class, validated, signed act — not an automatic
side effect of a checkbox.

**IF ASKED:** *"What other outcomes are there?"* → "kept, partially kept, broken,
refused, expired-unassessed, delegated, superseded, extended. Expiry, notably,
is recorded as `expired_unassessed` first — running out of time is not
automatically the same as breaking a promise."

---

## ACT 5 — VERIFY + REPORT

**SHOW:** `verify --json` on the assessment (`signature_verified`,
`signer_identity_verified`, `local_protocol_match`, `overall_trusted: true`);
then `status` and `report` showing Kept: 1.

**SAY:**
- "`verify` is the integrity check. It re-reads the bytes from CAS, recomputes
  the CIDs, checks the signature, checks the signer's identity material, and
  checks that the payload matches the protocol it claims."
- "Be precise about what this proves: it does **not** tell you Alice was a good
  person. It tells you the stored artifact is **internally and
  cryptographically consistent** — the bytes are intact, the signature is real,
  the protocol linkage holds. Trust is a separate, local judgment."
- "And `report` gives the whole chain in one view: work → promise → evidence →
  assessment."

**WHY:** Separate *integrity* (math) from *trust* (local policy). This is the
line people most often blur — draw it clearly.

**IF ASKED:**
- *"So `overall_trusted: true` means I should trust it?"* → "It means it passed
  local integrity **and** your local trust policy — here, a built-in signer with
  no extra policy loaded. Trust is always local and relationship-relative; the
  tool never claims global truth."
- *"Could I verify this on another machine?"* → "Yes — that's the payoff of
  content addressing. Ship the bytes, recompute the CID, verify the signature
  anywhere. Which brings us to storage…"

---

## ACT 6 — STORAGE TOUR  *(the crescendo — spend the most time here)*

**SHOW:** `data/cas/` (many `bafkrei…​.bin` files), `data/refs/` (logs +
`reference-sets/{work-observation,commitment-state,artifact-exchange}-heads`),
`data/indexes/` (grouped by state family), and `doctor --repairable` reporting
0 errors.

This is the point of the whole talk. Slow down.

**SAY — content addressing (the core idea):**
- "Every artifact we made — promise, evidence, assessment — lives here as an
  immutable file named by the **hash of its own content**: `bafkrei…​.bin`. The
  name isn't assigned; it's *computed from the bytes*."
- "Two consequences fall straight out of that. **Immutability**: change one byte
  and it's a different file with a different name — you can't silently rewrite
  history. **Deduplication and portability**: the same content is the same
  address everywhere, on any machine, forever. That `b...` name is CIDv1 base32,
  the standard printable form — same identifier on wire, in logs, and in JSON."

**SAY — source of truth vs projections (the thesis, paid off):**
- "Now the sentence from the start pays off. The Markdown records and the JSONL
  files? **Projections.** Disposable views. The durable source of truth is the
  signed bytes in `data/cas/`."
- "`data/refs/` names the current *heads* — reference sets like
  `commitment-state-heads` and `work-observation-heads`. `data/indexes/` is
  rebuildable acceleration so the CLI answers fast. Refs and indexes are the
  **operator layer**; they can be regenerated from CAS at any time."
- "So the layering is: **CAS = truth. Refs/indexes = operator convenience.
  Markdown/JSON = human-facing projections.** If you deleted every projection,
  the artifacts still tell the whole story."

**SAY — `doctor` (why the layering is safe):**
- "The risk of any cache layer is drift — the index quietly disagreeing with the
  truth. `doctor` is the guard: it checks that refs, indexes, and projections
  stay coherent with the CAS bytes, and it flags what's rebuildable. Zero errors
  here. That coherence check is what makes it safe to treat the friendly files
  as throwaway."

**SAY — where this is heading (optional forward-looking beat):**
- "This isn't just our idea. Upstream PromiseGrid is prototyping this exact
  direction as **POC18**: PromiseGrid-native *reference sets* and *sparse CAS*
  as canonical state, with Git treated as a **bridge adapter**, not the source
  of truth. Our reference-set files are the local, operator-grade version of
  that model. So we're skating where the puck is going — while being careful not
  to hard-code anything upstream hasn't frozen yet."

**WHY:** This is the differentiator. A task board has *current state*. This has
**immutable, portable, independently-verifiable history**, with human views as a
convenience on top. That's the whole pitch.

**IF ASKED:**
- *"Is this a blockchain?"* → "No. No global chain, no consensus, no mining.
  Just content-addressed storage plus signatures. Trust is local, not global —
  that's the Promise Theory framing: every artifact is one signer's local
  promise, and you decide locally whether to trust it."
- *"Why not just use git?"* → "Git is one possible *bridge*. But git gives you
  commit history of *files*; this gives you signed, protocol-typed *promises,
  evidence, and assessments* as first-class content-addressed objects you can
  exchange and reassess independently of any repo."
- *"What's `local-artifact-cas-v1`?"* → "Our implementation-local CAS profile
  name. We deliberately don't claim it's a frozen shared standard — upstream
  hasn't frozen a storage profile, so we stay honest about that."
- *"Can two people exchange these?"* → "Yes — export/import bundles and
  send/receive over a filesystem inbox exist today. Ship the bytes, verify on
  the other side. Network transport is future work."

---

## Closing

**SAY:**
- "So, end to end: ordinary repo → **promise** → **evidence** → **assessment**,
  every statement preserved as a **signed, content-addressed artifact**, with
  the readable files as disposable projections on top."
- "The tool draws two lines it never crosses: it doesn't edit your repos, and it
  doesn't pretend a checkbox is a verdict. And it keeps the exact bytes, so we
  can hand them to someone else, or reassess them later, without trusting our own
  summaries."
- *(repeat the thesis)* "The readable files are projections. The signed,
  content-addressed artifacts are the truth."
- "Next steps from here are richer peer exchange and trust accounting, and
  tracking the upstream POC18 reference-set direction."

---

## Timing (≈8–10 min)

| Segment | Minutes |
|---|---|
| Opening framing | 1 |
| Setup + Act 1 (work) | 1 |
| Act 2 (promise / plant CAS) | 1.5 |
| Act 3–4 (evidence / assessment) | 2 |
| Act 5 (verify / report) | 1.5 |
| **Act 6 (storage tour — the payoff)** | **2.5** |
| Closing | 0.5 |

If you're short on time, compress Acts 3–4 (say them in one breath: "she does
the work, we observe it as evidence, then we make a separate signed
assessment") and protect the full Act 6.

## Cheat sheet — five lines to memorize

1. "It observes repos; it never edits them."
2. "A checkbox is evidence. A verdict is a separate, signed act."
3. "`verify` proves integrity, not virtue. Trust is local."
4. "The name of each artifact is the hash of its own bytes — immutable and
   portable."
5. "CAS is the truth; refs and indexes are convenience; Markdown is a
   projection."
