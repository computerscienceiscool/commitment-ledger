package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"commitment-ledger/internal/assessment"
	"commitment-ledger/internal/commitment"
	"commitment-ledger/internal/config"
	"commitment-ledger/internal/evidence"
	"commitment-ledger/internal/exchange"
	"commitment-ledger/internal/gitrepo"
	"commitment-ledger/internal/grid"
	"commitment-ledger/internal/identity"
	"commitment-ledger/internal/ledger"
	"commitment-ledger/internal/model"
	"commitment-ledger/internal/protocol"
	"commitment-ledger/internal/report"
	"commitment-ledger/internal/todo"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	const root = "."
	store := ledger.NewStore(root)
	registry, err := protocol.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "commitment-ledger:", err)
		os.Exit(1)
	}
	if err := persistProtocolSpecs(store, registry); err != nil {
		fmt.Fprintln(os.Stderr, "commitment-ledger:", err)
		os.Exit(1)
	}
	now := time.Now()

	switch os.Args[1] {
	case "scan":
		err = runScan(root, store, registry, now, os.Args[2:])
	case "status":
		err = runStatus(store)
	case "commit":
		err = runCommit(root, store, registry, now, os.Args[2:])
	case "evidence":
		err = runEvidence(root, store, registry, now, os.Args[2:])
	case "assess":
		err = runAssess(root, store, registry, now, os.Args[2:])
	case "conformance":
		err = runConformance(root, store, registry, now, os.Args[2:])
	case "expire":
		err = runExpire(store, now)
	case "report":
		err = runReport(store, os.Args[2:])
	case "inspect":
		err = runInspect(root, store, registry, os.Args[2:])
	case "verify":
		err = runVerify(root, store, registry, os.Args[2:])
	case "export":
		err = runExport(root, store, registry, now, os.Args[2:])
	case "import":
		err = runImport(root, store, registry, os.Args[2:])
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "commitment-ledger:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("usage: commitment-ledger <scan|status|commit|evidence|assess|conformance|expire|report|inspect|verify|export|import> [flags]")
}

func runScan(root string, store *ledger.Store, registry protocol.Registry, now time.Time, args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	configPath := fs.String("config", "config/repos.json", "path to repo config")
	observer := fs.String("observer", "commitment-ledger", "observer identity for emitted evidence")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	prior, err := store.LoadLatestWorkItems()
	if err != nil {
		return err
	}
	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	existingEvidence, err := store.LoadEvidence()
	if err != nil {
		return err
	}

	for _, repoCfg := range cfg.Repos {
		if !repoCfg.Enabled {
			continue
		}

		state, err := gitrepo.Observe(repoCfg.LocalPath)
		if err != nil {
			return err
		}
		if repoCfg.Branch != "" && repoCfg.Branch != state.Branch {
			return fmt.Errorf("repo %s is on branch %s but config expects %s", repoCfg.Name, state.Branch, repoCfg.Branch)
		}

		items, err := todo.Parse(repoCfg.Name, state.Branch, state.Commit, repoCfg.ResolveTodoPath(), now, prior)
		if err != nil {
			return err
		}
		currentTargets := make(map[string]struct{}, len(items))
		for _, item := range items {
			currentTargets[model.WorkTarget(item.Repo, item.Branch, item.WorkID)] = struct{}{}
		}

		removed := removedWorkItems(repoCfg.Name, state.Branch, state.Commit, now, prior, currentTargets)
		persisted := append(append([]model.WorkItem(nil), items...), removed...)
		if err := store.AppendWorkItems(persisted); err != nil {
			return err
		}

		workMap := make(map[string]model.WorkItem, len(items))
		snapshot := model.Snapshot{
			Repo:       repoCfg.Name,
			Branch:     state.Branch,
			Commit:     state.Commit,
			ObservedAt: now.Format(time.RFC3339),
		}
		for _, item := range items {
			target := model.WorkTarget(item.Repo, item.Branch, item.WorkID)
			workMap[target] = item
			if item.IsSubtask {
				snapshot.SubtasksDiscovered++
			}
			if item.Status == "complete" && !item.IsSubtask {
				snapshot.CompletedWork++
			} else if !item.IsSubtask {
				snapshot.OpenWork++
			}
		}
		if err := store.AppendSnapshot(snapshot); err != nil {
			return err
		}

		derived := evidence.Derive(repoCfg.Name, state.Branch, state.Commit, workMap, commitments, existingEvidence, now)
		for _, item := range derived {
			current := commitments[item.CommitmentID]
			if current.ArtifactCID == "" {
				continue
			}
			item, err = emitEvidenceArtifact(root, store, registry, item, current.ArtifactCID, *observer)
			if err != nil {
				return err
			}
			if err := store.AppendEvidence(item); err != nil {
				return err
			}
		}
		existingEvidence = append(existingEvidence, derived...)

		fmt.Printf("%s %s\n", repoCfg.Name, state.Branch)
		fmt.Printf("Open work: %d\n", snapshot.OpenWork)
		fmt.Printf("Completed work: %d\n", snapshot.CompletedWork)
		fmt.Printf("Subtasks discovered: %d\n", snapshot.SubtasksDiscovered)
		fmt.Printf("Commit: %s\n\n", state.Commit)
	}

	return nil
}

func removedWorkItems(repo string, branch string, commit string, now time.Time, prior map[string]model.WorkItem, currentTargets map[string]struct{}) []model.WorkItem {
	var removed []model.WorkItem
	seenAt := now.Format(time.RFC3339)
	for target, item := range prior {
		if item.Repo != repo || item.Branch != branch {
			continue
		}
		if _, ok := currentTargets[target]; ok {
			continue
		}
		item.Commit = commit
		item.LastSeen = seenAt
		item.Removed = true
		removed = append(removed, item)
	}
	sort.Slice(removed, func(i, j int) bool {
		return removed[i].WorkID < removed[j].WorkID
	})
	return removed
}

func runStatus(store *ledger.Store) error {
	workItems, err := store.LoadLatestWorkItems()
	if err != nil {
		return err
	}
	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	for _, summary := range report.RepoSummaries(workItems, commitments) {
		fmt.Printf("Repo: %s\n", summary.Repo)
		fmt.Printf("Branch: %s\n", summary.Branch)
		fmt.Printf("Open TODOs: %d\n", summary.OpenTODOs)
		fmt.Printf("Open subtasks: %d\n", summary.OpenSubtasks)
		fmt.Printf("Active commitments: %d\n", summary.ActiveCommitments)
		fmt.Printf("Expired unassessed: %d\n", summary.Expired)
		fmt.Printf("Kept commitments: %d\n", summary.Kept)
		fmt.Printf("Partially kept: %d\n", summary.PartiallyKept)
		fmt.Printf("Broken: %d\n", summary.Broken)
		fmt.Printf("Refused: %d\n", summary.Refused)
		fmt.Printf("Delegated: %d\n", summary.Delegated)
		fmt.Printf("Superseded: %d\n", summary.Superseded)
		fmt.Printf("Extended: %d\n\n", summary.Extended)
	}
	return nil
}

func runCommit(root string, store *ledger.Store, registry protocol.Registry, now time.Time, args []string) error {
	fs := flag.NewFlagSet("commit", flag.ContinueOnError)
	promiser := fs.String("promiser", "", "promiser name")
	promisee := fs.String("promisee", "", "promisee name")
	repo := fs.String("repo", "", "repo name")
	branch := fs.String("branch", "", "branch")
	due := fs.String("due", "", "due date (YYYY-MM-DD)")
	promise := fs.String("promise", "", "promise text")
	var targets stringList
	fs.Var(&targets, "target", "work target (repeat)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	item, err := commitment.Create(store, *promiser, *repo, *branch, targets, *due, *promise, now)
	if err != nil {
		return err
	}
	item, err = emitCommitmentArtifact(root, store, registry, item, *promisee)
	if err != nil {
		return err
	}
	if err := store.AppendCommitment(item); err != nil {
		return err
	}
	fmt.Printf("%s %s\n", item.CommitmentID, item.ArtifactCID)
	return nil
}

func runEvidence(root string, store *ledger.Store, registry protocol.Registry, now time.Time, args []string) error {
	fs := flag.NewFlagSet("evidence", flag.ContinueOnError)
	commitmentID := fs.String("commitment", "", "commitment id")
	observer := fs.String("observer", "commitment-ledger", "observer identity")
	evidenceType := fs.String("type", model.EvidenceTypeManualNote, "evidence type")
	repo := fs.String("repo", "", "repo name")
	branch := fs.String("branch", "", "branch")
	commitHash := fs.String("commit", "", "commit hash")
	target := fs.String("target", "", "work target")
	notes := fs.String("notes", "", "notes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *commitmentID == "" {
		return fmt.Errorf("commitment is required")
	}

	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	current, ok := commitments[*commitmentID]
	if !ok {
		return fmt.Errorf("unknown commitment %q", *commitmentID)
	}
	if current.ArtifactCID == "" {
		return fmt.Errorf("commitment %q has no artifact cid; recreate it through the PromiseGrid path", *commitmentID)
	}
	*repo, *branch, err = validateEvidenceInput(store, current, *repo, *branch, *target)
	if err != nil {
		return err
	}

	existing, err := store.LoadEvidence()
	if err != nil {
		return err
	}
	item := evidence.NewManual(existing, *commitmentID, *evidenceType, *repo, *branch, *commitHash, *target, *notes, now)
	item, err = emitEvidenceArtifact(root, store, registry, item, current.ArtifactCID, *observer)
	if err != nil {
		return err
	}
	if err := store.AppendEvidence(item); err != nil {
		return err
	}
	fmt.Printf("%s %s\n", item.EvidenceID, item.ArtifactCID)
	return nil
}

func runAssess(root string, store *ledger.Store, registry protocol.Registry, now time.Time, args []string) error {
	fs := flag.NewFlagSet("assess", flag.ContinueOnError)
	commitmentID := fs.String("commitment", "", "commitment id")
	assessor := fs.String("assessor", "", "assessor")
	status := fs.String("status", "", "assessment status")
	notes := fs.String("notes", "", "notes")
	var basis stringList
	fs.Var(&basis, "basis", "evidence id or artifact cid (repeat)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *commitmentID == "" {
		return fmt.Errorf("commitment is required")
	}

	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	current, ok := commitments[*commitmentID]
	if !ok {
		return fmt.Errorf("unknown commitment %q", *commitmentID)
	}
	if current.ArtifactCID == "" {
		return fmt.Errorf("commitment %q has no artifact cid; recreate it through the PromiseGrid path", *commitmentID)
	}

	assessments, err := store.LoadAssessments()
	if err != nil {
		return err
	}
	evidenceItems, err := store.LoadEvidence()
	if err != nil {
		return err
	}
	resolvedBasis, err := resolveBasis(basis, evidenceItems, current.CommitmentID)
	if err != nil {
		return err
	}
	workItems, err := store.LoadLatestWorkItems()
	if err != nil {
		return err
	}
	if err := validateAssessmentAgainstWork(current, *status, workItems); err != nil {
		return err
	}
	assessmentRecord, updated, err := assessment.Create(assessments, current, *assessor, *status, resolvedBasis, *notes, now)
	if err != nil {
		return err
	}
	assessmentRecord, err = emitAssessmentArtifact(root, store, registry, assessmentRecord, current.ArtifactCID)
	if err != nil {
		return err
	}
	updated.ArtifactCID = current.ArtifactCID
	updated.ProtocolPCID = current.ProtocolPCID
	updated.Signer = current.Signer
	updated.SignerKeyID = current.SignerKeyID
	if err := store.AppendAssessment(assessmentRecord, updated); err != nil {
		return err
	}
	fmt.Printf("%s %s\n", assessmentRecord.AssessmentID, assessmentRecord.ArtifactCID)
	return nil
}

func runExpire(store *ledger.Store, now time.Time) error {
	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	updates := commitment.ExpireDue(commitments, now)
	for _, item := range updates {
		if err := store.AppendCommitment(item); err != nil {
			return err
		}
		fmt.Printf("%s -> %s\n", item.CommitmentID, item.Status)
	}
	return nil
}

func runConformance(root string, store *ledger.Store, registry protocol.Registry, now time.Time, args []string) error {
	fs := flag.NewFlagSet("conformance", flag.ContinueOnError)
	signer := fs.String("signer", "commitment-ledger", "signer identity")
	version := fs.String("version", "v0.1.0", "implementation version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	artifactCID, err := emitConformanceArtifact(root, store, registry, *signer, *version, now)
	if err != nil {
		return err
	}
	fmt.Println(artifactCID)
	return nil
}

func validateEvidenceInput(store *ledger.Store, current model.Commitment, repo string, branch string, target string) (string, string, error) {
	if repo == "" {
		repo = current.Repo
	}
	if branch == "" {
		branch = current.Branch
	}
	if repo != current.Repo || branch != current.Branch {
		return "", "", fmt.Errorf("evidence repo/branch must match commitment %s/%s", current.Repo, current.Branch)
	}
	if target == "" {
		return repo, branch, nil
	}

	targetRepo, targetBranch, _, ok := model.SplitTarget(target)
	if !ok {
		return "", "", fmt.Errorf("invalid target %q", target)
	}
	if targetRepo != current.Repo || targetBranch != current.Branch {
		return "", "", fmt.Errorf("target %q does not match commitment repo=%s branch=%s", target, current.Repo, current.Branch)
	}

	workItems, err := store.LoadLatestWorkItems()
	if err != nil {
		return "", "", err
	}
	if _, ok := workItems[target]; !ok {
		return "", "", fmt.Errorf("unknown target %q; run scan first", target)
	}
	if !commitmentCoversTarget(current, target) {
		return "", "", fmt.Errorf("target %q is outside commitment %q", target, current.CommitmentID)
	}
	return repo, branch, nil
}

func commitmentCoversTarget(current model.Commitment, target string) bool {
	for _, promised := range current.Targets {
		if target == promised || strings.HasPrefix(target, promised+"/") {
			return true
		}
	}
	return false
}

func runReport(store *ledger.Store, args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	repoName := fs.String("repo", "", "repo name")
	branch := fs.String("branch", "", "branch")
	promiser := fs.String("promiser", "", "promiser")
	workTarget := fs.String("work", "", "work target")
	if err := fs.Parse(args); err != nil {
		return err
	}

	workItems, err := store.LoadLatestWorkItems()
	if err != nil {
		return err
	}
	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}

	if *workTarget != "" {
		summary, err := report.FindWorkSummary(*workTarget, workItems, commitments)
		if err != nil {
			return err
		}
		fmt.Printf("Work: %s\n", summary.Target)
		fmt.Printf("Status: %s\n", summary.Status)
		fmt.Printf("Subtasks: %d\n", summary.Subtasks)
		fmt.Printf("Completed subtasks: %d\n", summary.CompletedSubtasks)
		fmt.Println("Commitments:")
		for _, item := range summary.Commitments {
			fmt.Printf("- %s promised %s by %s", item.Promiser, strings.Join(item.Targets, ", "), item.DueDate)
			if item.ArtifactCID != "" {
				fmt.Printf(" (%s)", item.ArtifactCID)
			}
			fmt.Println()
		}
		return nil
	}

	if *promiser != "" {
		persons := report.PersonSummaries(commitments)
		for _, item := range persons {
			if item.Promiser != *promiser {
				continue
			}
			fmt.Printf("Promiser: %s\n", item.Promiser)
			fmt.Printf("Open commitments: %d\n", item.OpenCommitments)
			fmt.Printf("Kept: %d\n", item.Kept)
			fmt.Printf("Partially kept: %d\n", item.PartiallyKept)
			fmt.Printf("Expired unassessed: %d\n", item.ExpiredUnassessed)
			fmt.Printf("Broken: %d\n", item.Broken)
			fmt.Printf("Refused: %d\n", item.Refused)
			fmt.Printf("Delegated: %d\n", item.Delegated)
			fmt.Printf("Superseded: %d\n", item.Superseded)
			fmt.Printf("Extended: %d\n", item.Extended)
			return nil
		}
		return fmt.Errorf("no commitments for promiser %q", *promiser)
	}

	summaries := report.RepoSummaries(workItems, commitments)
	if *repoName != "" {
		filtered := summaries[:0]
		for _, item := range summaries {
			if item.Repo == *repoName && (*branch == "" || item.Branch == *branch) {
				filtered = append(filtered, item)
			}
		}
		summaries = filtered
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Repo < summaries[j].Repo })
	for _, item := range summaries {
		fmt.Printf("Repo: %s\n", item.Repo)
		fmt.Printf("Branch: %s\n", item.Branch)
		fmt.Printf("Open TODOs: %d\n", item.OpenTODOs)
		fmt.Printf("Open subtasks: %d\n", item.OpenSubtasks)
		fmt.Printf("Active commitments: %d\n", item.ActiveCommitments)
		fmt.Printf("Expired unassessed: %d\n", item.Expired)
		fmt.Printf("Kept commitments: %d\n", item.Kept)
		fmt.Printf("Partially kept: %d\n", item.PartiallyKept)
		fmt.Printf("Broken: %d\n", item.Broken)
		fmt.Printf("Refused: %d\n", item.Refused)
		fmt.Printf("Delegated: %d\n", item.Delegated)
		fmt.Printf("Superseded: %d\n", item.Superseded)
		fmt.Printf("Extended: %d\n\n", item.Extended)
	}
	return nil
}

func runInspect(root string, store *ledger.Store, registry protocol.Registry, args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("inspect requires exactly one reference")
	}
	ref := fs.Arg(0)

	artifacts, err := store.LoadArtifacts()
	if err != nil {
		return err
	}
	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	evidenceItems, err := store.LoadEvidence()
	if err != nil {
		return err
	}
	assessments, err := store.LoadAssessments()
	if err != nil {
		return err
	}

	view, err := inspectReference(root, registry, ref, artifacts, commitments, evidenceItems, assessments)
	if err != nil {
		return err
	}
	printInspectView(view)
	return nil
}

func runVerify(root string, store *ledger.Store, registry protocol.Registry, args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("verify requires exactly one reference")
	}
	ref := fs.Arg(0)

	artifacts, err := store.LoadArtifacts()
	if err != nil {
		return err
	}
	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	evidenceItems, err := store.LoadEvidence()
	if err != nil {
		return err
	}
	assessments, err := store.LoadAssessments()
	if err != nil {
		return err
	}

	artifact, err := resolveArtifactReference(ref, artifacts, commitments, evidenceItems, assessments)
	if err != nil {
		return err
	}
	envelope, err := store.CAS.Get(artifact.ArtifactCID)
	if err != nil {
		return err
	}
	decoded, err := grid.DecodeEnvelope(envelope)
	if err != nil {
		return err
	}
	if decoded.EnvelopeCID != artifact.ArtifactCID {
		return fmt.Errorf("artifact cid mismatch: index=%s decoded=%s", artifact.ArtifactCID, decoded.EnvelopeCID)
	}
	if decoded.ProtocolPCID != artifact.ProtocolPCID {
		return fmt.Errorf("protocol pCID mismatch: index=%s decoded=%s", artifact.ProtocolPCID, decoded.ProtocolPCID)
	}
	if decoded.PayloadCID != artifact.PayloadCID {
		return fmt.Errorf("payload cid mismatch: index=%s decoded=%s", artifact.PayloadCID, decoded.PayloadCID)
	}
	if decoded.ProofCID != artifact.ProofCID {
		return fmt.Errorf("proof cid mismatch: index=%s decoded=%s", artifact.ProofCID, decoded.ProofCID)
	}
	if decoded.Proof.Signer != artifact.Signer {
		return fmt.Errorf("signer mismatch: index=%s proof=%s", artifact.Signer, decoded.Proof.Signer)
	}
	if decoded.Proof.KeyID != artifact.SignerKeyID {
		return fmt.Errorf("signer key mismatch: index=%s proof=%s", artifact.SignerKeyID, decoded.Proof.KeyID)
	}
	ident, _, err := identity.LoadVerifier(root, decoded.Proof.Signer)
	if err != nil {
		return fmt.Errorf("load signer identity %q: %w", decoded.Proof.Signer, err)
	}
	if ident.KeyID != decoded.Proof.KeyID {
		return fmt.Errorf("local identity key mismatch: identity=%s proof=%s", ident.KeyID, decoded.Proof.KeyID)
	}
	if ident.PublicKey != decoded.Proof.PublicKey {
		return fmt.Errorf("local identity public key mismatch for signer %q", decoded.Proof.Signer)
	}
	if err := grid.Verify(decoded.ProtocolPCID, decoded.PayloadBytes, decoded.ProofBytes); err != nil {
		return err
	}

	printVerifyResult(root, registry, artifact, decoded, ident)
	return nil
}

func runExport(root string, store *ledger.Store, registry protocol.Registry, now time.Time, args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	outPath := fs.String("out", "", "bundle output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *outPath == "" {
		return fmt.Errorf("export requires --out")
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("export requires exactly one reference")
	}
	ref := fs.Arg(0)

	artifacts, err := store.LoadArtifacts()
	if err != nil {
		return err
	}
	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	evidenceItems, err := store.LoadEvidence()
	if err != nil {
		return err
	}
	assessments, err := store.LoadAssessments()
	if err != nil {
		return err
	}
	artifact, err := resolveArtifactReference(ref, artifacts, commitments, evidenceItems, assessments)
	if err != nil {
		return err
	}
	envelope, err := store.CAS.Get(artifact.ArtifactCID)
	if err != nil {
		return err
	}
	bundle, err := buildBundle(root, registry, now, artifact, envelope, commitments, evidenceItems, assessments)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bundle: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		return fmt.Errorf("mkdir export path: %w", err)
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		return fmt.Errorf("write bundle %q: %w", *outPath, err)
	}
	fmt.Printf("Exported %s to %s\n", emptyFallback(artifact.RelatedID, artifact.ArtifactCID), *outPath)
	return nil
}

func runImport(root string, store *ledger.Store, registry protocol.Registry, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	inPath := fs.String("in", "", "bundle input path")
	installSupport := fs.Bool("install-support", true, "install bundled protocol and signer support")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inPath == "" {
		return fmt.Errorf("import requires --in")
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("import does not accept positional references")
	}
	data, err := os.ReadFile(*inPath)
	if err != nil {
		return fmt.Errorf("read bundle %q: %w", *inPath, err)
	}
	var bundle exchange.Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("parse bundle %q: %w", *inPath, err)
	}
	if bundle.Version != exchange.BundleVersion {
		return fmt.Errorf("unsupported bundle version %q", bundle.Version)
	}
	envelope, err := base64.StdEncoding.DecodeString(bundle.Envelope)
	if err != nil {
		return fmt.Errorf("decode bundle envelope: %w", err)
	}
	decoded, err := grid.DecodeEnvelope(envelope)
	if err != nil {
		return err
	}
	if decoded.EnvelopeCID != bundle.Artifact.ArtifactCID {
		return fmt.Errorf("bundle artifact cid mismatch: record=%s decoded=%s", bundle.Artifact.ArtifactCID, decoded.EnvelopeCID)
	}
	if decoded.ProtocolPCID != bundle.Artifact.ProtocolPCID {
		return fmt.Errorf("bundle protocol pCID mismatch: record=%s decoded=%s", bundle.Artifact.ProtocolPCID, decoded.ProtocolPCID)
	}
	if decoded.PayloadCID != bundle.Artifact.PayloadCID {
		return fmt.Errorf("bundle payload cid mismatch: record=%s decoded=%s", bundle.Artifact.PayloadCID, decoded.PayloadCID)
	}
	if decoded.ProofCID != bundle.Artifact.ProofCID {
		return fmt.Errorf("bundle proof cid mismatch: record=%s decoded=%s", bundle.Artifact.ProofCID, decoded.ProofCID)
	}

	if *installSupport {
		if bundle.Protocol != nil {
			if err := installProtocolSupport(root, *bundle.Protocol); err != nil {
				return err
			}
		}
		if bundle.Signer != nil {
			if err := installSignerSupport(root, *bundle.Signer); err != nil {
				return err
			}
		}
	}
	if _, err := store.CAS.Put(envelope); err != nil {
		return err
	}

	artifacts, err := store.LoadArtifacts()
	if err != nil {
		return err
	}
	if !artifactExists(artifacts, bundle.Artifact.ArtifactCID) {
		if err := store.AppendArtifact(bundle.Artifact, envelope); err != nil {
			return err
		}
	}

	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	if bundle.Commitment != nil {
		if current, ok := commitments[bundle.Commitment.CommitmentID]; !ok || !sameCommitment(current, *bundle.Commitment) {
			if err := store.AppendCommitment(*bundle.Commitment); err != nil {
				return err
			}
			commitments[bundle.Commitment.CommitmentID] = *bundle.Commitment
		}
	}

	if bundle.Evidence != nil {
		existingEvidence, err := store.LoadEvidence()
		if err != nil {
			return err
		}
		if !evidenceExists(existingEvidence, bundle.Evidence.EvidenceID) {
			if err := store.AppendEvidence(*bundle.Evidence); err != nil {
				return err
			}
		}
	}

	if bundle.Assessment != nil {
		existingAssessments, err := store.LoadAssessments()
		if err != nil {
			return err
		}
		if !assessmentExists(existingAssessments, bundle.Assessment.AssessmentID) {
			commitmentRecord := model.Commitment{}
			if bundle.Commitment != nil {
				commitmentRecord = *bundle.Commitment
			} else if current, ok := commitments[bundle.Assessment.CommitmentID]; ok {
				commitmentRecord = current
			}
			if commitmentRecord.CommitmentID == "" {
				return fmt.Errorf("assessment import requires related commitment projection")
			}
			if err := store.AppendAssessment(*bundle.Assessment, commitmentRecord); err != nil {
				return err
			}
		}
	}

	fmt.Printf("Imported %s from %s\n", emptyFallback(bundle.Artifact.RelatedID, bundle.Artifact.ArtifactCID), *inPath)
	if *installSupport {
		fmt.Println("Support material installed: yes")
	} else {
		fmt.Println("Support material installed: no")
	}
	_ = registry
	return nil
}

type inspectView struct {
	Reference    string
	Kind         string
	RelatedID    string
	RelatedCID   string
	ArtifactCID  string
	ProtocolName string
	ProtocolPCID string
	ProtocolPath string
	Signer       string
	SignerKeyID  string
	PayloadCID   string
	ProofCID     string
	ObservedAt   string
	RecordPath   string
	Details      []string
}

func inspectReference(root string, registry protocol.Registry, ref string, artifacts []model.ArtifactRecord, commitments map[string]model.Commitment, evidenceItems []model.Evidence, assessments []model.Assessment) (inspectView, error) {
	artifactByCID := artifactIndexByCID(artifacts)

	if item, ok := commitments[ref]; ok {
		return buildCommitmentInspectView(root, registry, ref, item, artifactByCID[item.ArtifactCID]), nil
	}
	for _, item := range evidenceItems {
		if item.EvidenceID == ref {
			return buildEvidenceInspectView(root, registry, ref, item, artifactByCID[item.ArtifactCID], commitments), nil
		}
	}
	for _, item := range assessments {
		if item.AssessmentID == ref {
			return buildAssessmentInspectView(root, registry, ref, item, artifactByCID[item.ArtifactCID], commitments), nil
		}
	}
	if artifact, ok := artifactByCID[ref]; ok {
		switch artifact.Kind {
		case "commitment_promise":
			if item, ok := commitments[artifact.RelatedID]; ok {
				return buildCommitmentInspectView(root, registry, ref, item, artifact), nil
			}
		case "commitment_evidence":
			for _, item := range evidenceItems {
				if item.EvidenceID == artifact.RelatedID {
					return buildEvidenceInspectView(root, registry, ref, item, artifact, commitments), nil
				}
			}
		case "commitment_assessment":
			for _, item := range assessments {
				if item.AssessmentID == artifact.RelatedID {
					return buildAssessmentInspectView(root, registry, ref, item, artifact, commitments), nil
				}
			}
		}
		return buildArtifactInspectView(root, registry, ref, artifact), nil
	}

	return inspectView{}, fmt.Errorf("unknown inspect reference %q", ref)
}

func artifactIndexByCID(artifacts []model.ArtifactRecord) map[string]model.ArtifactRecord {
	out := make(map[string]model.ArtifactRecord, len(artifacts))
	for _, item := range artifacts {
		out[item.ArtifactCID] = item
	}
	return out
}

func resolveArtifactReference(ref string, artifacts []model.ArtifactRecord, commitments map[string]model.Commitment, evidenceItems []model.Evidence, assessments []model.Assessment) (model.ArtifactRecord, error) {
	artifactByCID := artifactIndexByCID(artifacts)
	if artifact, ok := artifactByCID[ref]; ok {
		return artifact, nil
	}
	if item, ok := commitments[ref]; ok && item.ArtifactCID != "" {
		if artifact, ok := artifactByCID[item.ArtifactCID]; ok {
			return artifact, nil
		}
	}
	for _, item := range evidenceItems {
		if item.EvidenceID == ref && item.ArtifactCID != "" {
			if artifact, ok := artifactByCID[item.ArtifactCID]; ok {
				return artifact, nil
			}
		}
	}
	for _, item := range assessments {
		if item.AssessmentID == ref && item.ArtifactCID != "" {
			if artifact, ok := artifactByCID[item.ArtifactCID]; ok {
				return artifact, nil
			}
		}
	}
	return model.ArtifactRecord{}, fmt.Errorf("unknown verify reference %q", ref)
}

func artifactExists(artifacts []model.ArtifactRecord, artifactCID string) bool {
	for _, item := range artifacts {
		if item.ArtifactCID == artifactCID {
			return true
		}
	}
	return false
}

func evidenceExists(items []model.Evidence, evidenceID string) bool {
	for _, item := range items {
		if item.EvidenceID == evidenceID {
			return true
		}
	}
	return false
}

func assessmentExists(items []model.Assessment, assessmentID string) bool {
	for _, item := range items {
		if item.AssessmentID == assessmentID {
			return true
		}
	}
	return false
}

func sameCommitment(a model.Commitment, b model.Commitment) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func buildBundle(root string, registry protocol.Registry, now time.Time, artifact model.ArtifactRecord, envelope []byte, commitments map[string]model.Commitment, evidenceItems []model.Evidence, assessments []model.Assessment) (exchange.Bundle, error) {
	bundle := exchange.Bundle{
		Version:    exchange.BundleVersion,
		ExportedAt: now.Format(time.RFC3339),
		Artifact:   artifact,
		Envelope:   base64.StdEncoding.EncodeToString(envelope),
	}
	if artifact.ProtocolPCID != "" {
		support, err := exportProtocolSupport(root, registry, artifact.ProtocolPCID)
		if err == nil {
			bundle.Protocol = support
		}
	}
	if artifact.Signer != "" {
		support, err := exportSignerSupport(root, artifact.Signer)
		if err == nil {
			bundle.Signer = support
		}
	}
	switch artifact.Kind {
	case "commitment_promise":
		if item, ok := commitments[artifact.RelatedID]; ok {
			copy := item
			bundle.Commitment = &copy
		}
	case "commitment_evidence":
		for _, item := range evidenceItems {
			if item.EvidenceID != artifact.RelatedID {
				continue
			}
			copy := item
			bundle.Evidence = &copy
			if commitment, ok := commitments[item.CommitmentID]; ok {
				commitmentCopy := commitment
				bundle.Commitment = &commitmentCopy
			}
			break
		}
	case "commitment_assessment":
		for _, item := range assessments {
			if item.AssessmentID != artifact.RelatedID {
				continue
			}
			copy := item
			bundle.Assessment = &copy
			if commitment, ok := commitments[item.CommitmentID]; ok {
				commitmentCopy := commitment
				bundle.Commitment = &commitmentCopy
			}
			break
		}
	}
	return bundle, nil
}

func exportProtocolSupport(root string, registry protocol.Registry, pcid string) (*exchange.ProtocolSupport, error) {
	if spec, ok := registry.FindByPCID(pcid); ok {
		return &exchange.ProtocolSupport{
			Name:          spec.Name,
			ProtocolPCID:  spec.PCID,
			DocCID:        spec.DocCID,
			DocumentBytes: base64.StdEncoding.EncodeToString(spec.Bytes),
		}, nil
	}
	support, err := loadImportedProtocolSupport(root, pcid)
	if err != nil {
		return nil, err
	}
	return &support, nil
}

func exportSignerSupport(root string, name string) (*exchange.SignerSupport, error) {
	ident, pub, err := identity.LoadVerifier(root, name)
	if err != nil {
		return nil, err
	}
	return &exchange.SignerSupport{
		Name:      ident.Name,
		KeyID:     ident.KeyID,
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}, nil
}

func installProtocolSupport(root string, support exchange.ProtocolSupport) error {
	data, err := base64.StdEncoding.DecodeString(support.DocumentBytes)
	if err != nil {
		return fmt.Errorf("decode protocol support bytes: %w", err)
	}
	if got := protocol.SupportPCID(data); got != support.ProtocolPCID {
		return fmt.Errorf("protocol support pCID mismatch: bundle=%s computed=%s", support.ProtocolPCID, got)
	}
	docPath := importedProtocolDocPath(root, support.ProtocolPCID)
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		return fmt.Errorf("mkdir imported protocol dir: %w", err)
	}
	if err := os.WriteFile(docPath, data, 0o644); err != nil {
		return fmt.Errorf("write imported protocol doc: %w", err)
	}
	metaPath := importedProtocolMetaPath(root, support.ProtocolPCID)
	meta, err := json.MarshalIndent(support, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal protocol support: %w", err)
	}
	if err := os.WriteFile(metaPath, meta, 0o644); err != nil {
		return fmt.Errorf("write imported protocol metadata: %w", err)
	}
	return nil
}

func installSignerSupport(root string, support exchange.SignerSupport) error {
	path := filepath.Join(root, "config", "imported-identities", importedIdentityFilename(support.Name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir imported identity dir: %w", err)
	}
	payload, err := json.MarshalIndent(identity.Identity{
		Name:      support.Name,
		KeyID:     support.KeyID,
		PublicKey: support.PublicKey,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal signer support: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write imported identity: %w", err)
	}
	return nil
}

func importedIdentityFilename(name string) string {
	return filepath.Base(identityPathForName(name))
}

func importedProtocolDocPath(root string, pcid string) string {
	return filepath.Join(root, "data", "imported-protocols", pcid+".md")
}

func importedProtocolMetaPath(root string, pcid string) string {
	return filepath.Join(root, "data", "imported-protocols", pcid+".json")
}

func loadImportedProtocolSupport(root string, pcid string) (exchange.ProtocolSupport, error) {
	metaPath := importedProtocolMetaPath(root, pcid)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return exchange.ProtocolSupport{}, fmt.Errorf("read imported protocol metadata %q: %w", metaPath, err)
	}
	var support exchange.ProtocolSupport
	if err := json.Unmarshal(data, &support); err != nil {
		return exchange.ProtocolSupport{}, fmt.Errorf("parse imported protocol metadata %q: %w", metaPath, err)
	}
	return support, nil
}

type protocolLocation struct {
	Name     string
	Path     string
	Matched  bool
	Imported bool
}

func resolveProtocolLocation(root string, registry protocol.Registry, pcid string) protocolLocation {
	if spec, ok := registry.FindByPCID(pcid); ok {
		return protocolLocation{Name: spec.Name, Path: spec.Path, Matched: true}
	}
	if support, err := loadImportedProtocolSupport(root, pcid); err == nil {
		return protocolLocation{
			Name:     support.Name,
			Path:     importedProtocolDocPath(root, pcid),
			Matched:  true,
			Imported: true,
		}
	}
	return protocolLocation{}
}

func buildCommitmentInspectView(root string, registry protocol.Registry, ref string, item model.Commitment, artifact model.ArtifactRecord) inspectView {
	view := buildArtifactInspectView(root, registry, ref, artifact)
	view.Kind = "commitment_promise"
	view.RelatedID = item.CommitmentID
	view.RecordPath = filepath.Join(root, "records", "commitments", item.CommitmentID+".md")
	view.Details = []string{
		"Current Status: " + item.Status,
		"Promiser: " + item.Promiser,
		"Repo: " + item.Repo,
		"Branch: " + item.Branch,
		"Due Date: " + item.DueDate,
		"Promise: " + item.PromiseText,
		"Targets: " + strings.Join(item.Targets, ", "),
	}
	if view.ProtocolPCID == "" {
		view.ProtocolPCID = item.ProtocolPCID
	}
	return enrichProtocol(root, registry, view)
}

func buildEvidenceInspectView(root string, registry protocol.Registry, ref string, item model.Evidence, artifact model.ArtifactRecord, commitments map[string]model.Commitment) inspectView {
	view := buildArtifactInspectView(root, registry, ref, artifact)
	view.Kind = "commitment_evidence"
	view.RelatedID = item.EvidenceID
	view.RecordPath = "none (evidence stays in data/evidence.jsonl and commitment markdown)"
	view.Details = []string{
		"Evidence Type: " + item.EvidenceType,
		"Commitment ID: " + item.CommitmentID,
		"Repo: " + item.Repo,
		"Branch: " + item.Branch,
		"Observed Commit: " + item.Commit,
		"Target: " + emptyFallback(item.Target, "(none)"),
		"Observed At: " + item.ObservedAt,
		"Notes: " + emptyFallback(item.Notes, "(none)"),
	}
	if commitment, ok := commitments[item.CommitmentID]; ok {
		view.Details = append([]string{"Current Commitment Status: " + commitment.Status}, view.Details...)
	}
	if view.ProtocolPCID == "" {
		view.ProtocolPCID = item.ProtocolPCID
	}
	return enrichProtocol(root, registry, view)
}

func buildAssessmentInspectView(root string, registry protocol.Registry, ref string, item model.Assessment, artifact model.ArtifactRecord, commitments map[string]model.Commitment) inspectView {
	view := buildArtifactInspectView(root, registry, ref, artifact)
	view.Kind = "commitment_assessment"
	view.RelatedID = item.AssessmentID
	view.RecordPath = filepath.Join(root, "records", "assessments", item.AssessmentID+".md")
	view.Details = []string{
		"Assessment Status: " + item.Status,
		"Commitment ID: " + item.CommitmentID,
		"Assessor: " + item.Assessor,
		"Assessed At: " + item.AssessedAt,
		"Basis Count: " + fmt.Sprintf("%d", len(item.Basis)),
		"Basis: " + emptyFallback(strings.Join(item.Basis, ", "), "(none)"),
		"Notes: " + emptyFallback(item.Notes, "(none)"),
	}
	if commitment, ok := commitments[item.CommitmentID]; ok {
		view.Details = append([]string{"Current Commitment Status: " + commitment.Status}, view.Details...)
	}
	if view.ProtocolPCID == "" {
		view.ProtocolPCID = item.ProtocolPCID
	}
	return enrichProtocol(root, registry, view)
}

func buildArtifactInspectView(root string, registry protocol.Registry, ref string, artifact model.ArtifactRecord) inspectView {
	view := inspectView{
		Reference:    ref,
		Kind:         artifact.Kind,
		RelatedID:    artifact.RelatedID,
		RelatedCID:   artifact.RelatedCID,
		ArtifactCID:  artifact.ArtifactCID,
		ProtocolPCID: artifact.ProtocolPCID,
		Signer:       artifact.Signer,
		SignerKeyID:  artifact.SignerKeyID,
		PayloadCID:   artifact.PayloadCID,
		ProofCID:     artifact.ProofCID,
		ObservedAt:   artifact.ObservedAt,
	}
	return enrichProtocol(root, registry, view)
}

func enrichProtocol(root string, registry protocol.Registry, view inspectView) inspectView {
	if view.ProtocolPCID == "" {
		return view
	}
	location := resolveProtocolLocation(root, registry, view.ProtocolPCID)
	if location.Matched {
		view.ProtocolName = location.Name
		view.ProtocolPath = location.Path
	}
	return view
}

func printInspectView(view inspectView) {
	fmt.Printf("Reference: %s\n", view.Reference)
	fmt.Printf("Kind: %s\n", emptyFallback(view.Kind, "(unknown)"))
	fmt.Printf("Related ID: %s\n", emptyFallback(view.RelatedID, "(none)"))
	if view.RelatedCID != "" {
		fmt.Printf("Related CID: %s\n", view.RelatedCID)
	}
	fmt.Printf("Artifact CID: %s\n", emptyFallback(view.ArtifactCID, "(none)"))
	fmt.Printf("Protocol: %s\n", emptyFallback(view.ProtocolName, "(unknown local spec)"))
	fmt.Printf("Protocol pCID: %s\n", emptyFallback(view.ProtocolPCID, "(none)"))
	if view.ProtocolPath != "" {
		fmt.Printf("Protocol Doc: %s\n", view.ProtocolPath)
	}
	fmt.Printf("Signer: %s\n", emptyFallback(view.Signer, "(none)"))
	fmt.Printf("Signer Key ID: %s\n", emptyFallback(view.SignerKeyID, "(none)"))
	fmt.Printf("Payload CID: %s\n", emptyFallback(view.PayloadCID, "(none)"))
	fmt.Printf("Proof CID: %s\n", emptyFallback(view.ProofCID, "(none)"))
	fmt.Printf("Observed At: %s\n", emptyFallback(view.ObservedAt, "(none)"))
	if view.RecordPath != "" {
		fmt.Printf("Record Path: %s\n", view.RecordPath)
	}
	for _, line := range view.Details {
		fmt.Println(line)
	}
}

func emptyFallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func printVerifyResult(root string, registry protocol.Registry, artifact model.ArtifactRecord, decoded grid.DecodedArtifact, ident identity.Identity) {
	location := resolveProtocolLocation(root, registry, decoded.ProtocolPCID)
	fmt.Printf("Reference: %s\n", emptyFallback(artifact.RelatedID, artifact.ArtifactCID))
	fmt.Printf("Kind: %s\n", emptyFallback(artifact.Kind, "(unknown)"))
	fmt.Printf("Artifact CID: %s\n", artifact.ArtifactCID)
	fmt.Printf("Envelope CID Verified: yes\n")
	fmt.Printf("Payload CID Verified: yes\n")
	fmt.Printf("Proof CID Verified: yes\n")
	fmt.Printf("Signature Verified: yes\n")
	fmt.Printf("Signer Identity Verified: yes\n")
	fmt.Printf("Signer: %s\n", decoded.Proof.Signer)
	fmt.Printf("Signer Key ID: %s\n", decoded.Proof.KeyID)
	fmt.Printf("Local Identity File: %s\n", identitySupportPath(root, decoded.Proof.Signer))
	fmt.Printf("Protocol pCID: %s\n", decoded.ProtocolPCID)
	if location.Matched {
		fmt.Printf("Local Protocol Match: yes\n")
		fmt.Printf("Protocol: %s\n", location.Name)
		fmt.Printf("Protocol Doc: %s\n", location.Path)
	} else {
		fmt.Printf("Local Protocol Match: no\n")
	}
	fmt.Printf("Payload CID: %s\n", decoded.PayloadCID)
	fmt.Printf("Proof CID: %s\n", decoded.ProofCID)
	fmt.Printf("Observed At: %s\n", emptyFallback(artifact.ObservedAt, "(none)"))
	_ = ident
}

func identityPathForName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		b.WriteString("anon")
	}
	return filepath.Join("config", "identities", b.String()+".json")
}

func identitySupportPath(root string, name string) string {
	primary := filepath.Join(root, identityPathForName(name))
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	return filepath.Join(root, "config", "imported-identities", importedIdentityFilename(name))
}

func emitCommitmentArtifact(root string, store *ledger.Store, registry protocol.Registry, item model.Commitment, promisee string) (model.Commitment, error) {
	ident, pub, priv, err := identity.LoadOrCreate(root, item.Promiser)
	if err != nil {
		return model.Commitment{}, err
	}
	payloadBytes, err := protocol.MarshalPayload(protocol.CommitmentPromisePayload{
		Kind:        "commitment_promise",
		Promiser:    item.Promiser,
		Promisee:    promisee,
		Repo:        item.Repo,
		Branch:      item.Branch,
		Targets:     item.Targets,
		PromiseText: item.PromiseText,
		DueDate:     item.DueDate,
		CreatedAt:   item.CreatedAt,
	})
	if err != nil {
		return model.Commitment{}, err
	}
	artifact, err := grid.Build(registry.MustPCID(protocol.CommitmentPromise), payloadBytes, ident.Name, ident.KeyID, pub, priv)
	if err != nil {
		return model.Commitment{}, err
	}
	item.ArtifactCID = artifact.EnvelopeCID
	item.ProtocolPCID = artifact.ProtocolPCID
	item.Signer = ident.Name
	item.SignerKeyID = ident.KeyID
	if err := store.AppendArtifact(model.ArtifactRecord{
		ArtifactCID:  artifact.EnvelopeCID,
		ProtocolPCID: artifact.ProtocolPCID,
		Kind:         "commitment_promise",
		Signer:       ident.Name,
		SignerKeyID:  ident.KeyID,
		PayloadCID:   artifact.PayloadCID,
		ProofCID:     artifact.ProofCID,
		ObservedAt:   item.CreatedAt,
		RelatedID:    item.CommitmentID,
	}, artifact.Envelope); err != nil {
		return model.Commitment{}, err
	}
	return item, nil
}

func emitEvidenceArtifact(root string, store *ledger.Store, registry protocol.Registry, item model.Evidence, commitmentRef string, observer string) (model.Evidence, error) {
	ident, pub, priv, err := identity.LoadOrCreate(root, observer)
	if err != nil {
		return model.Evidence{}, err
	}
	payloadBytes, err := protocol.MarshalPayload(protocol.CommitmentEvidencePayload{
		Kind:             "commitment_evidence",
		CommitmentRef:    commitmentRef,
		Observer:         observer,
		Repo:             item.Repo,
		Branch:           item.Branch,
		SourceCommit:     item.Commit,
		Target:           item.Target,
		EvidenceKind:     item.EvidenceType,
		ObservedAt:       item.ObservedAt,
		ObservedBytesCID: item.ObservedBytesCID,
		Notes:            item.Notes,
	})
	if err != nil {
		return model.Evidence{}, err
	}
	artifact, err := grid.Build(registry.MustPCID(protocol.CommitmentEvidence), payloadBytes, ident.Name, ident.KeyID, pub, priv)
	if err != nil {
		return model.Evidence{}, err
	}
	item.ArtifactCID = artifact.EnvelopeCID
	item.ProtocolPCID = artifact.ProtocolPCID
	item.Signer = ident.Name
	item.SignerKeyID = ident.KeyID
	item.CommitmentRef = commitmentRef
	if err := store.AppendArtifact(model.ArtifactRecord{
		ArtifactCID:  artifact.EnvelopeCID,
		ProtocolPCID: artifact.ProtocolPCID,
		Kind:         "commitment_evidence",
		Signer:       ident.Name,
		SignerKeyID:  ident.KeyID,
		PayloadCID:   artifact.PayloadCID,
		ProofCID:     artifact.ProofCID,
		ObservedAt:   item.ObservedAt,
		RelatedID:    item.EvidenceID,
		RelatedCID:   commitmentRef,
	}, artifact.Envelope); err != nil {
		return model.Evidence{}, err
	}
	return item, nil
}

func emitAssessmentArtifact(root string, store *ledger.Store, registry protocol.Registry, item model.Assessment, commitmentRef string) (model.Assessment, error) {
	ident, pub, priv, err := identity.LoadOrCreate(root, item.Assessor)
	if err != nil {
		return model.Assessment{}, err
	}
	payloadBytes, err := protocol.MarshalPayload(protocol.CommitmentAssessmentPayload{
		Kind:          "commitment_assessment",
		CommitmentRef: commitmentRef,
		Assessor:      item.Assessor,
		Status:        item.Status,
		AssessedAt:    item.AssessedAt,
		Basis:         item.Basis,
		Notes:         item.Notes,
	})
	if err != nil {
		return model.Assessment{}, err
	}
	artifact, err := grid.Build(registry.MustPCID(protocol.CommitmentAssessment), payloadBytes, ident.Name, ident.KeyID, pub, priv)
	if err != nil {
		return model.Assessment{}, err
	}
	item.ArtifactCID = artifact.EnvelopeCID
	item.ProtocolPCID = artifact.ProtocolPCID
	item.Signer = ident.Name
	item.SignerKeyID = ident.KeyID
	item.CommitmentRef = commitmentRef
	if err := store.AppendArtifact(model.ArtifactRecord{
		ArtifactCID:  artifact.EnvelopeCID,
		ProtocolPCID: artifact.ProtocolPCID,
		Kind:         "commitment_assessment",
		Signer:       ident.Name,
		SignerKeyID:  ident.KeyID,
		PayloadCID:   artifact.PayloadCID,
		ProofCID:     artifact.ProofCID,
		ObservedAt:   item.AssessedAt,
		RelatedID:    item.AssessmentID,
		RelatedCID:   commitmentRef,
	}, artifact.Envelope); err != nil {
		return model.Assessment{}, err
	}
	return item, nil
}

func emitConformanceArtifact(root string, store *ledger.Store, registry protocol.Registry, signer string, version string, now time.Time) (string, error) {
	ident, pub, priv, err := identity.LoadOrCreate(root, signer)
	if err != nil {
		return "", err
	}
	payloadBytes, err := protocol.MarshalPayload(buildConformancePayload(registry, version, now))
	if err != nil {
		return "", err
	}
	artifact, err := grid.Build(registry.MustPCID(protocol.ImplementationConformance), payloadBytes, ident.Name, ident.KeyID, pub, priv)
	if err != nil {
		return "", err
	}
	if err := store.AppendArtifact(model.ArtifactRecord{
		ArtifactCID:  artifact.EnvelopeCID,
		ProtocolPCID: artifact.ProtocolPCID,
		Kind:         "implementation_conformance",
		Signer:       ident.Name,
		SignerKeyID:  ident.KeyID,
		PayloadCID:   artifact.PayloadCID,
		ProofCID:     artifact.ProofCID,
		ObservedAt:   now.Format(time.RFC3339),
	}, artifact.Envelope); err != nil {
		return "", err
	}
	return artifact.EnvelopeCID, nil
}

func buildConformancePayload(registry protocol.Registry, version string, now time.Time) protocol.ImplementationConformancePayload {
	claimed := []string{
		registry.MustPCID(protocol.CommitmentPromise),
		registry.MustPCID(protocol.CommitmentEvidenceV1),
		registry.MustPCID(protocol.CommitmentEvidence),
		registry.MustPCID(protocol.CommitmentAssessmentV1),
		registry.MustPCID(protocol.CommitmentAssessment),
		registry.MustPCID(protocol.ImplementationConformance),
	}
	emitted := []string{
		registry.MustPCID(protocol.CommitmentPromise),
		registry.MustPCID(protocol.CommitmentEvidence),
		registry.MustPCID(protocol.CommitmentAssessment),
		registry.MustPCID(protocol.ImplementationConformance),
	}
	historical := []string{
		registry.MustPCID(protocol.CommitmentEvidenceV1),
		registry.MustPCID(protocol.CommitmentAssessmentV1),
	}
	return protocol.ImplementationConformancePayload{
		Kind:                    "implementation_conformance",
		Implementation:          "commitment-ledger",
		Version:                 version,
		ClaimedProtocolPCIDs:    claimed,
		EmittedProtocolPCIDs:    emitted,
		HistoricalProtocolPCIDs: historical,
		ProjectionRules: []string{
			"JSONL files are append-only local indexes over artifact history.",
			"Markdown records are human-readable projections retaining artifact CIDs and protocol pCIDs.",
			"claimed_protocol_pcids names the frozen protocol docs the implementation can interpret locally.",
			"emitted_protocol_pcids names the frozen protocol docs current commands emit for new artifacts.",
			"historical_protocol_pcids names older frozen docs retained for reading historical local artifacts but not emitted by current commands.",
		},
		ClaimedAt: now.Format(time.RFC3339),
	}
}

func persistProtocolSpecs(store *ledger.Store, registry protocol.Registry) error {
	for _, spec := range registry.Specs() {
		if _, err := store.CAS.Put(spec.Bytes); err != nil {
			return err
		}
	}
	return nil
}

func resolveBasis(basis []string, evidenceItems []model.Evidence, commitmentID string) ([]string, error) {
	byID := make(map[string]model.Evidence, len(evidenceItems))
	byCID := make(map[string]model.Evidence, len(evidenceItems))
	for _, item := range evidenceItems {
		if item.ArtifactCID != "" {
			byID[item.EvidenceID] = item
			byCID[item.ArtifactCID] = item
		}
	}
	out := make([]string, 0, len(basis))
	seen := make(map[string]struct{}, len(basis))
	for _, item := range basis {
		evidence, ok := byID[item]
		if ok {
			if evidence.CommitmentID != commitmentID {
				return nil, fmt.Errorf("basis evidence %q belongs to commitment %q, not %q", item, evidence.CommitmentID, commitmentID)
			}
			if _, seenCID := seen[evidence.ArtifactCID]; !seenCID {
				out = append(out, evidence.ArtifactCID)
				seen[evidence.ArtifactCID] = struct{}{}
			}
			continue
		}
		evidence, ok = byCID[item]
		if !ok {
			return nil, fmt.Errorf("unknown basis reference %q", item)
		}
		if evidence.CommitmentID != commitmentID {
			return nil, fmt.Errorf("basis evidence %q belongs to commitment %q, not %q", item, evidence.CommitmentID, commitmentID)
		}
		if _, seenCID := seen[item]; !seenCID {
			out = append(out, item)
			seen[item] = struct{}{}
		}
	}
	return out, nil
}

func validateAssessmentAgainstWork(current model.Commitment, status string, workItems map[string]model.WorkItem) error {
	if status != model.StatusKept {
		return nil
	}
	for _, target := range current.Targets {
		item, ok := workItems[target]
		if !ok {
			return fmt.Errorf("cannot assess commitment %q as kept: unknown target %q; run scan first", current.CommitmentID, target)
		}
		if item.IsSubtask {
			if item.Status != "complete" {
				return fmt.Errorf("cannot assess commitment %q as kept: target %q is not complete", current.CommitmentID, target)
			}
			continue
		}
		if hasIncompleteSubtasks(item, workItems) {
			return fmt.Errorf("cannot assess commitment %q as kept: target %q has incomplete subtasks", current.CommitmentID, target)
		}
		if !hasSubtasks(item, workItems) && item.Status != "complete" {
			return fmt.Errorf("cannot assess commitment %q as kept: target %q is not complete", current.CommitmentID, target)
		}
	}
	return nil
}

func hasSubtasks(item model.WorkItem, workItems map[string]model.WorkItem) bool {
	for _, candidate := range workItems {
		if candidate.Repo == item.Repo && candidate.Branch == item.Branch && candidate.ParentWork == item.WorkID {
			return true
		}
	}
	return false
}

func hasIncompleteSubtasks(item model.WorkItem, workItems map[string]model.WorkItem) bool {
	for _, candidate := range workItems {
		if candidate.Repo == item.Repo && candidate.Branch == item.Branch && candidate.ParentWork == item.WorkID && candidate.Status != "complete" {
			return true
		}
	}
	return false
}

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}
