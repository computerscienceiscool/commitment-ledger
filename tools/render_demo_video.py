#!/usr/bin/env python3

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from tempfile import TemporaryDirectory
from textwrap import wrap

from PIL import Image, ImageDraw, ImageFont


ROOT = Path(__file__).resolve().parent.parent
SOURCE_DEMO_REPOS = Path(
    os.environ.get("COMMITMENT_LEDGER_DEMO_REPOS", str(ROOT.parent / "commitment-ledger-demo"))
)
OUTPUT_VIDEO = ROOT / "docs" / "demo-video.mp4"
BUILD_ROOT = ROOT / ".demo-video-build"
FONT_PATH = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
WIDTH = 1440
HEIGHT = 900
FPS = 2
SECONDS_PER_STEP = 10
BG = "#0b1020"
PANEL = "#111827"
TERM_BG = "#0d1117"
TEXT = "#e5e7eb"
MUTED = "#9ca3af"
ACCENT = "#93c5fd"
GREEN = "#86efac"
YELLOW = "#fde68a"
PINK = "#f9a8d4"


@dataclass
class Step:
    title: str
    commands: list[str]
    say: list[str]
    output: str


def run(cmd: str, cwd: Path) -> str:
    result = subprocess.run(
        cmd,
        cwd=str(cwd),
        shell=True,
        text=True,
        capture_output=True,
        env={**os.environ, "GOCACHE": "/tmp/gocache"},
    )
    out = result.stdout
    err = result.stderr
    if result.returncode != 0:
        raise RuntimeError(f"command failed: {cmd}\nstdout:\n{out}\nstderr:\n{err}")
    return out.rstrip()


def write_json(path: Path, payload: object) -> None:
    path.write_text(json.dumps(payload, indent=2) + "\n")


def prepare_workspace() -> Path:
    if not SOURCE_DEMO_REPOS.exists():
        raise RuntimeError(
            f"demo repos not found at {SOURCE_DEMO_REPOS}; run 'make demo-setup' or set COMMITMENT_LEDGER_DEMO_REPOS"
        )
    if BUILD_ROOT.exists():
        shutil.rmtree(BUILD_ROOT)
    workspace = BUILD_ROOT / "workspace"
    shutil.copytree(
        ROOT,
        workspace,
        ignore=shutil.ignore_patterns(".git", "data", "records", "__pycache__", "*.pyc", ".demo-video-build"),
    )
    shutil.rmtree(workspace / "docs" / "__pycache__", ignore_errors=True)
    shutil.copytree(SOURCE_DEMO_REPOS, workspace / "demo-repos")
    (workspace / "data").mkdir(exist_ok=True)
    (workspace / "records" / "commitments").mkdir(parents=True, exist_ok=True)
    (workspace / "records" / "assessments").mkdir(parents=True, exist_ok=True)
    repos = {
        "repos": [
            {
                "name": "alice-demo",
                "provider": "local",
                "url": "",
                "local_path": str(workspace / "demo-repos" / "alice-demo"),
                "branch": "main",
                "todo_file": "TODO/TODO.md",
                "enabled": True,
            },
            {
                "name": "bob-demo",
                "provider": "local",
                "url": "",
                "local_path": str(workspace / "demo-repos" / "bob-demo"),
                "branch": "main",
                "todo_file": "TODO/TODO.md",
                "enabled": True,
            },
            {
                "name": "dave-demo",
                "provider": "local",
                "url": "",
                "local_path": str(workspace / "demo-repos" / "dave-demo"),
                "branch": "repair",
                "todo_file": "TODO/TODO.md",
                "enabled": True,
            },
            {
                "name": "mallory-demo",
                "provider": "local",
                "url": "",
                "local_path": str(workspace / "demo-repos" / "mallory-demo"),
                "branch": "jj",
                "todo_file": "TODO/TODO.md",
                "enabled": True,
            },
        ]
    }
    write_json(workspace / "config" / "repos.json", repos)
    return workspace


def make_demo_steps(workspace: Path) -> list[Step]:
    steps: list[Step] = []

    steps.append(
        Step(
            title="Commitment Ledger Demo",
            commands=[
                "Repo-first work",
                "Promise against discovered TODO target",
                "Evidence and later assessment",
                "Inspect and verify signed artifacts",
            ],
            say=[
                "This walkthrough stays inside seeded local repos and a local ledger workspace.",
                "Watch for four layers: source work, promise, evidence, and final judgment.",
                "Every terminal pane command is repo-relative to keep the flow copy-paste friendly.",
            ],
            output=(
                "Goal\n"
                "----\n"
                "Show the shortest path from ordinary TODO work to a verifiable signed assessment artifact.\n\n"
                "Flow\n"
                "----\n"
                "1. Observe repo work\n"
                "2. Create a commitment\n"
                "3. Capture evidence from repo changes\n"
                "4. Assess and verify the result"
            ),
        )
    )

    steps.append(
        Step(
            title="Step 1: Source Work In Alice Repo",
            commands=[
                "sed -n '1,40p' demo-repos/alice-demo/TODO/TODO.md",
                "sed -n '1,40p' demo-repos/alice-demo/TODO/TODO-ravud-ship-welcome-flow.md",
            ],
            say=[
                "This is just an ordinary repo with an ordinary TODO index and detail file.",
                "At this point there is work, but no promise yet.",
            ],
            output="\n\n".join(
                run(cmd, workspace) for cmd in [
                    "sed -n '1,40p' demo-repos/alice-demo/TODO/TODO.md",
                    "sed -n '1,40p' demo-repos/alice-demo/TODO/TODO-ravud-ship-welcome-flow.md",
                ]
            ),
        )
    )

    scan1 = run("go run ./cmd/commitment-ledger scan --config config/repos.json", workspace)
    steps.append(
        Step(
            title="Step 2: Scan Real Local Repos",
            commands=["go run ./cmd/commitment-ledger scan --config config/repos.json"],
            say=[
                "The ledger scans seeded demo repos on disk and discovers branch-qualified work.",
                "It does not hand-maintain a separate task list.",
            ],
            output=scan1,
        )
    )

    work_ravud = run("grep 'TODO-ravud' data/work_items.jsonl | tail -n 4", workspace)
    steps.append(
        Step(
            title="Step 3: Machine View Of Alice Work",
            commands=["grep 'TODO-ravud' data/work_items.jsonl | tail -n 4"],
            say=[
                "The projection layer turns repo files into branch-qualified work targets.",
                "Here we see the top-level TODO and the three subtasks.",
            ],
            output=work_ravud,
        )
    )

    commit_cmd = (
        "go run ./cmd/commitment-ledger commit "
        "--promiser Alice "
        "--repo alice-demo "
        "--branch main "
        "--target alice-demo/main/TODO-ravud/1 "
        "--due 2026-07-01 "
        "--promise \"I promise to complete TODO-ravud subtask 1.\""
    )
    commit_out = run(commit_cmd, workspace)
    commitment_id, commitment_cid = commit_out.split()
    steps.append(
        Step(
            title="Step 4: Create Alice Commitment",
            commands=[commit_cmd],
            say=[
                "Now the work becomes a promise.",
                "The command returns a local commitment ID and a content-addressed artifact CID.",
            ],
            output=commit_out,
        )
    )

    record_cmd = f"cat records/commitments/{commitment_id}.md"
    steps.append(
        Step(
            title="Step 5: Human-Readable Commitment Projection",
            commands=[record_cmd],
            say=[
                "The Markdown record is a projection, not the primary artifact.",
                "It still carries the Artifact CID and Protocol pCID.",
            ],
            output=run(record_cmd, workspace),
        )
    )

    artifact_cmd = f"grep '{commitment_id}' data/artifacts.jsonl"
    steps.append(
        Step(
            title="Step 6: Artifact Index Row",
            commands=[artifact_cmd],
            say=[
                "This row indexes the raw artifact bytes in local CAS.",
                "It records who signed, which protocol doc owns the payload, and the related local ID.",
            ],
            output=run(artifact_cmd, workspace),
        )
    )

    inspect_cmd = f"go run ./cmd/commitment-ledger inspect --json {commitment_id}"
    steps.append(
        Step(
            title="Step 7: Operator Inspect View",
            commands=[inspect_cmd],
            say=[
                "Inspect is the operator-facing lookup view over the commitment artifact.",
                "It resolves the local record, signer, protocol doc, and projected status in one place.",
            ],
            output=run(inspect_cmd, workspace),
        )
    )

    check_cmd = (
        "python3 - <<'PY'\n"
        "from pathlib import Path\n"
        "path = Path('demo-repos/alice-demo/TODO/TODO-ravud-ship-welcome-flow.md')\n"
        "text = path.read_text()\n"
        "text = text.replace('- [ ] 1. Add route', '- [x] 1. Add route')\n"
        "path.write_text(text)\n"
        "PY\n"
        "git -C demo-repos/alice-demo add TODO/TODO-ravud-ship-welcome-flow.md\n"
        "git -C demo-repos/alice-demo -c user.name=Alice -c user.email=alice@example.com commit -m \"Complete Alice subtask 1\""
    )
    check_out = run(check_cmd, workspace)
    steps.append(
        Step(
            title="Step 8: Alice Actually Does The Work",
            commands=check_cmd.splitlines(),
            say=[
                "The source evidence lives in the work repo, not only in the ledger repo.",
                "We are changing the real subtask state and committing it normally.",
            ],
            output=check_out,
        )
    )

    scan2 = run("go run ./cmd/commitment-ledger scan --config config/repos.json", workspace)
    steps.append(
        Step(
            title="Step 9: Scan Again To Capture Evidence",
            commands=["go run ./cmd/commitment-ledger scan --config config/repos.json"],
            say=[
                "The second scan sees that the promised subtask is now checked.",
                "That creates evidence, but not yet final judgment.",
            ],
            output=scan2,
        )
    )

    evidence_cmd = f"grep '{commitment_id}' data/evidence.jsonl"
    evidence_out = run(evidence_cmd, workspace)
    todo_checked_line = ""
    for line in evidence_out.splitlines():
        if '"evidence_type":"todo_checked"' in line:
            todo_checked_line = line
            break
    if not todo_checked_line:
        raise RuntimeError("expected todo_checked evidence for Alice commitment")
    evidence_id = todo_checked_line.split('"evidence_id":"', 1)[1].split('"', 1)[0]
    steps.append(
        Step(
            title="Step 10: Evidence Row",
            commands=[evidence_cmd],
            say=[
                "This shows the commitment-linked evidence rows.",
                "The todo_checked row is the basis for later assessment.",
            ],
            output=evidence_out,
        )
    )

    assess_cmd = (
        "go run ./cmd/commitment-ledger assess "
        f"--commitment {commitment_id} "
        "--assessor Alice "
        "--status kept "
        f"--basis {evidence_id} "
        "--notes \"Completed before the due date.\""
    )
    assess_out = run(assess_cmd, workspace)
    assessment_id = assess_out.split()[0]
    steps.append(
        Step(
            title="Step 11: Assess The Promise As Kept",
            commands=[assess_cmd],
            say=[
                "Checked state is evidence; assessment is a separate explicit act.",
                "That keeps local judgment distinct from raw observation.",
            ],
            output=assess_out,
        )
    )

    verify_cmd = f"go run ./cmd/commitment-ledger verify --json {assessment_id}"
    steps.append(
        Step(
            title="Step 12: Verify The Assessment Artifact",
            commands=[verify_cmd],
            say=[
                "Verify is the integrity check over local CAS bytes, signer material, and protocol linkage.",
                "It tells the operator whether the final assessment artifact is structurally and cryptographically consistent.",
            ],
            output=run(verify_cmd, workspace),
        )
    )

    assessment_record_cmd = f"cat records/assessments/{assessment_id}.md"
    report_cmd = "go run ./cmd/commitment-ledger report --promiser Alice"
    steps.append(
        Step(
            title="Step 13: Final Human View",
            commands=[assessment_record_cmd, report_cmd],
            say=[
                "Now we can see the full chain: work, promise, evidence, and assessment.",
                "That is the PromiseGrid-shaped lifecycle this tool is trying to make visible.",
            ],
            output="\n\n".join([run(assessment_record_cmd, workspace), run(report_cmd, workspace)]),
        )
    )

    steps.append(
        Step(
            title="Step 14: Contrast Bob And Mallory",
            commands=[
                "sed -n '1,30p' demo-repos/mallory-demo/TODO/TODO-falun-handle-malformed-packet-report.md",
                "go run ./cmd/commitment-ledger report --repo bob-demo --branch main",
            ],
            say=[
                "Bob is the well-meaning but unreliable contrast.",
                "Mallory is the adversarial contrast with malformed or confusing input.",
            ],
            output="\n\n".join(
                [
                    run("sed -n '1,30p' demo-repos/mallory-demo/TODO/TODO-falun-handle-malformed-packet-report.md", workspace),
                    run("go run ./cmd/commitment-ledger report --repo bob-demo --branch main", workspace),
                ]
            ),
        )
    )

    return steps


def render_text(draw: ImageDraw.ImageDraw, text: str, xy: tuple[int, int], font: ImageFont.FreeTypeFont, fill: str, max_width: int) -> int:
    x, y = xy
    line_height = font.size + 8
    for raw_line in text.splitlines() or [""]:
        wrapped = wrap(raw_line, width=max(10, max_width // (font.size // 2 + 6))) or [""]
        for line in wrapped:
            draw.text((x, y), line, font=font, fill=fill)
            y += line_height
    return y


def is_shell_command(text: str) -> bool:
    prefixes = ("./", "go ", "make ", "sed ", "grep ", "cat ", "git ", "python3 ")
    return text.startswith(prefixes)


def draw_step(step: Step, index: int, total: int, frame_path: Path) -> None:
    img = Image.new("RGB", (WIDTH, HEIGHT), BG)
    draw = ImageDraw.Draw(img)
    title_font = ImageFont.truetype(FONT_PATH, 34)
    mono_font = ImageFont.truetype(FONT_PATH, 24)
    small_font = ImageFont.truetype(FONT_PATH, 20)

    draw.rounded_rectangle((28, 24, WIDTH - 28, 92), radius=18, fill=PANEL)
    draw.text((52, 42), f"{step.title}", font=title_font, fill=TEXT)
    draw.text((WIDTH - 200, 46), f"{index + 1}/{total}", font=small_font, fill=MUTED)

    term_box = (28, 112, 980, HEIGHT - 28)
    note_box = (1004, 112, WIDTH - 28, HEIGHT - 28)
    draw.rounded_rectangle(term_box, radius=18, fill=TERM_BG)
    draw.rounded_rectangle(note_box, radius=18, fill=PANEL)

    draw.text((50, 132), "$ Demo Terminal", font=small_font, fill=ACCENT)
    draw.text((1028, 132), "Narration and callouts", font=small_font, fill=ACCENT)

    y = 168
    for command in step.commands:
        prefix = "$ " if is_shell_command(command) else "• "
        y = render_text(draw, f"{prefix}{command}", (50, y), mono_font, GREEN if prefix == "$ " else PINK, term_box[2] - term_box[0] - 40)
        y += 4
    y += 6
    render_text(draw, step.output, (50, y), mono_font, TEXT, term_box[2] - term_box[0] - 40)

    note_y = 176
    for item in step.say:
        note_y = render_text(draw, f"- {item}", (1028, note_y), small_font, YELLOW, note_box[2] - note_box[0] - 36)
        note_y += 10

    img.save(frame_path)


def encode_video(frames_dir: Path, output_path: Path) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    with TemporaryDirectory(prefix="commitment-ledger-ffmpeg-", dir="/tmp") as runtime_dir:
        cmd = [
            "ffmpeg",
            "-y",
            "-framerate",
            str(FPS),
            "-start_number",
            "1",
            "-i",
            str(frames_dir / "frame-%04d.png"),
            "-c:v",
            "libx264",
            "-pix_fmt",
            "yuv420p",
            str(output_path),
        ]
        env = {**os.environ, "XDG_RUNTIME_DIR": runtime_dir}
        subprocess.run(cmd, check=True, env=env)


def main() -> int:
    workspace = prepare_workspace()
    steps = make_demo_steps(workspace)
    frames_dir = workspace / "frames"
    frames_dir.mkdir(exist_ok=True)

    frame_no = 1
    for index, step in enumerate(steps):
        for _ in range(FPS * SECONDS_PER_STEP):
            draw_step(step, index, len(steps), frames_dir / f"frame-{frame_no:04d}.png")
            frame_no += 1

    encode_video(frames_dir, OUTPUT_VIDEO)
    print(OUTPUT_VIDEO)
    return 0


if __name__ == "__main__":
    sys.exit(main())
