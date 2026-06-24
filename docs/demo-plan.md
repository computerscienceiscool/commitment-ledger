# Demo Plan

## Purpose

This plan shows how to exercise Commitment Ledger against real local git repos
using the naming style and character roles that already recur in the
PromiseGrid development guide.

The goal is not only to prove that the CLI runs, but to demonstrate the
PromiseGrid-oriented lifecycle:

- work exists in ordinary repos
- a promiser makes a commitment against that work
- evidence is collected as exact local observations
- assessment stays local and explicit

## Cast

Use these names consistently in the demo:

- `Alice`: primary actor and good-faith promiser/promisee
- `Bob`: means well, often lets people down, and is useful for partial or late fulfillment cases
- `Dave`: reliable fallback or repair helper
- `Mallory`: adversarial or misleading actor used for malformed, confusing, or hostile cases

This matches the guide well enough to stay familiar:

- `Bob` is the best fit for the well-meaning but unreliable role
- `Mallory` is the best fit for the deliberate troublemaker role

## Real Repo Set

Create four real local repos under one demo parent directory:

```text
~/lab/commitment-ledger-demo/
  alice-demo/
  bob-demo/
  dave-demo/
  mallory-demo/
```

Suggested initialization:

```bash
mkdir -p ~/lab/commitment-ledger-demo
cd ~/lab/commitment-ledger-demo

mkdir alice-demo bob-demo dave-demo mallory-demo
for repo in alice-demo bob-demo dave-demo mallory-demo; do
  cd ~/lab/commitment-ledger-demo/$repo
  git init -b main
  mkdir -p TODO
done
```

Repo-native shortcut:

```bash
cd /home/jj/lab/commitment-ledger
make demo-setup
```

That target creates the demo repos, seeds baseline TODO content, and writes
`config/repos.demo.json`.

## TODO Shape

Use proquint-style TODOs consistent with the dev-guide style:

```md
# TODO Index

- [ ] TODO-ravud - Ship welcome flow (`TODO/TODO-ravud-ship-welcome-flow.md`)
- [ ] TODO-lomik - Write persistence note (`TODO/TODO-lomik-write-persistence-note.md`)
```

Example detail file:

```md
# TODO-ravud: Ship welcome flow

- [ ] 1. Add route
- [ ] 2. Add tests
- [ ] 3. Add docs
```

Use real detail files so the current parser can discover subtasks.

## Repo Roles

### alice-demo

Primary happy-path repo.

Use it to show:

- normal scan
- normal commitment creation
- normal evidence recording
- kept assessment

### bob-demo

Well-meaning but unreliable repo.

Use it to show:

- sincere commitment
- missed due date
- partial completion
- `expired_unassessed`
- later `partially_kept` or `broken`

### dave-demo

Reliable helper repo.

Use it to show:

- review or repair commitments
- successful kept assessment after Bob lets Alice down

### mallory-demo

Adversarial repo.

Use it to show:

- malformed TODO structures
- branch divergence intended to confuse observers
- misleading checked-state changes
- evidence that should be recorded but not trusted as a global verdict

Do not use Mallory as the only negative case. Bob and Mallory are different:

- Bob fails in good faith
- Mallory fails or confuses on purpose

## Branch Setup

Use at least one non-`main` branch so the branch-specific model is visible.

Suggested:

- `alice-demo`: `main`
- `bob-demo`: `main`
- `dave-demo`: `repair`
- `mallory-demo`: `jj`

This lets the demo show that:

- the same work ID on different branches is not automatically the same state
- branch-qualified work targets matter

## Commitment Ledger Config

Populate [config/repos.json](/home/jj/lab/commitment-ledger/config/repos.json) with all four repos before the first scan, or use the generated `config/repos.demo.json` from `make demo-setup`. The checked-in file currently ships with an empty `repos` list so the repo does not point at any machine-specific paths by default.

Example:

```json
{
  "repos": [
    {
      "name": "alice-demo",
      "provider": "local",
      "url": "",
      "local_path": "/home/jj/lab/commitment-ledger-demo/alice-demo",
      "branch": "main",
      "todo_file": "TODO/TODO.md",
      "enabled": true
    },
    {
      "name": "bob-demo",
      "provider": "local",
      "url": "",
      "local_path": "/home/jj/lab/commitment-ledger-demo/bob-demo",
      "branch": "main",
      "todo_file": "TODO/TODO.md",
      "enabled": true
    },
    {
      "name": "dave-demo",
      "provider": "local",
      "url": "",
      "local_path": "/home/jj/lab/commitment-ledger-demo/dave-demo",
      "branch": "repair",
      "todo_file": "TODO/TODO.md",
      "enabled": true
    },
    {
      "name": "mallory-demo",
      "provider": "local",
      "url": "",
      "local_path": "/home/jj/lab/commitment-ledger-demo/mallory-demo",
      "branch": "jj",
      "todo_file": "TODO/TODO.md",
      "enabled": true
    }
  ]
}
```

Current implementation note:

- `local_path`, `branch`, `todo_file`, and `enabled` drive behavior today
- `provider` and `url` are retained in the config shape but are not used by the local-only v0.1 scanner
- `make demo-config` writes the same shape to `config/repos.demo.json` for the default demo root

## Scenario 1: Alice Keeps a Promise

1. Add a TODO in `alice-demo`.
2. Commit the TODO files in `alice-demo`.
3. Run:

```bash
go run ./cmd/commitment-ledger scan --config config/repos.demo.json
go run ./cmd/commitment-ledger commit \
  --promiser Alice \
  --repo alice-demo \
  --branch main \
  --target alice-demo/main/TODO-ravud/1 \
  --due 2026-07-01 \
  --promise "I promise to complete TODO-ravud subtask 1."
```

Equivalent make-based entry points:

```bash
make demo-scan
make commit COMMIT_ARGS='--promiser Alice --repo alice-demo --branch main --target alice-demo/main/TODO-ravud/1 --due 2026-07-01 --promise "I promise to complete TODO-ravud subtask 1."'
```

4. Check off the subtask in `alice-demo/TODO/TODO-ravud-ship-welcome-flow.md`.
5. Commit that repo change.
6. Run scan again.
7. Assess:

```bash
go run ./cmd/commitment-ledger assess \
  --commitment COMMITMENT-... \
  --assessor Alice \
  --status kept \
  --notes "Completed before the due date."
```

Expected result:

- a commitment artifact exists
- evidence artifacts exist
- an assessment artifact exists
- local projections show `kept`

## Scenario 2: Bob Means Well and Lets People Down

1. Add a TODO in `bob-demo`.
2. Scan.
3. Create a Bob commitment with a near due date.
4. Complete only one subtask or miss the date.
5. Run:

```bash
go run ./cmd/commitment-ledger expire
```

6. Assess later as either:

- `partially_kept` if some promised work landed
- `broken` if the promise was not kept in a meaningful way

Expected result:

- the demo shows the difference between expiration and later judgment
- Bob is not treated as hostile, only unreliable

## Scenario 3: Dave Repairs or Reviews Successfully

1. Create work in `dave-demo` on branch `repair`.
2. Let Dave commit either to repair Bob's missed work or review Alice's work.
3. Complete and assess as `kept`.

Expected result:

- shows multiple commitments against related work
- shows a branch-qualified target outside `main`
- shows a reliable actor after Bob's failure

## Scenario 4: Mallory Tries to Confuse the System

Use `mallory-demo` for negative cases:

- malformed TODO entry formatting
- detail-file numbering that is strange or incomplete
- branch-local checked state that conflicts with another branch
- noisy or misleading manual evidence notes

Expected result:

- malformed or ambiguous input is surfaced or quarantined locally
- no global truth is inferred just because Mallory emitted something
- local assessment remains explicit

## What To Inspect

During the demo, inspect:

- `data/artifacts.jsonl`
- `data/commitments.jsonl`
- `data/evidence.jsonl`
- `data/assessments.jsonl`
- `data/cas/`
- `records/commitments/`
- `records/assessments/`

This shows the intended layering:

- raw artifacts in CAS
- append-only local indexes
- human-readable projections

## Current Limits Of This Demo

This demo is real in the sense that it uses actual local git repos, actual TODO
files, actual branches, and actual emitted artifacts.

It is still limited by current implementation scope:

- no remote peer exchange yet
- no shared network transport yet
- no import of artifacts from another running Commitment Ledger peer yet
- trust remains a documented future direction rather than a rich current feature

So this is a strong local-repo demo, not yet a full multi-peer PromiseGrid
exchange demo.

## Recommended First Demo Run

If you want the smallest useful first pass, do only these:

1. `alice-demo` happy path
2. `bob-demo` missed/partial path
3. `mallory-demo` malformed/confusing path

Add `dave-demo` immediately after that if you want the “reliable repair
actor” contrast.
