package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"commitment-ledger/internal/config"
	"commitment-ledger/internal/ledger"
	"commitment-ledger/internal/model"
	"commitment-ledger/internal/protocol"
)

func TestResolveBasisMapsAndValidatesCommitmentEvidence(t *testing.T) {
	evidenceItems := []model.Evidence{
		{
			EvidenceID:   "EVIDENCE-20260623-001",
			ArtifactCID:  "bafy-e1",
			CommitmentID: "COMMITMENT-1",
		},
		{
			EvidenceID:   "EVIDENCE-20260623-002",
			ArtifactCID:  "bafy-e2",
			CommitmentID: "COMMITMENT-2",
		},
	}

	got, err := resolveBasis([]string{"EVIDENCE-20260623-001", "bafy-e1"}, evidenceItems, "COMMITMENT-1")
	if err != nil {
		t.Fatalf("resolveBasis: %v", err)
	}
	if len(got) != 1 || got[0] != "bafy-e1" {
		t.Fatalf("resolved basis = %#v, want [\"bafy-e1\"]", got)
	}

	_, err = resolveBasis([]string{"EVIDENCE-20260623-002"}, evidenceItems, "COMMITMENT-1")
	if err == nil || !strings.Contains(err.Error(), "belongs to commitment") {
		t.Fatalf("foreign evidence error = %v, want commitment mismatch", err)
	}

	_, err = resolveBasis([]string{"does-not-exist"}, evidenceItems, "COMMITMENT-1")
	if err == nil || !strings.Contains(err.Error(), "unknown basis reference") {
		t.Fatalf("unknown basis error = %v, want unknown basis reference", err)
	}
}

func TestValidateEvidenceInputRejectsMismatches(t *testing.T) {
	store := ledger.NewStore(t.TempDir())
	workItems := []model.WorkItem{
		{Repo: "repo", Branch: "main", WorkID: "TODO-ravud", Status: model.StatusOpen},
		{Repo: "repo", Branch: "main", WorkID: "TODO-ravud/1", ParentWork: "TODO-ravud", Status: model.StatusOpen, IsSubtask: true},
	}
	if err := store.AppendWorkItems(workItems); err != nil {
		t.Fatalf("append work items: %v", err)
	}

	current := model.Commitment{
		CommitmentID: "COMMITMENT-1",
		Repo:         "repo",
		Branch:       "main",
		Targets:      []string{"repo/main/TODO-ravud"},
	}

	repo, branch, err := validateEvidenceInput(store, current, "", "", "repo/main/TODO-ravud/1")
	if err != nil {
		t.Fatalf("validateEvidenceInput accepted descendant target: %v", err)
	}
	if repo != "repo" || branch != "main" {
		t.Fatalf("repo/branch = %s/%s, want repo/main", repo, branch)
	}

	_, _, err = validateEvidenceInput(store, current, "other", "main", "")
	if err == nil || !strings.Contains(err.Error(), "must match commitment") {
		t.Fatalf("repo mismatch error = %v, want repo/branch mismatch", err)
	}

	_, _, err = validateEvidenceInput(store, current, "", "", "repo/main/TODO-other")
	if err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("unknown target error = %v, want unknown target", err)
	}
}

func TestLifecycleFlowUsesV2EvidenceAndAssessmentProtocols(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}

	repoPath := filepath.Join(root, "fixture-repo")
	writeFixtureRepo(t, repoPath, false)
	gitCommitAll(t, repoPath, "Initial TODO state")

	cfg := config.ReposConfig{
		Repos: []config.RepoSource{
			{
				Name:      "fixture",
				LocalPath: repoPath,
				Branch:    "main",
				TodoFile:  "TODO/TODO.md",
				Enabled:   true,
			},
		},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	scanTime := time.Date(2026, 6, 24, 10, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, scanTime, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan initial: %v", err)
	}

	if err := runCommit(root, store, registry, scanTime.Add(5*time.Minute), []string{
		"--promiser", "Alice",
		"--repo", "fixture",
		"--branch", "main",
		"--target", "fixture/main/TODO-ravud/1",
		"--due", "2026-07-01",
		"--promise", "I promise to complete subtask 1.",
	}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}

	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		t.Fatalf("LoadLatestCommitments: %v", err)
	}
	if len(commitments) != 1 {
		t.Fatalf("got %d commitments, want 1", len(commitments))
	}
	var current model.Commitment
	for _, item := range commitments {
		current = item
	}
	if current.ProtocolPCID != registry.MustPCID(protocol.CommitmentPromise) {
		t.Fatalf("commitment protocol pCID = %q, want promise v1 pCID %q", current.ProtocolPCID, registry.MustPCID(protocol.CommitmentPromise))
	}

	writeFixtureRepo(t, repoPath, true)
	gitCommitAll(t, repoPath, "Complete subtask 1")

	secondScan := scanTime.Add(2 * time.Hour)
	if err := runScan(root, store, registry, secondScan, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan second: %v", err)
	}

	evidenceItems, err := store.LoadEvidence()
	if err != nil {
		t.Fatalf("LoadEvidence: %v", err)
	}
	var checkedEvidence model.Evidence
	for _, item := range evidenceItems {
		if item.CommitmentID == current.CommitmentID && item.EvidenceType == model.EvidenceTypeTodoChecked {
			checkedEvidence = item
			break
		}
	}
	if checkedEvidence.EvidenceID == "" {
		t.Fatal("expected todo_checked evidence for the commitment")
	}
	if checkedEvidence.ProtocolPCID != registry.MustPCID(protocol.CommitmentEvidence) {
		t.Fatalf("evidence protocol pCID = %q, want evidence v2 pCID %q", checkedEvidence.ProtocolPCID, registry.MustPCID(protocol.CommitmentEvidence))
	}

	if err := runAssess(root, store, registry, secondScan.Add(10*time.Minute), []string{
		"--commitment", current.CommitmentID,
		"--assessor", "Alice",
		"--status", model.StatusKept,
		"--basis", checkedEvidence.EvidenceID,
		"--notes", "Completed before the due date.",
	}); err != nil {
		t.Fatalf("runAssess: %v", err)
	}

	assessments, err := store.LoadAssessments()
	if err != nil {
		t.Fatalf("LoadAssessments: %v", err)
	}
	if len(assessments) != 1 {
		t.Fatalf("got %d assessments, want 1", len(assessments))
	}
	assessment := assessments[0]
	if assessment.ProtocolPCID != registry.MustPCID(protocol.CommitmentAssessment) {
		t.Fatalf("assessment protocol pCID = %q, want assessment v2 pCID %q", assessment.ProtocolPCID, registry.MustPCID(protocol.CommitmentAssessment))
	}
	if len(assessment.Basis) != 1 || assessment.Basis[0] != checkedEvidence.ArtifactCID {
		t.Fatalf("assessment basis = %#v, want [%q]", assessment.Basis, checkedEvidence.ArtifactCID)
	}

	reportOut := captureStdout(t, func() error {
		return runReport(store, []string{"--promiser", "Alice"})
	})
	if !strings.Contains(reportOut, "Promiser: Alice") || !strings.Contains(reportOut, "Kept: 1") {
		t.Fatalf("unexpected report output:\n%s", reportOut)
	}

	statusOut := captureStdout(t, func() error {
		return runStatus(store)
	})
	if !strings.Contains(statusOut, "Kept commitments: 1") || !strings.Contains(statusOut, "Broken: 0") {
		t.Fatalf("unexpected status output:\n%s", statusOut)
	}
}

func TestConformanceArtifactDistinguishesClaimedEmittedAndHistoricalProtocols(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}

	now := time.Date(2026, 6, 24, 15, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	payload := buildConformancePayload(registry, "v0.1.0", now)
	if len(payload.ClaimedProtocolPCIDs) != 6 {
		t.Fatalf("claimed_protocol_pcids len = %d, want 6", len(payload.ClaimedProtocolPCIDs))
	}
	if len(payload.EmittedProtocolPCIDs) != 4 {
		t.Fatalf("emitted_protocol_pcids len = %d, want 4", len(payload.EmittedProtocolPCIDs))
	}
	if len(payload.HistoricalProtocolPCIDs) != 2 {
		t.Fatalf("historical_protocol_pcids len = %d, want 2", len(payload.HistoricalProtocolPCIDs))
	}
	if payload.EmittedProtocolPCIDs[1] != registry.MustPCID(protocol.CommitmentEvidence) {
		t.Fatalf("emitted evidence pCID = %q, want v2 %q", payload.EmittedProtocolPCIDs[1], registry.MustPCID(protocol.CommitmentEvidence))
	}
	if payload.HistoricalProtocolPCIDs[0] != registry.MustPCID(protocol.CommitmentEvidenceV1) {
		t.Fatalf("historical evidence pCID = %q, want v1 %q", payload.HistoricalProtocolPCIDs[0], registry.MustPCID(protocol.CommitmentEvidenceV1))
	}

	artifactCID, err := emitConformanceArtifact(root, store, registry, "commitment-ledger", "v0.1.0", now)
	if err != nil {
		t.Fatalf("emitConformanceArtifact: %v", err)
	}

	artifacts, err := store.LoadArtifacts()
	if err != nil {
		t.Fatalf("LoadArtifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("got %d artifacts, want 1", len(artifacts))
	}
	if artifacts[0].ArtifactCID != artifactCID {
		t.Fatalf("artifact CID = %q, want %q", artifacts[0].ArtifactCID, artifactCID)
	}
}

func copyProtocolDocs(t *testing.T, root string) {
	t.Helper()
	repoRoot := repoRoot(t)
	sourceDir := filepath.Join(repoRoot, "docs", "protocols")
	destDir := filepath.Join(root, "docs", "protocols")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir protocol dir: %v", err)
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatalf("read protocol dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
		if err != nil {
			t.Fatalf("read protocol doc %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(destDir, entry.Name()), data, 0o644); err != nil {
			t.Fatalf("write protocol doc %s: %v", entry.Name(), err)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func writeConfig(t *testing.T, path string, cfg config.ReposConfig) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeFixtureRepo(t *testing.T, repoPath string, completed bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repoPath, "TODO"), 0o755); err != nil {
		t.Fatalf("mkdir TODO dir: %v", err)
	}
	index := "# TODO Index\n\n- [ ] TODO-ravud - Ship welcome flow (`TODO/TODO-ravud-ship-welcome-flow.md`)\n"
	subtask := "- [ ] 1. Add route\n"
	if completed {
		subtask = "- [x] 1. Add route\n"
	}
	detail := "# TODO-ravud: Ship welcome flow\n\n" + subtask + "- [ ] 2. Add docs\n"
	if err := os.WriteFile(filepath.Join(repoPath, "TODO", "TODO.md"), []byte(index), 0o644); err != nil {
		t.Fatalf("write TODO.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "TODO", "TODO-ravud-ship-welcome-flow.md"), []byte(detail), 0o644); err != nil {
		t.Fatalf("write detail file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		runGit(t, repoPath, "init", "-b", "main")
		runGit(t, repoPath, "config", "user.name", "Fixture User")
		runGit(t, repoPath, "config", "user.email", "fixture@example.com")
	} else if err != nil {
		t.Fatalf("stat .git: %v", err)
	}
}

func gitCommitAll(t *testing.T, repoPath string, message string) {
	t.Helper()
	runGit(t, repoPath, "add", "TODO/TODO.md", "TODO/TODO-ravud-ship-welcome-flow.md")
	runGit(t, repoPath, "commit", "-m", message)
}

func runGit(t *testing.T, repoPath string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = original
	data, readErr := io.ReadAll(r)
	_ = r.Close()
	if runErr != nil {
		t.Fatalf("captured function error: %v", runErr)
	}
	if readErr != nil {
		t.Fatalf("read captured stdout: %v", readErr)
	}
	return string(data)
}
