# Team Demo Script

## Purpose

This file is the live walkthrough script for demonstrating Commitment Ledger to
the team.

It combines:

- the demo repo files to point at
- the exact commands to run
- suggested narration for each step

This is meant to be read aloud while driving the terminal.

## Demo Cast

Use these names consistently:

- `Alice`: good-faith actor and first promiser
- `Bob`: means well and often lets people down
- `Dave`: reliable fallback
- `Mallory`: adversarial actor who tries to confuse things

## Demo Repo Paths

The live demo repos are:

```text
/home/jj/lab/commitment-ledger-demo/alice-demo
/home/jj/lab/commitment-ledger-demo/bob-demo
/home/jj/lab/commitment-ledger-demo/dave-demo
/home/jj/lab/commitment-ledger-demo/mallory-demo
```

The main app repo is:

```text
/home/jj/lab/commitment-ledger
```

## Opening Framing

What to say:

> Commitment Ledger is not editing team repos. It observes ordinary repos that
> already have TODO files, records promises made against that work, keeps exact
> artifact bytes locally, and lets us assess what happened later.

> The important shift is that we are not treating a task board as the whole
> truth. We are tracking work, promises about that work, evidence, and later
> judgment.

## Step 1: Show The Source Work

Commands:

```bash
sed -n '1,40p' /home/jj/lab/commitment-ledger-demo/alice-demo/TODO/TODO.md
sed -n '1,40p' /home/jj/lab/commitment-ledger-demo/alice-demo/TODO/TODO-ravud-ship-welcome-flow.md
```

What to say:

> This is just an ordinary repo with an ordinary TODO index and a detail file.

> Alice has not promised anything yet. This is only available work.

> The top-level TODO is `TODO-ravud`, and it has three subtasks underneath it.

What to point at:

- `TODO-ravud`
- `TODO-lomik`
- subtask `1. Add route`
- subtask `2. Add tests`
- subtask `3. Add docs`

## Step 2: Show Observation Of Real Repos

Commands:

```bash
cd /home/jj/lab/commitment-ledger
GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger scan --config config/repos.json
```

What to say:

> Now the ledger scans real local git repos. It is not inventing work and it is
> not maintaining a separate shadow task list by hand.

> It discovers work from Alice, Bob, Dave, and Mallory repos, and records that
> as branch-qualified work items.

What to point at:

- `alice-demo main`
- open work count
- subtask discovery count
- current commit hash

## Step 3: Narrow To Alice’s Discovered Work

Commands:

```bash
grep '"repo":"alice-demo"' /home/jj/lab/commitment-ledger/data/work_items.jsonl
grep 'TODO-ravud' /home/jj/lab/commitment-ledger/data/work_items.jsonl
```

What to say:

> This is the projection layer. The ledger has turned the repo files into
> machine-readable work items.

> The important thing to notice is that the targets are branch-qualified. We are
> not pretending there is one global state independent of branch.

What to point at:

- `alice-demo/main/TODO-ravud`
- `alice-demo/main/TODO-ravud/1`
- `alice-demo/main/TODO-ravud/2`
- `alice-demo/main/TODO-ravud/3`

## Step 4: Create Alice’s Commitment

Commands:

```bash
GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger commit \
  --promiser Alice \
  --repo alice-demo \
  --branch main \
  --target alice-demo/main/TODO-ravud/1 \
  --due 2026-07-01 \
  --promise "I promise to complete TODO-ravud subtask 1."
```

What to say:

> This is the first actual promise. Before this point, there was only work.

> The command returns two identifiers: a local commitment ID for operators and a
> CID for the emitted artifact bytes.

> That means we have both a human-facing handle and a content-addressed message
> identity.

What to point at:

- `COMMITMENT-...`
- `b...` artifact CID

## Step 5: Show The Human Projection

Commands:

```bash
COMMITMENT_ID=$(tail -n 1 /home/jj/lab/commitment-ledger/data/commitments.jsonl | sed -E 's/.*"commitment_id":"([^"]+)".*/\1/')
cat /home/jj/lab/commitment-ledger/records/commitments/$COMMITMENT_ID.md
```

What to say:

> This is the human-readable projection of the commitment.

> It includes the artifact CID and the protocol pCID, so the readable record can
> still point back to the exact bytes and the exact protocol document.

What to point at:

- `Artifact CID`
- `Protocol pCID`
- `Status: open`
- target `alice-demo/main/TODO-ravud/1`

## Step 6: Show The Artifact Index

Commands:

```bash
grep "$COMMITMENT_ID" /home/jj/lab/commitment-ledger/data/artifacts.jsonl
```

What to say:

> This is the local artifact index row.

> It is not the protocol artifact itself. It is a projection that tells us where
> the exact bytes live, which protocol owns the payload meaning, who signed it,
> and which local report ID it relates to.

What to point at:

- `artifact_cid`
- `protocol_pcid`
- `kind`
- `signer`
- `payload_cid`
- `proof_cid`
- `related_id`

## Step 7: Make Alice Actually Do The Work

Commands:

```bash
python3 - <<'PY'
from pathlib import Path
path = Path('/home/jj/lab/commitment-ledger-demo/alice-demo/TODO/TODO-ravud-ship-welcome-flow.md')
text = path.read_text()
text = text.replace('- [ ] 1. Add route', '- [x] 1. Add route')
path.write_text(text)
PY

git -C /home/jj/lab/commitment-ledger-demo/alice-demo add TODO/TODO-ravud-ship-welcome-flow.md
git -C /home/jj/lab/commitment-ledger-demo/alice-demo -c user.name=Alice -c user.email=alice@example.com commit -m "Complete Alice subtask 1"
```

What to say:

> Now Alice has actually changed the source repo. This is important: the source
> evidence lives in the work repo, not only in the ledger repo.

> We are changing the actual subtask state and committing it like normal repo
> work.

## Step 8: Scan Again To Capture Evidence

Commands:

```bash
GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger scan --config config/repos.json
grep 'todo_checked' /home/jj/lab/commitment-ledger/data/evidence.jsonl
```

What to say:

> The second scan sees that Alice’s promised subtask is now checked.

> That creates evidence. Evidence is not yet the final judgment, but it is the
> observable basis for judgment.

What to point at:

- `todo_checked`
- commitment reference
- target `alice-demo/main/TODO-ravud/1`

## Step 9: Assess Alice As Kept

Commands:

```bash
EVIDENCE_ID=$(tail -n 1 /home/jj/lab/commitment-ledger/data/evidence.jsonl | sed -E 's/.*"evidence_id":"([^"]+)".*/\\1/')

GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger assess \
  --commitment "$COMMITMENT_ID" \
  --assessor Alice \
  --status kept \
  --basis "$EVIDENCE_ID" \
  --notes "Completed before the due date."
```

What to say:

> Now we move from evidence to explicit assessment.

> The key design point is that checked work is evidence, but the final judgment
> is still a separate local act.

> This keeps the system from pretending that every observable state change is
> already a universal verdict.

## Step 10: Show The Result

Commands:

```bash
ASSESSMENT_ID=$(tail -n 1 /home/jj/lab/commitment-ledger/data/assessments.jsonl | sed -E 's/.*"assessment_id":"([^"]+)".*/\1/')
cat /home/jj/lab/commitment-ledger/records/assessments/$ASSESSMENT_ID.md
GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger report --promiser Alice
```

What to say:

> At this point we can see the full chain: work, promise, evidence, and
> assessment.

> This is the basic PromiseGrid-shaped story the tool is trying to make visible.

## Step 11: Contrast Bob

Commands:

```bash
GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger commit \
  --promiser Bob \
  --repo bob-demo \
  --branch main \
  --target bob-demo/main/TODO-muban/1 \
  --target bob-demo/main/TODO-muban/2 \
  --due 2026-06-21 \
  --promise "I promise to complete TODO-muban subtasks 1 and 2."

GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger expire
```

What to say:

> Bob is the well-meaning actor who often lets people down.

> The point here is to show that expiration is not automatically the same thing
> as a broken promise. The system records `expired_unassessed` first.

> Assessment remains a separate step.

## Step 12: Contrast Mallory

Commands:

```bash
sed -n '1,40p' /home/jj/lab/commitment-ledger-demo/mallory-demo/TODO/TODO-falun-handle-malformed-packet-report.md
grep '"repo":"mallory-demo"' /home/jj/lab/commitment-ledger/data/work_items.jsonl
```

What to say:

> Mallory is not just unreliable. Mallory tries to confuse the system.

> This repo contains intentionally malformed or awkward detail structure so we
> can demonstrate that the tool should observe conservatively and keep judgment
> local.

## Closing Summary

What to say:

> The system starts from ordinary repos. It does not require teams to abandon
> their normal TODO files.

> It adds a second layer: explicit promises, evidence, and assessments.

> It also keeps exact artifact bytes under content-addressed storage, so the
> readable reports are not the only thing we have.

> Right now this is a local real-repo demo. The next future step would be richer
> peer exchange, richer trust accounting, and migration toward any future
> upstream-frozen Commitment Ledger protocol.

## Recovery Notes

If the demo gets noisy, these commands usually reset the audience’s
understanding:

```bash
GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger status
GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger report --promiser Alice
GOCACHE=/tmp/gocache go run ./cmd/commitment-ledger report --repo alice-demo --branch main
```
