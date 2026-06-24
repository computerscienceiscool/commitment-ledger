package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"commitment-ledger/internal/changelog"
	"commitment-ledger/internal/config"
	"commitment-ledger/internal/exchange"
	"commitment-ledger/internal/grid"
	"commitment-ledger/internal/identity"
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

func TestValidateAssessmentAgainstWorkRequiresPromisedTargetsComplete(t *testing.T) {
	workItems := map[string]model.WorkItem{
		"repo/main/TODO-parent": {
			Repo:   "repo",
			Branch: "main",
			WorkID: "TODO-parent",
			Status: "open",
		},
		"repo/main/TODO-parent/1": {
			Repo:       "repo",
			Branch:     "main",
			WorkID:     "TODO-parent/1",
			ParentWork: "TODO-parent",
			Status:     "complete",
			IsSubtask:  true,
		},
		"repo/main/TODO-parent/2": {
			Repo:       "repo",
			Branch:     "main",
			WorkID:     "TODO-parent/2",
			ParentWork: "TODO-parent",
			Status:     "open",
			IsSubtask:  true,
		},
		"repo/main/TODO-leaf": {
			Repo:   "repo",
			Branch: "main",
			WorkID: "TODO-leaf",
			Status: "complete",
		},
	}

	err := validateAssessmentAgainstWork(model.Commitment{
		CommitmentID: "COMMITMENT-parent",
		Targets:      []string{"repo/main/TODO-parent"},
	}, model.StatusKept, workItems)
	if err == nil || !strings.Contains(err.Error(), "incomplete subtasks") {
		t.Fatalf("parent kept validation error = %v, want incomplete subtasks", err)
	}

	err = validateAssessmentAgainstWork(model.Commitment{
		CommitmentID: "COMMITMENT-subtask",
		Targets:      []string{"repo/main/TODO-parent/1"},
	}, model.StatusKept, workItems)
	if err != nil {
		t.Fatalf("subtask kept validation unexpectedly failed: %v", err)
	}

	err = validateAssessmentAgainstWork(model.Commitment{
		CommitmentID: "COMMITMENT-leaf",
		Targets:      []string{"repo/main/TODO-leaf"},
	}, model.StatusKept, workItems)
	if err != nil {
		t.Fatalf("leaf kept validation unexpectedly failed: %v", err)
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
	if err := persistProtocolSpecs(store, registry); err != nil {
		t.Fatalf("persistProtocolSpecs: %v", err)
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

	inspectCommitment := captureStdout(t, func() error {
		return runInspect(root, store, registry, []string{current.CommitmentID})
	})
	for _, fragment := range []string{
		"Reference: " + current.CommitmentID,
		"Kind: commitment_promise",
		"Current Status: kept",
		"Protocol: " + protocol.CommitmentPromise,
	} {
		if !strings.Contains(inspectCommitment, fragment) {
			t.Fatalf("commitment inspect after assessment missing %q:\n%s", fragment, inspectCommitment)
		}
	}

	inspectAssessment := captureStdout(t, func() error {
		return runInspect(root, store, registry, []string{assessment.AssessmentID})
	})
	for _, fragment := range []string{
		"Reference: " + assessment.AssessmentID,
		"Kind: commitment_assessment",
		"Assessment Status: kept",
		"Current Commitment Status: kept",
		"Protocol: " + protocol.CommitmentAssessment,
	} {
		if !strings.Contains(inspectAssessment, fragment) {
			t.Fatalf("assessment inspect output missing %q:\n%s", fragment, inspectAssessment)
		}
	}

	verifyCommitment := captureStdout(t, func() error {
		return runVerify(root, store, registry, []string{current.CommitmentID})
	})
	for _, fragment := range []string{
		"Kind: commitment_promise",
		"Envelope CID Verified: yes",
		"Signature Verified: yes",
		"Signer Identity Verified: yes",
		"Protocol: " + protocol.CommitmentPromise,
		"Local Protocol Match: yes",
	} {
		if !strings.Contains(verifyCommitment, fragment) {
			t.Fatalf("commitment verify output missing %q:\n%s", fragment, verifyCommitment)
		}
	}

	verifyEvidence := captureStdout(t, func() error {
		return runVerify(root, store, registry, []string{checkedEvidence.EvidenceID})
	})
	for _, fragment := range []string{
		"Kind: commitment_evidence",
		"Envelope CID Verified: yes",
		"Signature Verified: yes",
		"Signer Identity Verified: yes",
		"Protocol: " + protocol.CommitmentEvidence,
	} {
		if !strings.Contains(verifyEvidence, fragment) {
			t.Fatalf("evidence verify output missing %q:\n%s", fragment, verifyEvidence)
		}
	}

	verifyAssessment := captureStdout(t, func() error {
		return runVerify(root, store, registry, []string{assessment.AssessmentID})
	})
	for _, fragment := range []string{
		"Kind: commitment_assessment",
		"Envelope CID Verified: yes",
		"Signature Verified: yes",
		"Signer Identity Verified: yes",
		"Protocol: " + protocol.CommitmentAssessment,
	} {
		if !strings.Contains(verifyAssessment, fragment) {
			t.Fatalf("assessment verify output missing %q:\n%s", fragment, verifyAssessment)
		}
	}

	reportOut := captureStdout(t, func() error {
		return runReport(root, store, []string{"--promiser", "Alice"})
	})
	if !strings.Contains(reportOut, "Promiser: Alice") || !strings.Contains(reportOut, "Kept: 1") {
		t.Fatalf("unexpected report output:\n%s", reportOut)
	}

	statusOut := captureStdout(t, func() error {
		return runStatus(root, store, nil)
	})
	if !strings.Contains(statusOut, "Kept commitments: 1") || !strings.Contains(statusOut, "Broken: 0") {
		t.Fatalf("unexpected status output:\n%s", statusOut)
	}
}

func TestRunInspectResolvesIDsAndArtifactCIDs(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}
	if err := persistProtocolSpecs(store, registry); err != nil {
		t.Fatalf("persistProtocolSpecs: %v", err)
	}

	repoPath := filepath.Join(root, "fixture-repo")
	writeFixtureRepo(t, repoPath, false)
	gitCommitAll(t, repoPath, "Initial TODO state")

	cfg := config.ReposConfig{
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	now := time.Date(2026, 6, 24, 11, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan initial: %v", err)
	}
	if err := runCommit(root, store, registry, now.Add(time.Minute), []string{
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
	var current model.Commitment
	for _, item := range commitments {
		current = item
	}

	writeFixtureRepo(t, repoPath, true)
	gitCommitAll(t, repoPath, "Complete subtask 1")
	if err := runScan(root, store, registry, now.Add(2*time.Hour), []string{"--config", configPath}); err != nil {
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
		t.Fatal("expected todo_checked evidence")
	}

	inspectCommitment := captureStdout(t, func() error {
		return runInspect(root, store, registry, []string{current.CommitmentID})
	})
	for _, fragment := range []string{
		"Reference: " + current.CommitmentID,
		"Kind: commitment_promise",
		"Related ID: " + current.CommitmentID,
		"Protocol: " + protocol.CommitmentPromise,
		"Protocol Doc: " + filepath.Join(root, "docs", "protocols", protocol.CommitmentPromise+".md"),
		"Record Path: " + filepath.Join(root, "records", "commitments", current.CommitmentID+".md"),
		"Current Status: open",
	} {
		if !strings.Contains(inspectCommitment, fragment) {
			t.Fatalf("commitment inspect output missing %q:\n%s", fragment, inspectCommitment)
		}
	}

	inspectEvidence := captureStdout(t, func() error {
		return runInspect(root, store, registry, []string{checkedEvidence.EvidenceID})
	})
	for _, fragment := range []string{
		"Reference: " + checkedEvidence.EvidenceID,
		"Kind: commitment_evidence",
		"Related ID: " + checkedEvidence.EvidenceID,
		"Protocol: " + protocol.CommitmentEvidence,
		"Current Commitment Status: open",
		"Evidence Type: todo_checked",
		"Record Path: none (evidence stays in data/evidence.jsonl and commitment markdown)",
	} {
		if !strings.Contains(inspectEvidence, fragment) {
			t.Fatalf("evidence inspect output missing %q:\n%s", fragment, inspectEvidence)
		}
	}

	inspectArtifact := captureStdout(t, func() error {
		return runInspect(root, store, registry, []string{checkedEvidence.ArtifactCID})
	})
	for _, fragment := range []string{
		"Reference: " + checkedEvidence.ArtifactCID,
		"Kind: commitment_evidence",
		"Artifact CID: " + checkedEvidence.ArtifactCID,
		"Protocol pCID: " + checkedEvidence.ProtocolPCID,
	} {
		if !strings.Contains(inspectArtifact, fragment) {
			t.Fatalf("artifact inspect output missing %q:\n%s", fragment, inspectArtifact)
		}
	}
}

func TestRunInspectRejectsUnknownReference(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}

	err = runInspect(root, store, registry, []string{"does-not-exist"})
	if err == nil || !strings.Contains(err.Error(), "unknown inspect reference") {
		t.Fatalf("runInspect error = %v, want unknown inspect reference", err)
	}
}

func TestRunVerifyRejectsUnknownReference(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}

	err = runVerify(root, store, registry, []string{"does-not-exist"})
	if err == nil || !strings.Contains(err.Error(), "unknown verify reference") {
		t.Fatalf("runVerify error = %v, want unknown verify reference", err)
	}
}

func TestRunExportImportRoundTripWithSupport(t *testing.T) {
	exportRoot := t.TempDir()
	copyProtocolDocs(t, exportRoot)
	exportStore := ledger.NewStore(exportRoot)
	exportRegistry, err := protocol.Load(exportRoot)
	if err != nil {
		t.Fatalf("protocol.Load export: %v", err)
	}

	repoPath := filepath.Join(exportRoot, "fixture-repo")
	writeFixtureRepo(t, repoPath, false)
	gitCommitAll(t, repoPath, "Initial TODO state")
	cfg := config.ReposConfig{
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(exportRoot, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	now := time.Date(2026, 6, 24, 16, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(exportRoot, exportStore, exportRegistry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan initial: %v", err)
	}
	if err := runCommit(exportRoot, exportStore, exportRegistry, now.Add(time.Minute), []string{
		"--promiser", "Alice",
		"--repo", "fixture",
		"--branch", "main",
		"--target", "fixture/main/TODO-ravud/1",
		"--due", "2026-07-01",
		"--promise", "I promise to complete subtask 1.",
	}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	commitment := latestCommitment(t, exportStore)
	writeFixtureRepo(t, repoPath, true)
	gitCommitAll(t, repoPath, "Complete subtask 1")
	if err := runScan(exportRoot, exportStore, exportRegistry, now.Add(2*time.Hour), []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan second: %v", err)
	}
	evidenceItem := latestEvidenceByType(t, exportStore, model.EvidenceTypeTodoChecked)
	if err := runAssess(exportRoot, exportStore, exportRegistry, now.Add(3*time.Hour), []string{
		"--commitment", commitment.CommitmentID,
		"--assessor", "Alice",
		"--status", model.StatusKept,
		"--basis", evidenceItem.EvidenceID,
		"--notes", "Completed before the due date.",
	}); err != nil {
		t.Fatalf("runAssess: %v", err)
	}
	assessment := latestAssessment(t, exportStore)

	evidenceBundlePath := filepath.Join(exportRoot, "exports", "evidence.json")
	if err := runExport(exportRoot, exportStore, exportRegistry, now.Add(4*time.Hour), []string{"--out", evidenceBundlePath, evidenceItem.EvidenceID}); err != nil {
		t.Fatalf("runExport evidence: %v", err)
	}
	assessmentBundlePath := filepath.Join(exportRoot, "exports", "assessment.json")
	if err := runExport(exportRoot, exportStore, exportRegistry, now.Add(4*time.Hour), []string{"--out", assessmentBundlePath, assessment.AssessmentID}); err != nil {
		t.Fatalf("runExport assessment: %v", err)
	}

	importRoot := t.TempDir()
	copyProtocolDocs(t, importRoot)
	importStore := ledger.NewStore(importRoot)
	importRegistry, err := protocol.Load(importRoot)
	if err != nil {
		t.Fatalf("protocol.Load import: %v", err)
	}
	if err := runImport(importRoot, importStore, importRegistry, []string{"--in", evidenceBundlePath}); err != nil {
		t.Fatalf("runImport evidence: %v", err)
	}
	if err := runImport(importRoot, importStore, importRegistry, []string{"--in", assessmentBundlePath}); err != nil {
		t.Fatalf("runImport assessment: %v", err)
	}

	verifyEvidence := captureStdout(t, func() error {
		return runVerify(importRoot, importStore, importRegistry, []string{evidenceItem.EvidenceID})
	})
	if !strings.Contains(verifyEvidence, "Kind: commitment_evidence") ||
		!strings.Contains(verifyEvidence, "Signature Verified: yes") ||
		!strings.Contains(verifyEvidence, "Identity Source: imported support") {
		t.Fatalf("unexpected imported evidence verify output:\n%s", verifyEvidence)
	}

	verifyAssessment := captureStdout(t, func() error {
		return runVerify(importRoot, importStore, importRegistry, []string{assessment.AssessmentID})
	})
	if !strings.Contains(verifyAssessment, "Kind: commitment_assessment") ||
		!strings.Contains(verifyAssessment, "Protocol: "+protocol.CommitmentAssessment) ||
		!strings.Contains(verifyAssessment, "Latest Import Source: "+assessmentBundlePath) {
		t.Fatalf("unexpected imported assessment verify output:\n%s", verifyAssessment)
	}

	inspectAssessment := captureStdout(t, func() error {
		return runInspect(importRoot, importStore, importRegistry, []string{assessment.AssessmentID})
	})
	if !strings.Contains(inspectAssessment, "Current Commitment Status: kept") ||
		!strings.Contains(inspectAssessment, "Latest Import Source: "+assessmentBundlePath) {
		t.Fatalf("unexpected imported assessment inspect output:\n%s", inspectAssessment)
	}
}

func TestRunImportVerifyFailsWithoutSignerSupport(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	bundle := syntheticBundle(t, root, "external-protocol-v1", []byte("external protocol doc"), "Mallory")
	bundle.Signer = nil
	bundlePath := filepath.Join(root, "bundle.json")
	writeBundle(t, bundlePath, bundle)

	importRoot := t.TempDir()
	copyProtocolDocs(t, importRoot)
	importStore := ledger.NewStore(importRoot)
	importRegistry, err := protocol.Load(importRoot)
	if err != nil {
		t.Fatalf("protocol.Load import: %v", err)
	}
	if err := runImport(importRoot, importStore, importRegistry, []string{"--in", bundlePath, "--install-support=false"}); err != nil {
		t.Fatalf("runImport: %v", err)
	}

	err = runVerify(importRoot, importStore, importRegistry, []string{bundle.Artifact.ArtifactCID})
	if err == nil || !strings.Contains(err.Error(), "load signer identity") {
		t.Fatalf("runVerify error = %v, want missing signer identity", err)
	}
}

func TestRunImportVerifyUsesImportedProtocolSupportAndFailsOnMismatchedSigner(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	bundle := syntheticBundle(t, root, "external-protocol-v1", []byte("external protocol doc"), "Mallory")
	bundlePath := filepath.Join(root, "bundle.json")
	writeBundle(t, bundlePath, bundle)

	importRoot := t.TempDir()
	copyProtocolDocs(t, importRoot)
	importStore := ledger.NewStore(importRoot)
	importRegistry, err := protocol.Load(importRoot)
	if err != nil {
		t.Fatalf("protocol.Load import: %v", err)
	}
	if err := runImport(importRoot, importStore, importRegistry, []string{"--in", bundlePath}); err != nil {
		t.Fatalf("runImport: %v", err)
	}
	verifyOut := captureStdout(t, func() error {
		return runVerify(importRoot, importStore, importRegistry, []string{bundle.Artifact.ArtifactCID})
	})
	if !strings.Contains(verifyOut, "Local Protocol Match: yes") ||
		!strings.Contains(verifyOut, "Protocol: external-protocol-v1") ||
		!strings.Contains(verifyOut, "Protocol Source: imported support") {
		t.Fatalf("expected imported protocol support in verify output:\n%s", verifyOut)
	}

	bundle.Signer.PublicKey = base64.StdEncoding.EncodeToString([]byte("not-a-real-public-key"))
	badBundlePath := filepath.Join(root, "bundle-bad-signer.json")
	writeBundle(t, badBundlePath, bundle)

	badImportRoot := t.TempDir()
	copyProtocolDocs(t, badImportRoot)
	badImportStore := ledger.NewStore(badImportRoot)
	badImportRegistry, err := protocol.Load(badImportRoot)
	if err != nil {
		t.Fatalf("protocol.Load bad import: %v", err)
	}
	if err := runImport(badImportRoot, badImportStore, badImportRegistry, []string{"--in", badBundlePath}); err != nil {
		t.Fatalf("runImport bad signer: %v", err)
	}
	err = runVerify(badImportRoot, badImportStore, badImportRegistry, []string{bundle.Artifact.ArtifactCID})
	if err == nil || !strings.Contains(err.Error(), "local identity public key mismatch") {
		t.Fatalf("runVerify error = %v, want signer mismatch", err)
	}
}

func TestRunImportRejectsMismatchedProtocolSupport(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	bundle := syntheticBundle(t, root, "external-protocol-v1", []byte("external protocol doc"), "Mallory")
	bundle.Protocol.DocumentBytes = base64.StdEncoding.EncodeToString([]byte("tampered protocol doc"))
	bundlePath := filepath.Join(root, "bundle-bad-protocol.json")
	writeBundle(t, bundlePath, bundle)

	importRoot := t.TempDir()
	copyProtocolDocs(t, importRoot)
	importStore := ledger.NewStore(importRoot)
	importRegistry, err := protocol.Load(importRoot)
	if err != nil {
		t.Fatalf("protocol.Load import: %v", err)
	}
	err = runImport(importRoot, importStore, importRegistry, []string{"--in", bundlePath})
	if err == nil || !strings.Contains(err.Error(), "protocol support pCID mismatch") {
		t.Fatalf("runImport error = %v, want protocol support pCID mismatch", err)
	}
}

func TestRunImportRejectsConflictingCommitmentProjection(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	bundle := syntheticBundle(t, root, protocol.CommitmentPromise, []byte("external protocol doc"), "Mallory")
	bundle.Artifact.Kind = "commitment_promise"
	bundle.Artifact.RelatedID = "COMMITMENT-1"
	bundle.Commitment = &model.Commitment{
		CommitmentID: "COMMITMENT-1",
		Promiser:     "Alice",
		Repo:         "repo",
		Branch:       "main",
		Targets:      []string{"repo/main/TODO-ravud"},
		PromiseText:  "Imported promise",
		CreatedAt:    "2026-06-24T10:00:00-07:00",
		DueDate:      "2026-07-01",
		Status:       model.StatusOpen,
		ArtifactCID:  bundle.Artifact.ArtifactCID,
		ProtocolPCID: bundle.Artifact.ProtocolPCID,
	}
	bundlePath := filepath.Join(root, "bundle.json")
	writeBundle(t, bundlePath, bundle)

	importRoot := t.TempDir()
	copyProtocolDocs(t, importRoot)
	importStore := ledger.NewStore(importRoot)
	importRegistry, err := protocol.Load(importRoot)
	if err != nil {
		t.Fatalf("protocol.Load import: %v", err)
	}
	if err := importStore.AppendCommitment(model.Commitment{
		CommitmentID: "COMMITMENT-1",
		Promiser:     "Alice",
		Repo:         "repo",
		Branch:       "main",
		Targets:      []string{"repo/main/TODO-ravud"},
		PromiseText:  "Local conflicting promise",
		CreatedAt:    "2026-06-24T10:00:00-07:00",
		DueDate:      "2026-07-01",
		Status:       model.StatusOpen,
	}); err != nil {
		t.Fatalf("AppendCommitment: %v", err)
	}

	err = runImport(importRoot, importStore, importRegistry, []string{"--in", bundlePath})
	if err == nil || !strings.Contains(err.Error(), "commitment conflict") {
		t.Fatalf("runImport error = %v, want commitment conflict", err)
	}
}

func TestRunDoctorDetectsMissingArtifactCAS(t *testing.T) {
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
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	now := time.Date(2026, 6, 24, 22, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	if err := runCommit(root, store, registry, now.Add(time.Minute), []string{
		"--promiser", "Alice",
		"--repo", "fixture",
		"--branch", "main",
		"--target", "fixture/main/TODO-ravud/1",
		"--due", "2026-07-01",
		"--promise", "I promise to complete subtask 1.",
	}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	commitment := latestCommitment(t, store)
	if err := os.Remove(store.CAS.Path(commitment.ArtifactCID)); err != nil {
		t.Fatalf("remove CAS object: %v", err)
	}

	summary, err := doctorReport(root, store, registry)
	if err != nil {
		t.Fatalf("doctorReport: %v", err)
	}
	if len(summary.Errors) != 1 || !strings.Contains(summary.Errors[0], "missing CAS bytes") {
		t.Fatalf("unexpected doctor summary: %#v", summary)
	}

	err = runDoctor(root, store, registry, nil)
	if err == nil || !strings.Contains(err.Error(), "doctor found 1 error") {
		t.Fatalf("runDoctor error = %v, want doctor failure", err)
	}
}

func TestRunRepairRebuildsRecordsAndProtocolCAS(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}
	if err := persistProtocolSpecs(store, registry); err != nil {
		t.Fatalf("persistProtocolSpecs: %v", err)
	}

	repoPath := filepath.Join(root, "fixture-repo")
	writeFixtureRepo(t, repoPath, false)
	gitCommitAll(t, repoPath, "Initial TODO state")
	cfg := config.ReposConfig{
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	now := time.Date(2026, 6, 24, 22, 30, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	if err := runCommit(root, store, registry, now.Add(time.Minute), []string{
		"--promiser", "Alice",
		"--repo", "fixture",
		"--branch", "main",
		"--target", "fixture/main/TODO-ravud/1",
		"--due", "2026-07-01",
		"--promise", "I promise to complete subtask 1.",
	}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	commitment := latestCommitment(t, store)
	recordPath := filepath.Join(root, "records", "commitments", commitment.CommitmentID+".md")
	if err := os.Remove(recordPath); err != nil {
		t.Fatalf("remove record: %v", err)
	}
	pcid := registry.MustPCID(protocol.CommitmentPromise)
	if err := os.Remove(store.CAS.Path(pcid)); err != nil {
		t.Fatalf("remove protocol CAS object: %v", err)
	}

	out := captureStdout(t, func() error {
		return runRepair(root, store, registry, nil)
	})
	if !strings.Contains(out, "Rewrote commitment records: 1") ||
		!strings.Contains(out, "Restored built-in protocol docs to CAS: 7") ||
		!strings.Contains(out, "Restored imported artifact envelopes: 0") {
		t.Fatalf("unexpected repair output:\n%s", out)
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatalf("expected repaired record file: %v", err)
	}
	if _, err := os.Stat(store.CAS.Path(pcid)); err != nil {
		t.Fatalf("expected repaired protocol CAS file: %v", err)
	}
}

func TestRunAssessRejectsKeptForIncompleteParentTarget(t *testing.T) {
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
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	if err := runCommit(root, store, registry, now.Add(time.Minute), []string{
		"--promiser", "Alice",
		"--repo", "fixture",
		"--branch", "main",
		"--target", "fixture/main/TODO-ravud",
		"--due", "2026-07-01",
		"--promise", "I promise to complete the whole TODO.",
	}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}

	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		t.Fatalf("LoadLatestCommitments: %v", err)
	}
	var current model.Commitment
	for _, item := range commitments {
		current = item
	}

	err = runAssess(root, store, registry, now.Add(2*time.Minute), []string{
		"--commitment", current.CommitmentID,
		"--assessor", "Alice",
		"--status", model.StatusKept,
		"--notes", "Trying to assess early.",
	})
	if err == nil || !strings.Contains(err.Error(), "incomplete subtasks") {
		t.Fatalf("runAssess error = %v, want incomplete subtasks", err)
	}
}

func TestRunScanRemovesMissingTargetsFromLatestWorkItems(t *testing.T) {
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
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan initial: %v", err)
	}

	if err := removeFixtureDetailFile(repoPath); err != nil {
		t.Fatalf("remove detail file: %v", err)
	}
	gitCommitAll(t, repoPath, "Remove detail file")

	if err := runScan(root, store, registry, now.Add(time.Minute), []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan second: %v", err)
	}

	workItems, err := store.LoadLatestWorkItems()
	if err != nil {
		t.Fatalf("LoadLatestWorkItems: %v", err)
	}
	if _, ok := workItems["fixture/main/TODO-ravud/1"]; ok {
		t.Fatal("expected removed subtask to be absent after second scan")
	}
	if _, ok := workItems["fixture/main/TODO-ravud"]; !ok {
		t.Fatal("expected parent target to remain after second scan")
	}
}

func TestRunReportPromiserShowsAllTerminalOutcomes(t *testing.T) {
	store := ledger.NewStore(t.TempDir())
	items := []model.Commitment{
		{CommitmentID: "a", Promiser: "Alice", Status: model.StatusOpen},
		{CommitmentID: "b", Promiser: "Alice", Status: model.StatusKept},
		{CommitmentID: "c", Promiser: "Alice", Status: model.StatusPartiallyKept},
		{CommitmentID: "d", Promiser: "Alice", Status: model.StatusExpiredUnassessed},
		{CommitmentID: "e", Promiser: "Alice", Status: model.StatusBroken},
		{CommitmentID: "f", Promiser: "Alice", Status: model.StatusRefused},
		{CommitmentID: "g", Promiser: "Alice", Status: model.StatusDelegated},
		{CommitmentID: "h", Promiser: "Alice", Status: model.StatusSuperseded},
		{CommitmentID: "i", Promiser: "Alice", Status: model.StatusExtended},
	}
	for _, item := range items {
		if err := store.AppendCommitment(item); err != nil {
			t.Fatalf("append commitment %s: %v", item.CommitmentID, err)
		}
	}

	out := captureStdout(t, func() error {
		return runReport(t.TempDir(), store, []string{"--promiser", "Alice"})
	})
	for _, fragment := range []string{
		"Promiser: Alice",
		"Open commitments: 1",
		"Kept: 1",
		"Partially kept: 1",
		"Expired unassessed: 1",
		"Broken: 1",
		"Refused: 1",
		"Delegated: 1",
		"Superseded: 1",
		"Extended: 1",
	} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("report output missing %q:\n%s", fragment, out)
		}
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
	if len(payload.ClaimedProtocolPCIDs) != 7 {
		t.Fatalf("claimed_protocol_pcids len = %d, want 7", len(payload.ClaimedProtocolPCIDs))
	}
	if len(payload.EmittedProtocolPCIDs) != 5 {
		t.Fatalf("emitted_protocol_pcids len = %d, want 5", len(payload.EmittedProtocolPCIDs))
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

func TestRunConformanceCanWriteChangelogAndInspectShowsCoverage(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}

	now := time.Date(2026, 6, 24, 19, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	out := captureStdout(t, func() error {
		return runConformance(root, store, registry, now, []string{"--version", "v0.2.0", "--write-changelog"})
	})
	if !strings.Contains(out, "Updated "+changelog.Path(root)) {
		t.Fatalf("conformance output missing changelog update:\n%s", out)
	}

	body, err := os.ReadFile(changelog.Path(root))
	if err != nil {
		t.Fatalf("read changelog: %v", err)
	}
	if !strings.Contains(string(body), "Current commitment-ledger v0.2.0 emission for local frozen `implementation-conformance-v1`.") {
		t.Fatalf("changelog missing implementation conformance entry:\n%s", string(body))
	}

	artifacts, err := store.LoadArtifacts()
	if err != nil {
		t.Fatalf("LoadArtifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("got %d artifacts, want 1", len(artifacts))
	}

	inspectOut := captureStdout(t, func() error {
		return runInspect(root, store, registry, []string{artifacts[0].ArtifactCID})
	})
	if !strings.Contains(inspectOut, "Protocol: "+protocol.ImplementationConformance) ||
		!strings.Contains(inspectOut, "Conformance Claim: implements (scope=full, breaking-change=false)") {
		t.Fatalf("inspect output missing conformance coverage:\n%s", inspectOut)
	}
}

func TestRunSendAndReceiveRoundTrip(t *testing.T) {
	exportRoot := t.TempDir()
	copyProtocolDocs(t, exportRoot)
	exportStore := ledger.NewStore(exportRoot)
	exportRegistry, err := protocol.Load(exportRoot)
	if err != nil {
		t.Fatalf("protocol.Load export: %v", err)
	}

	repoPath := filepath.Join(exportRoot, "fixture-repo")
	writeFixtureRepo(t, repoPath, false)
	gitCommitAll(t, repoPath, "Initial TODO state")
	cfg := config.ReposConfig{
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(exportRoot, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	now := time.Date(2026, 6, 24, 20, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(exportRoot, exportStore, exportRegistry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	if err := runCommit(exportRoot, exportStore, exportRegistry, now.Add(time.Minute), []string{
		"--promiser", "Alice",
		"--repo", "fixture",
		"--branch", "main",
		"--target", "fixture/main/TODO-ravud/1",
		"--due", "2026-07-01",
		"--promise", "I promise to complete subtask 1.",
	}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	commitment := latestCommitment(t, exportStore)
	outbox := filepath.Join(exportRoot, "outbox")
	if err := runSend(exportRoot, exportStore, exportRegistry, now.Add(2*time.Minute), []string{"--outbox", outbox, commitment.CommitmentID}); err != nil {
		t.Fatalf("runSend: %v", err)
	}
	files, err := os.ReadDir(outbox)
	if err != nil {
		t.Fatalf("ReadDir outbox: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("outbox files = %d, want 1", len(files))
	}

	importRoot := t.TempDir()
	copyProtocolDocs(t, importRoot)
	importStore := ledger.NewStore(importRoot)
	importRegistry, err := protocol.Load(importRoot)
	if err != nil {
		t.Fatalf("protocol.Load import: %v", err)
	}
	archive := filepath.Join(importRoot, "archive")
	if err := runReceive(importRoot, importStore, importRegistry, now.Add(3*time.Minute), []string{"--inbox", outbox, "--archive", archive}); err != nil {
		t.Fatalf("runReceive: %v", err)
	}
	archived, err := os.ReadDir(archive)
	if err != nil {
		t.Fatalf("ReadDir archive: %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("archived files = %d, want 1", len(archived))
	}
	artifacts, err := importStore.LoadArtifacts()
	if err != nil {
		t.Fatalf("LoadArtifacts import: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("got %d imported artifacts, want 2", len(artifacts))
	}

	verifyOut := captureStdout(t, func() error {
		return runVerify(importRoot, importStore, importRegistry, []string{commitment.CommitmentID})
	})
	if !strings.Contains(verifyOut, "Kind: commitment_promise") ||
		!strings.Contains(verifyOut, "Latest Import Mode: receive") {
		t.Fatalf("unexpected receive verify output:\n%s", verifyOut)
	}
	receiptArtifact := artifacts[1]
	if receiptArtifact.Kind != "exchange_receipt" {
		t.Fatalf("receipt artifact kind = %q, want exchange_receipt", receiptArtifact.Kind)
	}
	receiptOut := captureStdout(t, func() error {
		return runVerify(importRoot, importStore, importRegistry, []string{receiptArtifact.RelatedID})
	})
	if !strings.Contains(receiptOut, "Kind: exchange_receipt") ||
		!strings.Contains(receiptOut, "Protocol: "+protocol.ExchangeReceipt) {
		t.Fatalf("unexpected receipt verify output:\n%s", receiptOut)
	}
}

func TestRunVerifyAppliesTrustPolicy(t *testing.T) {
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
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	now := time.Date(2026, 6, 24, 21, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	if err := runCommit(root, store, registry, now.Add(time.Minute), []string{
		"--promiser", "Alice",
		"--repo", "fixture",
		"--branch", "main",
		"--target", "fixture/main/TODO-ravud/1",
		"--due", "2026-07-01",
		"--promise", "I promise to complete subtask 1.",
	}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	commitment := latestCommitment(t, store)

	writeTrustPolicy(t, root, map[string]any{
		"trust_built_in_signers":   false,
		"trust_built_in_protocols": false,
		"trusted_signers":          []string{"Alice"},
		"trusted_protocol_pcids":   []string{commitment.ProtocolPCID},
	})

	out := captureStdout(t, func() error {
		return runVerify(root, store, registry, []string{commitment.CommitmentID})
	})
	for _, fragment := range []string{
		"Signer Trusted: yes (listed in trust policy)",
		"Protocol Trusted: yes (listed in trust policy)",
		"Import Source Trusted: n/a (no import provenance for this artifact)",
		"Overall Trust: yes",
	} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("verify output missing %q:\n%s", fragment, out)
		}
	}
}

func TestRunStatusExchangeAndReportImports(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}
	bundle := syntheticBundle(t, root, "external-protocol-v1", []byte("external protocol doc"), "Mallory")
	inbox := filepath.Join(root, "peer-inbox")
	bundlePath := filepath.Join(inbox, "bundle.json")
	writeBundle(t, bundlePath, bundle)
	writeTrustPolicy(t, root, map[string]any{
		"trust_built_in_signers":       true,
		"trust_built_in_protocols":     true,
		"trusted_signers":              []string{"Mallory"},
		"trusted_protocol_pcids":       []string{bundle.Artifact.ProtocolPCID},
		"trusted_import_modes":         []string{"receive"},
		"trusted_import_path_prefixes": []string{inbox},
	})

	if err := runReceive(root, store, registry, time.Date(2026, 6, 24, 21, 30, 0, 0, time.FixedZone("PDT", -7*3600)), []string{"--inbox", inbox}); err != nil {
		t.Fatalf("runReceive: %v", err)
	}

	statusOut := captureStdout(t, func() error {
		return runStatus(root, store, []string{"--exchange"})
	})
	for _, fragment := range []string{
		"Total imports: 1",
		"Unique imported artifacts: 1",
		"Trusted imports: 1",
		"Receipt artifacts: 1",
		"Imported artifacts with receipts: 1",
		"Mode receive: 1",
	} {
		if !strings.Contains(statusOut, fragment) {
			t.Fatalf("status --exchange output missing %q:\n%s", fragment, statusOut)
		}
	}

	reportOut := captureStdout(t, func() error {
		return runReport(root, store, []string{"--imports"})
	})
	for _, fragment := range []string{
		"Source: " + bundlePath,
		"Imports: 1",
		"Trusted: yes",
		"Modes: receive",
		"Receipt Artifacts: 1",
		"Receipt Signers: commitment-ledger",
	} {
		if !strings.Contains(reportOut, fragment) {
			t.Fatalf("report --imports output missing %q:\n%s", fragment, reportOut)
		}
	}
}

func TestRunDoctorAndReportJSON(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}
	bundle := syntheticBundle(t, root, "external-protocol-v1", []byte("external protocol doc"), "Mallory")
	inbox := filepath.Join(root, "peer-inbox")
	bundlePath := filepath.Join(inbox, "bundle.json")
	writeBundle(t, bundlePath, bundle)
	if err := runReceive(root, store, registry, time.Date(2026, 6, 24, 21, 45, 0, 0, time.FixedZone("PDT", -7*3600)), []string{"--inbox", inbox}); err != nil {
		t.Fatalf("runReceive: %v", err)
	}

	doctorOut := captureStdout(t, func() error {
		return runDoctor(root, store, registry, []string{"--json"})
	})
	if !strings.Contains(doctorOut, "\"artifacts\": 2") ||
		!strings.Contains(doctorOut, "\"errors\": []") ||
		!strings.Contains(doctorOut, "\"repairable_errors\": []") {
		t.Fatalf("unexpected doctor json output:\n%s", doctorOut)
	}

	reportOut := captureStdout(t, func() error {
		return runReport(root, store, []string{"--imports", "--json"})
	})
	if !strings.Contains(reportOut, strconv.Quote(bundlePath)+": {") ||
		!strings.Contains(reportOut, "\"trusted\": false") ||
		!strings.Contains(reportOut, "\"receipt_artifacts\": 1") {
		t.Fatalf("unexpected report json output:\n%s", reportOut)
	}
}

func TestRunInspectAndVerifyJSON(t *testing.T) {
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
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)

	now := time.Date(2026, 6, 24, 23, 15, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	if err := runCommit(root, store, registry, now.Add(time.Minute), []string{
		"--promiser", "Alice",
		"--repo", "fixture",
		"--branch", "main",
		"--target", "fixture/main/TODO-ravud/1",
		"--due", "2026-07-01",
		"--promise", "I promise to complete subtask 1.",
	}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	commitment := latestCommitment(t, store)

	inspectOut := captureStdout(t, func() error {
		return runInspect(root, store, registry, []string{"--json", commitment.CommitmentID})
	})
	for _, fragment := range []string{
		"\"reference\": " + strconv.Quote(commitment.CommitmentID),
		"\"kind\": \"commitment_promise\"",
		"\"protocol_name\": " + strconv.Quote(protocol.CommitmentPromise),
	} {
		if !strings.Contains(inspectOut, fragment) {
			t.Fatalf("inspect json output missing %q:\n%s", fragment, inspectOut)
		}
	}

	verifyOut := captureStdout(t, func() error {
		return runVerify(root, store, registry, []string{"--json", commitment.CommitmentID})
	})
	for _, fragment := range []string{
		"\"artifact_cid\": " + strconv.Quote(commitment.ArtifactCID),
		"\"signature_verified\": true",
		"\"overall_trusted\": true",
		"\"protocol_name\": " + strconv.Quote(protocol.CommitmentPromise),
	} {
		if !strings.Contains(verifyOut, fragment) {
			t.Fatalf("verify json output missing %q:\n%s", fragment, verifyOut)
		}
	}
}

func TestRunStatusJSON(t *testing.T) {
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
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)
	now := time.Date(2026, 6, 24, 23, 30, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan: %v", err)
	}

	statusOut := captureStdout(t, func() error {
		return runStatus(root, store, []string{"--json"})
	})
	for _, fragment := range []string{
		"\"repo\": \"fixture\"",
		"\"branch\": \"main\"",
		"\"open_todos\": 1",
	} {
		if !strings.Contains(statusOut, fragment) {
			t.Fatalf("status json output missing %q:\n%s", fragment, statusOut)
		}
	}
}

func TestRunDoctorRepairableHintsImportedArtifactCAS(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}
	bundle := syntheticBundle(t, root, "external-protocol-v1", []byte("external protocol doc"), "Mallory")
	inbox := filepath.Join(root, "peer-inbox")
	bundlePath := filepath.Join(inbox, "bundle.json")
	writeBundle(t, bundlePath, bundle)
	now := time.Date(2026, 6, 24, 23, 45, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runImportAt(root, store, registry, now, []string{"--in", bundlePath}); err != nil {
		t.Fatalf("runImportAt: %v", err)
	}
	if err := os.Remove(store.CAS.Path(bundle.Artifact.ArtifactCID)); err != nil {
		t.Fatalf("remove imported artifact CAS: %v", err)
	}

	doctorOut, doctorErr := captureStdoutWithError(t, func() error {
		return runDoctor(root, store, registry, []string{"--repairable"})
	})
	if doctorErr == nil || !strings.Contains(doctorErr.Error(), "doctor found 1 error") {
		t.Fatalf("runDoctor --repairable error = %v, want doctor failure", doctorErr)
	}
	if !strings.Contains(doctorOut, "Repairable Errors: 1") ||
		!strings.Contains(doctorOut, "run repair --import-artifacts") ||
		!strings.Contains(doctorOut, "Non-repairable Errors: 0") {
		t.Fatalf("unexpected doctor repairable output:\n%s", doctorOut)
	}

	report, err := doctorReport(root, store, registry)
	if err != nil {
		t.Fatalf("doctorReport: %v", err)
	}
	if len(report.RepairableErrors) != 1 || len(report.NonRepairableErrors) != 0 || len(report.RepairHints) != 1 {
		t.Fatalf("unexpected doctor report classification: %#v", report)
	}
}

func TestRunProvenanceShowsReceiveHistoryAndReceipts(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}
	bundle := syntheticBundle(t, root, "external-protocol-v1", []byte("external protocol doc"), "Mallory")
	inbox := filepath.Join(root, "peer-inbox")
	bundlePath := filepath.Join(inbox, "bundle.json")
	writeBundle(t, bundlePath, bundle)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runReceive(root, store, registry, now, []string{"--inbox", inbox}); err != nil {
		t.Fatalf("runReceive: %v", err)
	}

	textOut := captureStdout(t, func() error {
		return runProvenance(root, store, []string{"--mode", "receive", "--signer", "Mallory"})
	})
	for _, fragment := range []string{
		"Mode: receive",
		"Source: " + bundlePath,
		"Artifact CID: " + bundle.Artifact.ArtifactCID,
		"Receipt Count: 1",
	} {
		if !strings.Contains(textOut, fragment) {
			t.Fatalf("provenance output missing %q:\n%s", fragment, textOut)
		}
	}

	jsonOut := captureStdout(t, func() error {
		return runProvenance(root, store, []string{"--artifact", bundle.Artifact.ArtifactCID, "--json"})
	})
	for _, fragment := range []string{
		"\"artifact_cid\": " + strconv.Quote(bundle.Artifact.ArtifactCID),
		"\"mode\": \"receive\"",
		"\"receipts\": [",
	} {
		if !strings.Contains(jsonOut, fragment) {
			t.Fatalf("provenance json output missing %q:\n%s", fragment, jsonOut)
		}
	}
}

func TestRunIdentityRotateArchivesAndReplacesKey(t *testing.T) {
	root := t.TempDir()
	if _, _, _, err := identity.LoadOrCreate(root, "Alice"); err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	before, err := loadIdentityInfo(root, "Alice")
	if err != nil {
		t.Fatalf("loadIdentityInfo before: %v", err)
	}

	out := captureStdout(t, func() error {
		return runIdentity(root, []string{"rotate", "--name", "Alice"})
	})
	if !strings.Contains(out, "Old Key ID: "+before.KeyID) || !strings.Contains(out, "New Key ID: alice-ed25519-v2") {
		t.Fatalf("unexpected rotate output:\n%s", out)
	}

	after, err := loadIdentityInfo(root, "Alice")
	if err != nil {
		t.Fatalf("loadIdentityInfo after: %v", err)
	}
	if after.KeyID != "alice-ed25519-v2" {
		t.Fatalf("rotated key id = %q, want alice-ed25519-v2", after.KeyID)
	}
	archivePath := filepath.Join(root, "config", "identities", "archive", "alice-"+before.KeyID+".json")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archived identity: %v", err)
	}
}

func TestRunIdentityHistoryShowsCurrentAndArchivedKeys(t *testing.T) {
	root := t.TempDir()
	if _, _, _, err := identity.LoadOrCreate(root, "Alice"); err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if err := runIdentity(root, []string{"rotate", "--name", "Alice"}); err != nil {
		t.Fatalf("runIdentity rotate: %v", err)
	}

	textOut := captureStdout(t, func() error {
		return runIdentity(root, []string{"history", "Alice"})
	})
	for _, fragment := range []string{
		"Name: Alice",
		"Current Key ID: alice-ed25519-v2",
		"Archived Keys: 1",
		"Archived Key: alice-ed25519-v1",
	} {
		if !strings.Contains(textOut, fragment) {
			t.Fatalf("identity history output missing %q:\n%s", fragment, textOut)
		}
	}

	jsonOut := captureStdout(t, func() error {
		return runIdentity(root, []string{"history", "--json", "Alice"})
	})
	for _, fragment := range []string{
		"\"name\": \"Alice\"",
		"\"key_id\": \"alice-ed25519-v2\"",
		"\"source\": \"archived\"",
		"\"key_id\": \"alice-ed25519-v1\"",
	} {
		if !strings.Contains(jsonOut, fragment) {
			t.Fatalf("identity history json output missing %q:\n%s", fragment, jsonOut)
		}
	}
}

func TestVerifyAndInspectUseArchivedSignerKeyAfterRotation(t *testing.T) {
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
		Repos: []config.RepoSource{{
			Name:      "fixture",
			LocalPath: repoPath,
			Branch:    "main",
			TodoFile:  "TODO/TODO.md",
			Enabled:   true,
		}},
	}
	configPath := filepath.Join(root, "config", "repos.json")
	writeConfig(t, configPath, cfg)
	now := time.Date(2026, 6, 25, 0, 30, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runScan(root, store, registry, now, []string{"--config", configPath}); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	if err := runCommit(root, store, registry, now.Add(time.Minute), []string{
		"--promiser", "Alice",
		"--repo", "fixture",
		"--branch", "main",
		"--target", "fixture/main/TODO-ravud/1",
		"--due", "2026-07-01",
		"--promise", "I promise to complete subtask 1.",
	}); err != nil {
		t.Fatalf("runCommit: %v", err)
	}
	commitment := latestCommitment(t, store)
	if err := runIdentity(root, []string{"rotate", "--name", "Alice"}); err != nil {
		t.Fatalf("runIdentity rotate: %v", err)
	}

	verifyOut := captureStdout(t, func() error {
		return runVerify(root, store, registry, []string{commitment.CommitmentID})
	})
	if !strings.Contains(verifyOut, "Signer Key State: archived") ||
		!strings.Contains(verifyOut, "Identity Source: archived local identity") {
		t.Fatalf("unexpected verify output after rotation:\n%s", verifyOut)
	}

	inspectOut := captureStdout(t, func() error {
		return runInspect(root, store, registry, []string{commitment.CommitmentID})
	})
	if !strings.Contains(inspectOut, "Signer Key State: archived") ||
		!strings.Contains(inspectOut, "Signer Identity Path: "+filepath.Join(root, "config", "identities", "archive", "alice-alice-ed25519-v1.json")) {
		t.Fatalf("unexpected inspect output after rotation:\n%s", inspectOut)
	}

	verifyJSON := captureStdout(t, func() error {
		return runVerify(root, store, registry, []string{"--json", commitment.CommitmentID})
	})
	if !strings.Contains(verifyJSON, "\"signer_key_state\": \"archived\"") {
		t.Fatalf("verify json output missing archived key state:\n%s", verifyJSON)
	}
}

func TestRunRepairRestoresImportedArtifactEnvelope(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}
	bundle := syntheticBundle(t, root, "external-protocol-v1", []byte("external protocol doc"), "Mallory")
	inbox := filepath.Join(root, "peer-inbox")
	bundlePath := filepath.Join(inbox, "bundle.json")
	writeBundle(t, bundlePath, bundle)
	now := time.Date(2026, 6, 24, 22, 45, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runImportAt(root, store, registry, now, []string{"--in", bundlePath}); err != nil {
		t.Fatalf("runImportAt: %v", err)
	}
	if err := os.Remove(store.CAS.Path(bundle.Artifact.ArtifactCID)); err != nil {
		t.Fatalf("remove imported artifact CAS: %v", err)
	}

	out := captureStdout(t, func() error {
		return runRepair(root, store, registry, []string{"--import-artifacts"})
	})
	if !strings.Contains(out, "Restored imported artifact envelopes: 1") {
		t.Fatalf("unexpected repair output:\n%s", out)
	}
	if _, err := os.Stat(store.CAS.Path(bundle.Artifact.ArtifactCID)); err != nil {
		t.Fatalf("expected repaired imported artifact CAS: %v", err)
	}
}

func TestRunRepairRestoresImportedSupportFiles(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}
	bundle := syntheticBundle(t, root, "external-protocol-v1", []byte("external protocol doc"), "Mallory")
	inbox := filepath.Join(root, "peer-inbox")
	bundlePath := filepath.Join(inbox, "bundle.json")
	writeBundle(t, bundlePath, bundle)
	now := time.Date(2026, 6, 25, 0, 15, 0, 0, time.FixedZone("PDT", -7*3600))
	if err := runImportAt(root, store, registry, now, []string{"--in", bundlePath}); err != nil {
		t.Fatalf("runImportAt: %v", err)
	}

	protocolMetaPath := filepath.Join(root, "data", "imported-protocols", bundle.Protocol.ProtocolPCID+".json")
	protocolDocPath := filepath.Join(root, "data", "imported-protocols", bundle.Protocol.ProtocolPCID+".md")
	identityPath := filepath.Join(root, "config", "imported-identities", importedIdentityFilename(bundle.Signer.Name))
	if err := os.Remove(protocolMetaPath); err != nil {
		t.Fatalf("remove imported protocol metadata: %v", err)
	}
	if err := os.Remove(protocolDocPath); err != nil {
		t.Fatalf("remove imported protocol doc: %v", err)
	}
	if err := os.Remove(identityPath); err != nil {
		t.Fatalf("remove imported signer support: %v", err)
	}

	doctorOut, doctorErr := captureStdoutWithError(t, func() error {
		return runDoctor(root, store, registry, []string{"--repairable"})
	})
	if doctorErr == nil || !strings.Contains(doctorErr.Error(), "doctor found 2 error") {
		t.Fatalf("runDoctor --repairable error = %v, want doctor failure", doctorErr)
	}
	if !strings.Contains(doctorOut, "run repair --import-support") {
		t.Fatalf("doctor repairable output missing import-support hint:\n%s", doctorOut)
	}

	out := captureStdout(t, func() error {
		return runRepair(root, store, registry, []string{"--import-support"})
	})
	if !strings.Contains(out, "Restored imported protocol support files: 1") ||
		!strings.Contains(out, "Restored imported signer support files: 1") {
		t.Fatalf("unexpected repair support output:\n%s", out)
	}
	if _, err := os.Stat(protocolMetaPath); err != nil {
		t.Fatalf("expected restored imported protocol metadata: %v", err)
	}
	if _, err := os.Stat(protocolDocPath); err != nil {
		t.Fatalf("expected restored imported protocol doc: %v", err)
	}
	if _, err := os.Stat(identityPath); err != nil {
		t.Fatalf("expected restored imported signer support: %v", err)
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

func removeFixtureDetailFile(repoPath string) error {
	indexPath := filepath.Join(repoPath, "TODO", "TODO.md")
	detailPath := filepath.Join(repoPath, "TODO", "TODO-ravud-ship-welcome-flow.md")
	index := "# TODO Index\n\n- [ ] TODO-ravud - Ship welcome flow\n"
	if err := os.WriteFile(indexPath, []byte(index), 0o644); err != nil {
		return err
	}
	if err := os.Remove(detailPath); err != nil {
		return err
	}
	return nil
}

func latestCommitment(t *testing.T, store *ledger.Store) model.Commitment {
	t.Helper()
	items, err := store.LoadLatestCommitments()
	if err != nil {
		t.Fatalf("LoadLatestCommitments: %v", err)
	}
	var current model.Commitment
	for _, item := range items {
		current = item
	}
	if current.CommitmentID == "" {
		t.Fatal("expected latest commitment")
	}
	return current
}

func latestEvidenceByType(t *testing.T, store *ledger.Store, evidenceType string) model.Evidence {
	t.Helper()
	items, err := store.LoadEvidence()
	if err != nil {
		t.Fatalf("LoadEvidence: %v", err)
	}
	var current model.Evidence
	for _, item := range items {
		if item.EvidenceType == evidenceType {
			current = item
		}
	}
	if current.EvidenceID == "" {
		t.Fatalf("expected evidence of type %s", evidenceType)
	}
	return current
}

func latestAssessment(t *testing.T, store *ledger.Store) model.Assessment {
	t.Helper()
	items, err := store.LoadAssessments()
	if err != nil {
		t.Fatalf("LoadAssessments: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected latest assessment")
	}
	return items[len(items)-1]
}

func syntheticBundle(t *testing.T, root string, protocolName string, protocolDoc []byte, signerName string) exchange.Bundle {
	t.Helper()
	protocolPCID := protocol.SupportPCID(protocolDoc)
	ident, pub, priv, err := identity.LoadOrCreate(root, signerName)
	if err != nil {
		t.Fatalf("LoadOrCreate identity: %v", err)
	}
	payload := []byte(`{"kind":"synthetic_exchange_test"}`)
	artifact, err := grid.Build(protocolPCID, payload, ident.Name, ident.KeyID, pub, priv)
	if err != nil {
		t.Fatalf("grid.Build: %v", err)
	}
	return exchange.Bundle{
		Version:    exchange.BundleVersion,
		ExportedAt: time.Date(2026, 6, 24, 18, 0, 0, 0, time.FixedZone("PDT", -7*3600)).Format(time.RFC3339),
		Artifact: model.ArtifactRecord{
			ArtifactCID:  artifact.EnvelopeCID,
			ProtocolPCID: artifact.ProtocolPCID,
			Kind:         "synthetic_test",
			Signer:       ident.Name,
			SignerKeyID:  ident.KeyID,
			PayloadCID:   artifact.PayloadCID,
			ProofCID:     artifact.ProofCID,
			ObservedAt:   time.Date(2026, 6, 24, 18, 0, 0, 0, time.FixedZone("PDT", -7*3600)).Format(time.RFC3339),
			RelatedID:    artifact.EnvelopeCID,
		},
		Envelope: base64.StdEncoding.EncodeToString(artifact.Envelope),
		Protocol: &exchange.ProtocolSupport{
			Name:          protocolName,
			ProtocolPCID:  protocolPCID,
			DocCID:        protocolPCID,
			DocumentBytes: base64.StdEncoding.EncodeToString(protocolDoc),
		},
		Signer: &exchange.SignerSupport{
			Name:      ident.Name,
			KeyID:     ident.KeyID,
			PublicKey: ident.PublicKey,
		},
	}
}

func writeBundle(t *testing.T, path string, bundle exchange.Bundle) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir bundle dir: %v", err)
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
}

func writeTrustPolicy(t *testing.T, root string, payload map[string]any) {
	t.Helper()
	path := filepath.Join(root, "config", "trust-policy.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir trust policy dir: %v", err)
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal trust policy: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write trust policy: %v", err)
	}
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
	out, runErr := captureStdoutWithError(t, fn)
	if runErr != nil {
		t.Fatalf("captured function error: %v", runErr)
	}
	return out
}

func captureStdoutWithError(t *testing.T, fn func() error) (string, error) {
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
	if readErr != nil {
		t.Fatalf("read captured stdout: %v", readErr)
	}
	return string(data), runErr
}
