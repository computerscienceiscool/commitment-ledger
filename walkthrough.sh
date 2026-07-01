#!/usr/bin/env bash
#
# walkthrough.sh — a narrated, self-contained tour of Commitment Ledger.
#
# WHAT THIS DOES
#   It drives the whole lifecycle the tool is built around, end to end, and
#   explains each step out loud as it runs:
#
#       WORK  ->  PROMISE  ->  (do the work)  ->  EVIDENCE  ->  ASSESSMENT
#                                                                   |
#                                                    VERIFY + REPORT + STORAGE TOUR
#
#   In plain terms: the ledger observes an ordinary TODO-driven git repo, you
#   record a *promise* against a piece of that work, the promiser actually does
#   the work, a re-scan turns that into *evidence*, you record a final
#   *assessment*, and then we verify the signed artifact bytes and tour the
#   content-addressed storage (CAS) that this branch is all about.
#
# WHY IT IS SAFE TO RUN
#   Everything happens inside a throwaway SANDBOX directory. It does NOT touch
#   your real data/, records/, config/identities/, or the repos under
#   ~/lab/commitment-ledger-demo. Run it as many times as you like; each run
#   starts from a clean slate. The binary is built fresh into the sandbox, so it
#   doesn't even write to the repo's bin/.
#
# HOW TO RUN
#   ./walkthrough.sh              # interactive: pauses between steps (press Enter)
#   AUTO=1 ./walkthrough.sh       # run straight through with no pauses
#   SANDBOX=/some/dir ./walkthrough.sh   # choose where the sandbox lives
#
#   After it finishes, it prints the sandbox path so you can `cd` in and poke
#   around at the files it created.
#
# ---------------------------------------------------------------------------

set -uo pipefail

# --- Configuration ----------------------------------------------------------
# REPO is the directory this script lives in (the app repo root), so the script
# works no matter where you invoke it from.
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# The sandbox is a disposable scratch area. Everything the demo writes lives here.
SANDBOX="${SANDBOX:-${TMPDIR:-/tmp}/commitment-ledger-walkthrough}"

# The built binary and the fake "work" repo we'll observe.
BIN="$SANDBOX/commitment-ledger"
DEMO_REPO="$SANDBOX/demo-repo"

# Keep Go's build cache out of the way (matches what the Makefile does).
export GOCACHE="${GOCACHE:-/tmp/gocache}"

# A due date safely in the future (relative to the real clock the app reads),
# so the commitment shows up as "open" rather than "expired".
DUE="$(date -d '+14 days' +%F 2>/dev/null || echo 2026-07-15)"

# --- Presentation helpers ---------------------------------------------------
# Colors, but only when stdout is a real terminal (so piping to a file stays clean).
if [[ -t 1 ]] && command -v tput >/dev/null 2>&1 && [[ -n "$(tput colors 2>/dev/null || echo 0)" ]]; then
  BOLD="$(tput bold)"; DIM="$(tput dim)"; RESET="$(tput sgr0)"
  RED="$(tput setaf 1)"; GREEN="$(tput setaf 2)"; YELLOW="$(tput setaf 3)"
  BLUE="$(tput setaf 4)"; CYAN="$(tput setaf 6)"
else
  BOLD=""; DIM=""; RESET=""; RED=""; GREEN=""; YELLOW=""; BLUE=""; CYAN=""
fi

# banner: a big labeled section header, so you always know which ACT you're in.
banner() {
  echo
  echo "${BOLD}${BLUE}============================================================${RESET}"
  echo "${BOLD}${BLUE}  $*${RESET}"
  echo "${BOLD}${BLUE}============================================================${RESET}"
}

# explain: a plain-language narration line describing what's about to happen.
explain() { echo "${YELLOW}» $*${RESET}"; }

# run: print the exact command (so you can see what's being executed), run it,
# and abort loudly if it fails. Use this for the commands that matter.
run() {
  echo "${CYAN}\$ ${*}${RESET}"
  "$@"
  local rc=$?
  if (( rc != 0 )); then
    echo "${RED}✗ command failed (exit $rc). Stopping.${RESET}" >&2
    exit "$rc"
  fi
}

# peek: a display-only helper for `cat`/`grep`/etc. It shows the command dimly,
# never aborts the script (viewing state should never kill the tour), and is
# meant for pipelines we run via `bash -c`.
peek() {
  echo "${DIM}\$ ${1}${RESET}"
  bash -c "$1" || true
}

# pause: wait for Enter between steps when interactive. Skipped with AUTO=1 or
# when there's no terminal (e.g. output is being piped).
pause() {
  if [[ -t 0 && "${AUTO:-0}" != "1" ]]; then
    read -rp "${DIM}   … press Enter to continue …${RESET}" _ || true
  fi
}

# --- Preflight: tools we depend on ------------------------------------------
for tool in go git python3; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "${RED}This walkthrough needs '$tool' on your PATH. Please install it first.${RESET}" >&2
    exit 1
  fi
done

banner "SETUP — build the tool and create a disposable sandbox"

# Start every run from a clean slate so the demo is repeatable.
explain "Wiping any previous sandbox at: $SANDBOX"
rm -rf "$SANDBOX"
mkdir -p "$SANDBOX"

# The ledger always reads config/ + docs/protocols/ and writes data/ + records/
# RELATIVE TO ITS CURRENT DIRECTORY (root is hardcoded to "."). So if we run the
# binary with the sandbox as the working directory, all of its state lands here
# and never mixes with your real ledger.
explain "The tool resolves all state relative to its working directory, so we run it inside the sandbox."

# Protocol docs are frozen by exact bytes to define pCIDs, so the tool needs a
# copy of them at <root>/docs/protocols. Identities, by contrast, are generated
# automatically on first use, so we don't need to copy any keys.
explain "Copying docs/protocols/ into the sandbox (their exact bytes define the protocol pCIDs)."
mkdir -p "$SANDBOX/docs" "$SANDBOX/config"
cp -R "$REPO/docs/protocols" "$SANDBOX/docs/protocols"

# Build a fresh binary straight into the sandbox (leaves the repo's bin/ alone).
explain "Building the commitment-ledger binary (first build may take a moment)…"
run bash -c "cd '$REPO' && go build -o '$BIN' ./cmd/commitment-ledger"

# From here on, operate from inside the sandbox.
cd "$SANDBOX"
pause

# ---------------------------------------------------------------------------
banner "SETUP — seed a fake 'work' repo with an ordinary TODO list"

# The ledger observes normal git repos that happen to keep a TODO index. We make
# one here so the demo is self-contained (no dependency on ~/lab/...-demo repos).
explain "Creating a normal git repo with a TODO index and a detail file — this stands in for a real project."
mkdir -p "$DEMO_REPO/TODO"
git -C "$DEMO_REPO" init -b main -q
git -C "$DEMO_REPO" config user.name  "Alice"
git -C "$DEMO_REPO" config user.email "alice@example.com"

# The top-level TODO index: one work item that links to a detail file.
cat > "$DEMO_REPO/TODO/TODO.md" <<'EOF'
# TODO Index

- [ ] TODO-ravud - Ship welcome flow (`TODO/TODO-ravud-ship-welcome-flow.md`)
EOF

# The detail file: three concrete subtasks under that work item.
cat > "$DEMO_REPO/TODO/TODO-ravud-ship-welcome-flow.md" <<'EOF'
# TODO-ravud: Ship welcome flow

- [ ] 1. Add route
- [ ] 2. Add tests
- [ ] 3. Add docs
EOF

git -C "$DEMO_REPO" add TODO
git -C "$DEMO_REPO" commit -q -m "Seed demo TODOs"
explain "Here is the work the way a human sees it — nothing has been promised yet:"
peek "cat '$DEMO_REPO/TODO/TODO.md' '$DEMO_REPO/TODO/TODO-ravud-ship-welcome-flow.md'"

# Point a repo-config at that sandbox repo. `scan` insists the repo is on the
# branch named here, which is why we init'd it on main.
cat > "$SANDBOX/config/repos.json" <<EOF
{
  "repos": [
    {
      "name": "demo",
      "provider": "local",
      "url": "",
      "local_path": "$DEMO_REPO",
      "branch": "main",
      "todo_file": "TODO/TODO.md",
      "enabled": true
    }
  ]
}
EOF
pause

# ---------------------------------------------------------------------------
banner "ACT 1 — WORK: scan the repo to discover work items"

explain "The ledger reads the repo's TODO files and turns them into branch-qualified work items."
explain "It never edits the source repo — it only observes it."
run "$BIN" scan --config config/repos.json

echo
explain "That discovery is projected into data/work_items.jsonl. Notice the targets are branch-qualified"
explain "(e.g. demo/main/TODO-ravud/1) — there is no pretense of one global state independent of branch:"
peek "python3 -c \"import json;[print(' -', json.loads(l)['repo']+'/'+json.loads(l)['branch']+'/'+json.loads(l)['work_id']) for l in open('data/work_items.jsonl') if l.strip()]\""
pause

# ---------------------------------------------------------------------------
banner "ACT 2 — PROMISE: Alice commits to a specific subtask"

# This is the first real promise. Before this line there was only *work*; now
# there's a signed commitment artifact recorded against one specific target.
TARGET="demo/main/TODO-ravud/1"
explain "Alice promises to finish subtask 1 (\"Add route\"), due $DUE."
explain "The command returns two ids: a human-facing COMMITMENT id and the content-addressed CID of the signed artifact bytes."

echo "${CYAN}\$ $BIN commit --promiser Alice --repo demo --branch main --target $TARGET --due $DUE --promise \"I promise to complete TODO-ravud subtask 1.\"${RESET}"
commit_out="$("$BIN" commit \
  --promiser Alice \
  --repo demo \
  --branch main \
  --target "$TARGET" \
  --due "$DUE" \
  --promise "I promise to complete TODO-ravud subtask 1.")"
rc=$?
if (( rc != 0 )); then echo "${RED}✗ commit failed${RESET}" >&2; exit "$rc"; fi
echo "$commit_out"

# commit prints "COMMITMENT-... <artifact_cid>" — split it into two variables.
COMMITMENT_ID="${commit_out%% *}"
ARTIFACT_CID="${commit_out##* }"
echo
explain "Captured commitment id: ${BOLD}$COMMITMENT_ID${RESET}${YELLOW}"
explain "Captured artifact CID:  ${BOLD}$ARTIFACT_CID${RESET}"
pause

echo
explain "The human-readable projection of that promise (records/…) still points back to the exact bytes and protocol:"
peek "cat 'records/commitments/$COMMITMENT_ID.md'"
echo
explain "And the local artifact index row tells us where the bytes live, who signed, and the payload/proof CIDs:"
peek "grep '$COMMITMENT_ID' data/artifacts.jsonl | python3 -m json.tool"
pause

echo
explain "The signed artifact bytes themselves live in content-addressed storage under data/cas/ — addressed by that CID:"
peek "find data/cas -type f | head"
echo
explain "'inspect' is the operator lookup: it ties the commitment id back to the artifact, protocol, signer, and current state:"
run "$BIN" inspect --json "$COMMITMENT_ID"
pause

# ---------------------------------------------------------------------------
banner "ACT 3 — EVIDENCE: Alice actually does the work, then we re-scan"

# The crucial design point: evidence comes from real changes in the *source*
# repo, not from anything typed into the ledger. So Alice checks the box and
# commits it like ordinary work.
explain "Alice ticks subtask 1's checkbox in the source repo and commits it — exactly like normal project work."
python3 - "$DEMO_REPO/TODO/TODO-ravud-ship-welcome-flow.md" <<'PY'
import sys
p = sys.argv[1]
text = open(p).read().replace('- [ ] 1. Add route', '- [x] 1. Add route')
open(p, 'w').write(text)
PY
git -C "$DEMO_REPO" add TODO/TODO-ravud-ship-welcome-flow.md
git -C "$DEMO_REPO" commit -q -m "Complete subtask 1: add route"
explain "Subtask 1 is now checked in the source repo:"
peek "grep '1. Add route' '$DEMO_REPO/TODO/TODO-ravud-ship-welcome-flow.md'"
pause

echo
explain "A second scan notices the promised box flipped, and records that observation as EVIDENCE (not yet a verdict):"
run "$BIN" scan --config config/repos.json
echo
explain "Here is the scan-derived evidence for our commitment (evidence_type = todo_checked):"
peek "python3 -c \"import json;[print(' -', json.loads(l)['evidence_id'], json.loads(l)['evidence_type'], json.loads(l).get('target','')) for l in open('data/evidence.jsonl') if l.strip() and json.loads(l).get('commitment_id')=='$COMMITMENT_ID']\""

# Grab the todo_checked evidence id to cite as the basis for the assessment.
EVIDENCE_ID="$(python3 - "$COMMITMENT_ID" <<'PY'
import json, sys
cid = sys.argv[1]
best = ""
for line in open('data/evidence.jsonl'):
    line = line.strip()
    if not line:
        continue
    d = json.loads(line)
    if d.get('commitment_id') == cid and d.get('evidence_type') == 'todo_checked':
        best = d['evidence_id']   # keep the last (newest) match
print(best)
PY
)"
explain "Citing evidence: ${BOLD}${EVIDENCE_ID:-<none found>}${RESET}"
pause

# ---------------------------------------------------------------------------
banner "ACT 4 — ASSESSMENT: record the final verdict (a separate, deliberate act)"

# Evidence is observable; the assessment is a distinct decision. The tool refuses
# to pretend that "the box is checked" is automatically the same as "kept".
explain "We now record the terminal outcome as 'kept', citing the evidence above."
explain "'kept' is validated against the latest scanned work state — you can't claim kept if the work isn't actually done."
if [[ -n "$EVIDENCE_ID" ]]; then
  run "$BIN" assess --commitment "$COMMITMENT_ID" --assessor Alice --status kept --basis "$EVIDENCE_ID" --notes "Completed before the due date."
else
  # Fallback: still assess even if we couldn't auto-detect the evidence id.
  explain "(No todo_checked evidence id captured; assessing without an explicit basis.)"
  run "$BIN" assess --commitment "$COMMITMENT_ID" --assessor Alice --status kept --notes "Completed before the due date."
fi

# Find the assessment id we just wrote, to show its record.
ASSESSMENT_ID="$(python3 -c "
import json
last=''
for l in open('data/assessments.jsonl'):
    l=l.strip()
    if l: last=json.loads(l)['assessment_id']
print(last)
" 2>/dev/null || echo "")"
echo
explain "The human-readable assessment record:"
peek "cat 'records/assessments/$ASSESSMENT_ID.md'"
pause

# ---------------------------------------------------------------------------
banner "ACT 5 — VERIFY + REPORT: prove integrity and see the whole chain"

# verify is a *cryptographic/structural* check, not a moral one. It confirms the
# stored artifact's bytes, signature, signer identity, and protocol linkage are
# internally consistent.
explain "'verify' checks the stored artifact: CAS bytes, envelope/payload/proof CIDs, signature, signer identity, and protocol match."
explain "It does NOT judge whether Alice was 'good' — only whether the artifact is internally and cryptographically consistent."
run "$BIN" verify --json "$ASSESSMENT_ID"
pause

echo
explain "Repo-level status — kept vs non-kept outcomes are surfaced separately:"
run "$BIN" status
echo
explain "And the full story for Alice: work -> promise -> evidence -> assessment, all in one report:"
run "$BIN" report --promiser Alice
pause

# ---------------------------------------------------------------------------
banner "ACT 6 — STORAGE TOUR: the CAS-first layout this branch is about"

# This branch moves the tool toward CAS-first storage: the signed artifact bytes
# are the durable source of truth, while refs/indexes are a rebuildable operator
# layer and the Markdown/JSONL files are disposable projections.
explain "Durable source of truth = immutable signed artifacts in content-addressed storage (data/cas/):"
peek "find data/cas -type f | sort"
echo
explain "Local refs name the current 'heads' / reference sets (a step toward PromiseGrid-native reference sets):"
peek "find data/refs -type f | sort"
echo
explain "Indexes are rebuildable local acceleration, grouped by state family:"
peek "find data/indexes -type f | sort"
pause

echo
explain "'doctor' checks that the projections/refs/indexes stay coherent with the CAS bytes — the integrity story this branch tightens:"
run "$BIN" doctor --repairable

# ---------------------------------------------------------------------------
banner "DONE"
echo "${GREEN}You just watched the full lifecycle:${RESET}"
echo "  ${GREEN}WORK → PROMISE → EVIDENCE → ASSESSMENT → VERIFY, backed by content-addressed artifacts.${RESET}"
echo
echo "Everything the demo created is in the sandbox — explore it freely:"
echo "  ${BOLD}$SANDBOX${RESET}"
echo "    demo-repo/            the observed 'work' git repo"
echo "    data/cas/             the signed artifact bytes (source of truth)"
echo "    data/refs, indexes/   the operator/reference layer (rebuildable)"
echo "    data/*.jsonl          machine-readable projections"
echo "    records/              human-readable Markdown projections"
echo
echo "Re-run any time (it always starts clean). Try:  ${BOLD}AUTO=1 ./walkthrough.sh${RESET}  to skip the pauses."
echo "Poke at the live tool yourself, e.g.:"
echo "  ${DIM}(cd '$SANDBOX' && '$BIN' status)${RESET}"
echo "  ${DIM}(cd '$SANDBOX' && '$BIN' report --promiser Alice)${RESET}"
