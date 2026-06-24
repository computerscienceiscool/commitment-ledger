package main

import (
	"crypto/ed25519"
	"crypto/rand"
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
	"commitment-ledger/internal/changelog"
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
	"commitment-ledger/internal/trust"
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
		err = runStatus(root, store, os.Args[2:])
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
		err = runReport(root, store, os.Args[2:])
	case "inspect":
		err = runInspect(root, store, registry, os.Args[2:])
	case "verify":
		err = runVerify(root, store, registry, os.Args[2:])
	case "export":
		err = runExport(root, store, registry, now, os.Args[2:])
	case "import":
		err = runImportAt(root, store, registry, now, os.Args[2:])
	case "provenance":
		err = runProvenance(root, store, os.Args[2:])
	case "send":
		err = runSend(root, store, registry, now, os.Args[2:])
	case "receive":
		err = runReceive(root, store, registry, now, os.Args[2:])
	case "doctor":
		err = runDoctor(root, store, registry, os.Args[2:])
	case "repair":
		err = runRepair(root, store, registry, os.Args[2:])
	case "identity":
		err = runIdentity(root, os.Args[2:])
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
	fmt.Println("usage: commitment-ledger <scan|status|commit|evidence|assess|conformance|expire|report|inspect|verify|export|import|provenance|send|receive|doctor|repair|identity> [flags]")
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

func runStatus(root string, store *ledger.Store, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	exchangeOnly := fs.Bool("exchange", false, "show exchange and import summary instead of repo work summary")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	imports, err := store.LoadImports()
	if err != nil {
		return err
	}
	if *exchangeOnly {
		policy, err := trust.Load(root)
		if err != nil {
			return err
		}
		artifacts, err := store.LoadArtifacts()
		if err != nil {
			return err
		}
		if *jsonOut {
			return printJSON(summarizeImports(policy, imports, artifacts))
		}
		printExchangeStatus(policy, imports, artifacts)
		return nil
	}
	workItems, err := store.LoadLatestWorkItems()
	if err != nil {
		return err
	}
	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return err
	}
	summaries := report.RepoSummaries(workItems, commitments)
	if *jsonOut {
		return printJSON(summaries)
	}
	for _, summary := range summaries {
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
	writeChangelog := fs.Bool("write-changelog", false, "update CHANGELOG.md managed conformance entries")
	if err := fs.Parse(args); err != nil {
		return err
	}
	artifactCID, err := emitConformanceArtifact(root, store, registry, *signer, *version, now)
	if err != nil {
		return err
	}
	if *writeChangelog {
		if err := changelog.WriteManaged(root, registry, *version); err != nil {
			return err
		}
		fmt.Printf("Updated %s\n", changelog.Path(root))
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

func runReport(root string, store *ledger.Store, args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	repoName := fs.String("repo", "", "repo name")
	branch := fs.String("branch", "", "branch")
	promiser := fs.String("promiser", "", "promiser")
	workTarget := fs.String("work", "", "work target")
	importsOnly := fs.Bool("imports", false, "show import and exchange summary")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *importsOnly {
		imports, err := store.LoadImports()
		if err != nil {
			return err
		}
		artifacts, err := store.LoadArtifacts()
		if err != nil {
			return err
		}
		policy, err := trust.Load(root)
		if err != nil {
			return err
		}
		if *jsonOut {
			return printJSON(summarizeImports(policy, imports, artifacts))
		}
		printImportReport(policy, imports, artifacts)
		return nil
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
		if *jsonOut {
			return printJSON(summary)
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
			if *jsonOut {
				return printJSON(item)
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
	if *jsonOut {
		return printJSON(summaries)
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
	jsonOut := fs.Bool("json", false, "emit JSON")
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
	imports, err := store.LoadImports()
	if err != nil {
		return err
	}

	view, err := inspectReference(root, registry, ref, artifacts, commitments, evidenceItems, assessments, imports)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(view)
	}
	printInspectView(view)
	return nil
}

func runVerify(root string, store *ledger.Store, registry protocol.Registry, args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
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
	imports, err := store.LoadImports()
	if err != nil {
		return err
	}
	policy, err := trust.Load(root)
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
	match, err := resolveIdentityMatch(root, decoded.Proof.Signer, decoded.Proof.KeyID)
	if err != nil {
		return fmt.Errorf("load signer identity %q: %w", decoded.Proof.Signer, err)
	}
	if match.Identity.KeyID != decoded.Proof.KeyID {
		return fmt.Errorf("local identity key mismatch: identity=%s proof=%s", match.Identity.KeyID, decoded.Proof.KeyID)
	}
	if match.Identity.PublicKey != decoded.Proof.PublicKey {
		return fmt.Errorf("local identity public key mismatch for signer %q", decoded.Proof.Signer)
	}
	if err := grid.Verify(decoded.ProtocolPCID, decoded.PayloadBytes, decoded.ProofBytes); err != nil {
		return err
	}

	result := buildVerifyResult(root, registry, artifact, decoded, match, latestImportForArtifact(imports, artifact.ArtifactCID), policy)
	if *jsonOut {
		return printJSON(result)
	}
	printVerifyResult(result)
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
	return runImportAt(root, store, registry, time.Now(), args)
}

func runImportAt(root string, store *ledger.Store, registry protocol.Registry, now time.Time, args []string) error {
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
	_, err := importBundlePath(root, store, registry, now, *inPath, *installSupport, "import", true)
	return err
}

func runProvenance(root string, store *ledger.Store, args []string) error {
	fs := flag.NewFlagSet("provenance", flag.ContinueOnError)
	artifactCID := fs.String("artifact", "", "filter by imported artifact CID")
	sourcePath := fs.String("source", "", "filter by source path")
	signer := fs.String("signer", "", "filter by artifact signer")
	mode := fs.String("mode", "", "filter by import mode")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("provenance does not accept positional references")
	}
	imports, err := store.LoadImports()
	if err != nil {
		return err
	}
	artifacts, err := store.LoadArtifacts()
	if err != nil {
		return err
	}
	rows := buildProvenanceRows(imports, artifacts, *artifactCID, *sourcePath, *signer, *mode)
	if *jsonOut {
		return printJSON(rows)
	}
	for _, row := range rows {
		fmt.Printf("Imported At: %s\n", row.ImportedAt)
		fmt.Printf("Mode: %s\n", row.Mode)
		fmt.Printf("Source: %s\n", row.SourcePath)
		fmt.Printf("Artifact CID: %s\n", row.ArtifactCID)
		fmt.Printf("Related ID: %s\n", emptyFallback(row.RelatedID, "(none)"))
		fmt.Printf("Protocol pCID: %s\n", row.ProtocolPCID)
		fmt.Printf("Signer: %s\n", emptyFallback(row.Signer, "(none)"))
		fmt.Printf("Support Installed: %s\n", yesNo(row.SupportInstalled))
		if row.InstalledProtocolPCID != "" {
			fmt.Printf("Installed Protocol pCID: %s\n", row.InstalledProtocolPCID)
		}
		if row.InstalledSignerIdentity != "" {
			fmt.Printf("Installed Signer Identity: %s\n", row.InstalledSignerIdentity)
		}
		fmt.Printf("Receipt Count: %d\n", len(row.Receipts))
		for _, receipt := range row.Receipts {
			fmt.Printf("Receipt: %s by %s at %s\n", receipt.ArtifactCID, receipt.Signer, receipt.ObservedAt)
		}
		fmt.Println()
	}
	return nil
}

func runSend(root string, store *ledger.Store, registry protocol.Registry, now time.Time, args []string) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	outbox := fs.String("outbox", "", "peer outbox directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *outbox == "" {
		return fmt.Errorf("send requires --outbox")
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("send requires exactly one reference")
	}
	ref := fs.Arg(0)
	if err := os.MkdirAll(*outbox, 0o755); err != nil {
		return fmt.Errorf("mkdir outbox: %w", err)
	}
	outPath := filepath.Join(*outbox, sendBundleFilename(now, ref))
	if err := runExport(root, store, registry, now, []string{"--out", outPath, ref}); err != nil {
		return err
	}
	fmt.Printf("Queued bundle for peer exchange: %s\n", outPath)
	return nil
}

func runReceive(root string, store *ledger.Store, registry protocol.Registry, now time.Time, args []string) error {
	fs := flag.NewFlagSet("receive", flag.ContinueOnError)
	inbox := fs.String("inbox", "", "peer inbox directory")
	archive := fs.String("archive", "", "optional archive directory for processed bundles")
	installSupport := fs.Bool("install-support", true, "install bundled protocol and signer support")
	receiptSigner := fs.String("receipt-signer", "commitment-ledger", "signer identity for local receive receipts; empty disables receipts")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inbox == "" {
		return fmt.Errorf("receive requires --inbox")
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("receive does not accept positional references")
	}
	entries, err := os.ReadDir(*inbox)
	if err != nil {
		return fmt.Errorf("read inbox %q: %w", *inbox, err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(*inbox, entry.Name()))
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no bundle files found in %q", *inbox)
	}
	if *archive != "" {
		if err := os.MkdirAll(*archive, 0o755); err != nil {
			return fmt.Errorf("mkdir archive: %w", err)
		}
	}
	for _, path := range paths {
		record, err := importBundlePath(root, store, registry, now, path, *installSupport, "receive", false)
		if err != nil {
			return err
		}
		if strings.TrimSpace(*receiptSigner) != "" {
			if _, err := emitExchangeReceiptArtifact(root, store, registry, record, *receiptSigner, now); err != nil {
				return err
			}
		}
		if *archive != "" {
			dst := filepath.Join(*archive, filepath.Base(path))
			if err := os.Rename(path, dst); err != nil {
				return fmt.Errorf("archive bundle %q: %w", path, err)
			}
		}
	}
	fmt.Printf("Received %d bundle(s) from %s\n", len(paths), *inbox)
	return nil
}

func runDoctor(root string, store *ledger.Store, registry protocol.Registry, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	repairableOnly := fs.Bool("repairable", false, "show repairable findings and hints")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("doctor does not accept positional references")
	}

	report, err := doctorReport(root, store, registry)
	if err != nil {
		return err
	}
	if *jsonOut {
		if err := printJSON(report); err != nil {
			return err
		}
		if len(report.Errors) > 0 {
			return fmt.Errorf("doctor found %d error(s)", len(report.Errors))
		}
		return nil
	}
	if *repairableOnly {
		fmt.Printf("Repairable Errors: %d\n", len(report.RepairableErrors))
		for _, issue := range report.RepairableErrors {
			fmt.Printf("Repairable: %s\n", issue)
		}
		if len(report.RepairHints) == 0 {
			fmt.Println("Repair Hints: none")
		} else {
			for _, hint := range report.RepairHints {
				fmt.Printf("Repair Hint: %s\n", hint)
			}
		}
		fmt.Printf("Non-repairable Errors: %d\n", len(report.NonRepairableErrors))
		for _, issue := range report.NonRepairableErrors {
			fmt.Printf("Non-repairable: %s\n", issue)
		}
		if len(report.Errors) > 0 {
			return fmt.Errorf("doctor found %d error(s)", len(report.Errors))
		}
		return nil
	}
	fmt.Printf("Artifacts indexed: %d\n", report.Artifacts)
	fmt.Printf("Primary identities: %d\n", report.PrimaryIdentities)
	fmt.Printf("Imported identities: %d\n", report.ImportedIdentities)
	fmt.Printf("Imported protocols: %d\n", report.ImportedProtocols)
	fmt.Printf("Warnings: %d\n", len(report.Warnings))
	fmt.Printf("Errors: %d\n", len(report.Errors))
	fmt.Printf("Repairable errors: %d\n", len(report.RepairableErrors))
	for _, warning := range report.Warnings {
		fmt.Printf("Warning: %s\n", warning)
	}
	for _, issue := range report.Errors {
		fmt.Printf("Error: %s\n", issue)
	}
	for _, hint := range report.RepairHints {
		fmt.Printf("Repair Hint: %s\n", hint)
	}
	if len(report.Errors) > 0 {
		return fmt.Errorf("doctor found %d error(s)", len(report.Errors))
	}
	return nil
}

func runRepair(root string, store *ledger.Store, registry protocol.Registry, args []string) error {
	fs := flag.NewFlagSet("repair", flag.ContinueOnError)
	records := fs.Bool("records", false, "rewrite Markdown projection records from JSONL state")
	protocolCAS := fs.Bool("protocol-cas", false, "restore built-in frozen protocol docs into local CAS")
	importArtifacts := fs.Bool("import-artifacts", false, "restore imported artifact envelopes from bundle source paths when possible")
	importSupport := fs.Bool("import-support", false, "restore imported signer and protocol support from bundle source paths when possible")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("repair does not accept positional references")
	}
	if !*records && !*protocolCAS && !*importArtifacts && !*importSupport {
		*records = true
		*protocolCAS = true
		*importArtifacts = true
		*importSupport = true
	}

	var rewrittenCommitments int
	var rewrittenAssessments int
	if *records {
		commitments, err := store.LoadLatestCommitments()
		if err != nil {
			return err
		}
		for _, item := range commitments {
			if err := store.WriteCommitmentRecord(item); err != nil {
				return err
			}
			rewrittenCommitments++
		}
		assessments, err := store.LoadAssessments()
		if err != nil {
			return err
		}
		for _, item := range assessments {
			commitmentRecord, ok := commitments[item.CommitmentID]
			if !ok {
				return fmt.Errorf("assessment %s references missing commitment %s", item.AssessmentID, item.CommitmentID)
			}
			if err := writeRepairTextFile(filepath.Join(root, "records", "assessments", item.AssessmentID+".md"), ledger.AssessmentMarkdown(item, commitmentRecord)); err != nil {
				return err
			}
			rewrittenAssessments++
		}
	}
	restoredProtocols := 0
	if *protocolCAS {
		for _, spec := range registry.Specs() {
			if _, err := store.CAS.Put(spec.Bytes); err != nil {
				return err
			}
			restoredProtocols++
		}
	}
	restoredImportedArtifacts := 0
	if *importArtifacts {
		count, err := restoreImportedArtifacts(store)
		if err != nil {
			return err
		}
		restoredImportedArtifacts = count
	}
	restoredImportedProtocols := 0
	restoredImportedSigners := 0
	if *importSupport {
		protocols, signers, err := restoreImportedSupport(root, store)
		if err != nil {
			return err
		}
		restoredImportedProtocols = protocols
		restoredImportedSigners = signers
	}

	fmt.Printf("Rewrote commitment records: %d\n", rewrittenCommitments)
	fmt.Printf("Rewrote assessment records: %d\n", rewrittenAssessments)
	fmt.Printf("Restored built-in protocol docs to CAS: %d\n", restoredProtocols)
	fmt.Printf("Restored imported artifact envelopes: %d\n", restoredImportedArtifacts)
	fmt.Printf("Restored imported protocol support files: %d\n", restoredImportedProtocols)
	fmt.Printf("Restored imported signer support files: %d\n", restoredImportedSigners)
	return nil
}

type identityInfo struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Path       string `json:"path"`
	KeyID      string `json:"key_id"`
	PublicKey  string `json:"public_key"`
	HasPrivate bool   `json:"has_private"`
}

type identityHistory struct {
	Name     string         `json:"name"`
	Current  *identityInfo  `json:"current,omitempty"`
	Archived []identityInfo `json:"archived,omitempty"`
	Imported *identityInfo  `json:"imported,omitempty"`
}

type identityMatch struct {
	Identity identity.Identity
	Info     identityInfo
	KeyState string
}

func runIdentity(root string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("identity requires a subcommand: list, show, history, rotate")
	}
	switch args[0] {
	case "list":
		return runIdentityList(root, args[1:])
	case "show":
		return runIdentityShow(root, args[1:])
	case "history":
		return runIdentityHistory(root, args[1:])
	case "rotate":
		return runIdentityRotate(root, args[1:])
	default:
		return fmt.Errorf("unknown identity subcommand %q", args[0])
	}
}

func runIdentityList(root string, args []string) error {
	fs := flag.NewFlagSet("identity list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	items, err := listIdentities(root)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(items)
	}
	for _, item := range items {
		fmt.Printf("%s %s %s\n", item.Source, item.Name, item.KeyID)
	}
	return nil
}

func runIdentityShow(root string, args []string) error {
	fs := flag.NewFlagSet("identity show", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("identity show requires exactly one name")
	}
	item, err := loadIdentityInfo(root, fs.Arg(0))
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(item)
	}
	fmt.Printf("Name: %s\n", item.Name)
	fmt.Printf("Source: %s\n", item.Source)
	fmt.Printf("Path: %s\n", item.Path)
	fmt.Printf("Key ID: %s\n", item.KeyID)
	fmt.Printf("Has Private Key: %s\n", yesNo(item.HasPrivate))
	fmt.Printf("Public Key: %s\n", item.PublicKey)
	return nil
}

func runIdentityHistory(root string, args []string) error {
	fs := flag.NewFlagSet("identity history", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("identity history requires exactly one name")
	}
	history, err := loadIdentityHistory(root, fs.Arg(0))
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(history)
	}
	fmt.Printf("Name: %s\n", history.Name)
	if history.Current != nil {
		fmt.Printf("Current Key ID: %s\n", history.Current.KeyID)
		fmt.Printf("Current Path: %s\n", history.Current.Path)
	}
	if history.Imported != nil {
		fmt.Printf("Imported Key ID: %s\n", history.Imported.KeyID)
		fmt.Printf("Imported Path: %s\n", history.Imported.Path)
	}
	fmt.Printf("Archived Keys: %d\n", len(history.Archived))
	for _, item := range history.Archived {
		fmt.Printf("Archived Key: %s %s\n", item.KeyID, item.Path)
	}
	return nil
}

func runIdentityRotate(root string, args []string) error {
	fs := flag.NewFlagSet("identity rotate", flag.ContinueOnError)
	name := fs.String("name", "", "identity name")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("identity rotate requires --name")
	}
	current, _, _, err := identity.Load(root, *name)
	if err != nil {
		return err
	}
	archivePath, rotated, err := rotateIdentity(root, current)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]string{
			"name":          rotated.Name,
			"old_key_id":    current.KeyID,
			"new_key_id":    rotated.KeyID,
			"archive_path":  archivePath,
			"identity_path": filepath.Join(root, identityPathForName(rotated.Name)),
		})
	}
	fmt.Printf("Name: %s\n", rotated.Name)
	fmt.Printf("Old Key ID: %s\n", current.KeyID)
	fmt.Printf("New Key ID: %s\n", rotated.KeyID)
	fmt.Printf("Archived Prior Identity: %s\n", archivePath)
	return nil
}

func rotateIdentity(root string, current identity.Identity) (string, identity.Identity, error) {
	path := filepath.Join(root, identityPathForName(current.Name))
	archivePath := filepath.Join(root, "config", "identities", "archive", strings.TrimSuffix(filepath.Base(path), ".json")+"-"+current.KeyID+".json")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return "", identity.Identity{}, fmt.Errorf("mkdir identity archive: %w", err)
	}
	currentData, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return "", identity.Identity{}, fmt.Errorf("marshal current identity: %w", err)
	}
	if err := os.WriteFile(archivePath, currentData, 0o600); err != nil {
		return "", identity.Identity{}, fmt.Errorf("write identity archive: %w", err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", identity.Identity{}, fmt.Errorf("generate rotated identity: %w", err)
	}
	rotated := identity.Identity{
		Name:       current.Name,
		KeyID:      nextKeyID(current.KeyID),
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
		PrivateKey: base64.StdEncoding.EncodeToString(priv),
	}
	out, err := json.MarshalIndent(rotated, "", "  ")
	if err != nil {
		return "", identity.Identity{}, fmt.Errorf("marshal rotated identity: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return "", identity.Identity{}, fmt.Errorf("write rotated identity: %w", err)
	}
	return archivePath, rotated, nil
}

func nextKeyID(current string) string {
	if idx := strings.LastIndex(current, "-v"); idx >= 0 {
		prefix := current[:idx]
		var version int
		if _, err := fmt.Sscanf(current[idx+2:], "%d", &version); err == nil {
			return fmt.Sprintf("%s-v%d", prefix, version+1)
		}
	}
	return current + "-v2"
}

func listIdentities(root string) ([]identityInfo, error) {
	var out []identityInfo
	if items, err := listIdentityDir(root, filepath.Join(root, "config", "identities"), "primary"); err != nil {
		return nil, err
	} else {
		out = append(out, items...)
	}
	if items, err := listIdentityDir(root, filepath.Join(root, "config", "imported-identities"), "imported"); err != nil {
		return nil, err
	} else {
		out = append(out, items...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source == out[j].Source {
			return out[i].Name < out[j].Name
		}
		return out[i].Source < out[j].Source
	})
	return out, nil
}

func loadIdentityHistory(root string, name string) (identityHistory, error) {
	history := identityHistory{Name: name}
	if current, err := loadIdentityInfo(root, name); err == nil && current.Source == "primary" {
		history.Current = &current
	}
	archived, err := loadArchivedIdentityInfos(root, name)
	if err != nil {
		return identityHistory{}, err
	}
	history.Archived = archived
	if imported, err := loadImportedIdentityInfo(root, name); err == nil {
		history.Imported = &imported
	}
	if history.Current == nil && history.Imported == nil && len(history.Archived) == 0 {
		return identityHistory{}, fmt.Errorf("identity %q not found", name)
	}
	return history, nil
}

func listIdentityDir(root string, dir string, source string) ([]identityInfo, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read identity dir %q: %w", dir, err)
	}
	out := []identityInfo{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		info, err := loadIdentityInfo(root, name)
		if err != nil {
			return nil, err
		}
		if info.Source == source {
			out = append(out, info)
		}
	}
	return out, nil
}

func loadIdentityInfo(root string, name string) (identityInfo, error) {
	if ident, _, _, err := identity.Load(root, name); err == nil {
		return identityInfo{
			Name:       ident.Name,
			Source:     "primary",
			Path:       filepath.Join(root, identityPathForName(name)),
			KeyID:      ident.KeyID,
			PublicKey:  ident.PublicKey,
			HasPrivate: true,
		}, nil
	}
	return loadImportedIdentityInfo(root, name)
}

func loadImportedIdentityInfo(root string, name string) (identityInfo, error) {
	ident, _, err := identity.LoadVerifier(root, name)
	if err != nil {
		return identityInfo{}, err
	}
	path := filepath.Join(root, "config", "imported-identities", importedIdentityFilename(name))
	if _, err := os.Stat(path); err != nil {
		return identityInfo{}, err
	}
	return identityInfo{
		Name:       ident.Name,
		Source:     "imported",
		Path:       path,
		KeyID:      ident.KeyID,
		PublicKey:  ident.PublicKey,
		HasPrivate: ident.PrivateKey != "",
	}, nil
}

func loadArchivedIdentityInfos(root string, name string) ([]identityInfo, error) {
	dir := filepath.Join(root, "config", "identities", "archive")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read identity archive dir %q: %w", dir, err)
	}
	prefix := strings.TrimSuffix(filepath.Base(identityPathForName(name)), ".json") + "-"
	out := []identityInfo{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		ident, err := readIdentityFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, identityInfo{
			Name:       ident.Name,
			Source:     "archived",
			Path:       path,
			KeyID:      ident.KeyID,
			PublicKey:  ident.PublicKey,
			HasPrivate: ident.PrivateKey != "",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].KeyID > out[j].KeyID })
	return out, nil
}

func readIdentityFile(path string) (identity.Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return identity.Identity{}, fmt.Errorf("read identity file %q: %w", path, err)
	}
	var ident identity.Identity
	if err := json.Unmarshal(data, &ident); err != nil {
		return identity.Identity{}, fmt.Errorf("parse identity file %q: %w", path, err)
	}
	return ident, nil
}

func resolveIdentityMatch(root string, name string, keyID string) (identityMatch, error) {
	if ident, _, _, err := identity.Load(root, name); err == nil && ident.KeyID == keyID {
		return identityMatch{
			Identity: ident,
			Info: identityInfo{
				Name:       ident.Name,
				Source:     "primary",
				Path:       filepath.Join(root, identityPathForName(name)),
				KeyID:      ident.KeyID,
				PublicKey:  ident.PublicKey,
				HasPrivate: true,
			},
			KeyState: "active",
		}, nil
	}
	archived, err := loadArchivedIdentityInfos(root, name)
	if err != nil {
		return identityMatch{}, err
	}
	for _, item := range archived {
		if item.KeyID != keyID {
			continue
		}
		ident, err := readIdentityFile(item.Path)
		if err != nil {
			return identityMatch{}, err
		}
		return identityMatch{Identity: ident, Info: item, KeyState: "archived"}, nil
	}
	if imported, err := loadImportedIdentityInfo(root, name); err == nil && imported.KeyID == keyID {
		ident, err := readIdentityFile(imported.Path)
		if err != nil {
			return identityMatch{}, err
		}
		return identityMatch{Identity: ident, Info: imported, KeyState: "imported"}, nil
	}
	return identityMatch{}, fmt.Errorf("signer %q with key %q not found in current, archived, or imported identity material", name, keyID)
}

func restoreImportedArtifacts(store *ledger.Store) (int, error) {
	imports, err := store.LoadImports()
	if err != nil {
		return 0, err
	}
	restored := 0
	seen := map[string]struct{}{}
	for i := len(imports) - 1; i >= 0; i-- {
		record := imports[i]
		if record.ArtifactCID == "" || record.SourcePath == "" {
			continue
		}
		if _, ok := seen[record.ArtifactCID]; ok {
			continue
		}
		seen[record.ArtifactCID] = struct{}{}
		if _, err := store.CAS.Get(record.ArtifactCID); err == nil {
			continue
		}
		data, err := os.ReadFile(record.SourcePath)
		if err != nil {
			return restored, fmt.Errorf("read import bundle %q for %s: %w", record.SourcePath, record.ArtifactCID, err)
		}
		bundle, err := exchange.ParseBundle(data)
		if err != nil {
			return restored, fmt.Errorf("parse import bundle %q for %s: %w", record.SourcePath, record.ArtifactCID, err)
		}
		if bundle.Artifact.ArtifactCID != record.ArtifactCID {
			return restored, fmt.Errorf("bundle %q artifact mismatch: import=%s bundle=%s", record.SourcePath, record.ArtifactCID, bundle.Artifact.ArtifactCID)
		}
		envelope, err := base64.StdEncoding.DecodeString(bundle.Envelope)
		if err != nil {
			return restored, fmt.Errorf("decode import bundle %q envelope: %w", record.SourcePath, err)
		}
		if got, err := store.CAS.Put(envelope); err != nil {
			return restored, err
		} else if got != record.ArtifactCID {
			return restored, fmt.Errorf("restored envelope cid mismatch for %s: got %s", record.ArtifactCID, got)
		}
		restored++
	}
	return restored, nil
}

func restoreImportedSupport(root string, store *ledger.Store) (int, int, error) {
	imports, err := store.LoadImports()
	if err != nil {
		return 0, 0, err
	}
	restoredProtocols := 0
	restoredSigners := 0
	seenProtocols := map[string]struct{}{}
	seenSigners := map[string]struct{}{}
	for i := len(imports) - 1; i >= 0; i-- {
		record := imports[i]
		if record.SourcePath == "" {
			continue
		}
		if record.InstalledProtocolPCID != "" {
			if _, ok := seenProtocols[record.InstalledProtocolPCID]; !ok {
				seenProtocols[record.InstalledProtocolPCID] = struct{}{}
				if !importedProtocolHealthy(root, record.InstalledProtocolPCID) {
					bundle, err := parseBundleAtPath(record.SourcePath)
					if err != nil {
						return restoredProtocols, restoredSigners, err
					}
					if bundle.Protocol == nil || bundle.Protocol.ProtocolPCID != record.InstalledProtocolPCID {
						return restoredProtocols, restoredSigners, fmt.Errorf("bundle %q missing protocol support for %s", record.SourcePath, record.InstalledProtocolPCID)
					}
					if err := installProtocolSupport(root, *bundle.Protocol); err != nil {
						return restoredProtocols, restoredSigners, err
					}
					restoredProtocols++
				}
			}
		}
		if record.InstalledSignerIdentity != "" {
			if _, ok := seenSigners[record.InstalledSignerIdentity]; !ok {
				seenSigners[record.InstalledSignerIdentity] = struct{}{}
				if !importedSignerHealthy(root, record.InstalledSignerIdentity) {
					bundle, err := parseBundleAtPath(record.SourcePath)
					if err != nil {
						return restoredProtocols, restoredSigners, err
					}
					if bundle.Signer == nil || bundle.Signer.Name != record.InstalledSignerIdentity {
						return restoredProtocols, restoredSigners, fmt.Errorf("bundle %q missing signer support for %s", record.SourcePath, record.InstalledSignerIdentity)
					}
					if err := installSignerSupport(root, *bundle.Signer); err != nil {
						return restoredProtocols, restoredSigners, err
					}
					restoredSigners++
				}
			}
		}
	}
	return restoredProtocols, restoredSigners, nil
}

func parseBundleAtPath(path string) (exchange.Bundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return exchange.Bundle{}, fmt.Errorf("read import bundle %q: %w", path, err)
	}
	bundle, err := exchange.ParseBundle(data)
	if err != nil {
		return exchange.Bundle{}, fmt.Errorf("parse import bundle %q: %w", path, err)
	}
	return bundle, nil
}

func importedProtocolHealthy(root string, pcid string) bool {
	support, err := loadImportedProtocolSupport(root, pcid)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(importedProtocolDocPath(root, pcid))
	if err != nil {
		return false
	}
	return protocol.SupportPCID(data) == support.ProtocolPCID
}

func importedSignerHealthy(root string, name string) bool {
	path := filepath.Join(root, "config", "imported-identities", importedIdentityFilename(name))
	if _, err := os.Stat(path); err != nil {
		return false
	}
	_, _, err := identity.LoadVerifier(root, name)
	return err == nil
}

func writeRepairTextFile(path string, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func importBundlePath(root string, store *ledger.Store, registry protocol.Registry, now time.Time, inPath string, installSupport bool, mode string, announce bool) (model.ImportRecord, error) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return model.ImportRecord{}, fmt.Errorf("read bundle %q: %w", inPath, err)
	}
	bundle, err := exchange.ParseBundle(data)
	if err != nil {
		return model.ImportRecord{}, fmt.Errorf("parse bundle %q: %w", inPath, err)
	}
	if bundle.Version != exchange.BundleVersion {
		return model.ImportRecord{}, fmt.Errorf("unsupported bundle version %q", bundle.Version)
	}
	envelope, err := base64.StdEncoding.DecodeString(bundle.Envelope)
	if err != nil {
		return model.ImportRecord{}, fmt.Errorf("decode bundle envelope: %w", err)
	}
	decoded, err := grid.DecodeEnvelope(envelope)
	if err != nil {
		return model.ImportRecord{}, err
	}
	if decoded.EnvelopeCID != bundle.Artifact.ArtifactCID {
		return model.ImportRecord{}, fmt.Errorf("bundle artifact cid mismatch: record=%s decoded=%s", bundle.Artifact.ArtifactCID, decoded.EnvelopeCID)
	}
	if decoded.ProtocolPCID != bundle.Artifact.ProtocolPCID {
		return model.ImportRecord{}, fmt.Errorf("bundle protocol pCID mismatch: record=%s decoded=%s", bundle.Artifact.ProtocolPCID, decoded.ProtocolPCID)
	}
	if decoded.PayloadCID != bundle.Artifact.PayloadCID {
		return model.ImportRecord{}, fmt.Errorf("bundle payload cid mismatch: record=%s decoded=%s", bundle.Artifact.PayloadCID, decoded.PayloadCID)
	}
	if decoded.ProofCID != bundle.Artifact.ProofCID {
		return model.ImportRecord{}, fmt.Errorf("bundle proof cid mismatch: record=%s decoded=%s", bundle.Artifact.ProofCID, decoded.ProofCID)
	}

	importRecord := model.ImportRecord{
		ImportedAt:       now.Format(time.RFC3339),
		Mode:             mode,
		SourcePath:       inPath,
		ArtifactCID:      bundle.Artifact.ArtifactCID,
		RelatedID:        bundle.Artifact.RelatedID,
		ProtocolPCID:     bundle.Artifact.ProtocolPCID,
		Signer:           bundle.Artifact.Signer,
		SupportInstalled: installSupport,
	}

	if installSupport {
		if bundle.Protocol != nil {
			if err := installProtocolSupport(root, *bundle.Protocol); err != nil {
				return model.ImportRecord{}, err
			}
			importRecord.InstalledProtocolPCID = bundle.Protocol.ProtocolPCID
		}
		if bundle.Signer != nil {
			if err := installSignerSupport(root, *bundle.Signer); err != nil {
				return model.ImportRecord{}, err
			}
			importRecord.InstalledSignerIdentity = bundle.Signer.Name
		}
	}
	if _, err := store.CAS.Put(envelope); err != nil {
		return model.ImportRecord{}, err
	}

	artifacts, err := store.LoadArtifacts()
	if err != nil {
		return model.ImportRecord{}, err
	}
	if existing, ok := artifactByCID(artifacts, bundle.Artifact.ArtifactCID); ok {
		if !sameArtifactRecord(existing, bundle.Artifact) {
			return model.ImportRecord{}, fmt.Errorf("artifact conflict for %s", bundle.Artifact.ArtifactCID)
		}
	} else {
		if err := store.AppendArtifact(bundle.Artifact, envelope); err != nil {
			return model.ImportRecord{}, err
		}
	}

	commitments, err := store.LoadLatestCommitments()
	if err != nil {
		return model.ImportRecord{}, err
	}
	if bundle.Commitment != nil {
		if current, ok := commitments[bundle.Commitment.CommitmentID]; ok {
			if !sameCommitment(current, *bundle.Commitment) {
				return model.ImportRecord{}, fmt.Errorf("commitment conflict for %s", bundle.Commitment.CommitmentID)
			}
		} else {
			if err := store.AppendCommitment(*bundle.Commitment); err != nil {
				return model.ImportRecord{}, err
			}
			commitments[bundle.Commitment.CommitmentID] = *bundle.Commitment
		}
	}

	if bundle.Evidence != nil {
		existingEvidence, err := store.LoadEvidence()
		if err != nil {
			return model.ImportRecord{}, err
		}
		if current, ok := evidenceByID(existingEvidence, bundle.Evidence.EvidenceID); ok {
			if !sameEvidence(current, *bundle.Evidence) {
				return model.ImportRecord{}, fmt.Errorf("evidence conflict for %s", bundle.Evidence.EvidenceID)
			}
		} else {
			if err := store.AppendEvidence(*bundle.Evidence); err != nil {
				return model.ImportRecord{}, err
			}
		}
	}

	if bundle.Assessment != nil {
		existingAssessments, err := store.LoadAssessments()
		if err != nil {
			return model.ImportRecord{}, err
		}
		if current, ok := assessmentByID(existingAssessments, bundle.Assessment.AssessmentID); ok {
			if !sameAssessment(current, *bundle.Assessment) {
				return model.ImportRecord{}, fmt.Errorf("assessment conflict for %s", bundle.Assessment.AssessmentID)
			}
		} else {
			commitmentRecord := model.Commitment{}
			if bundle.Commitment != nil {
				commitmentRecord = *bundle.Commitment
			} else if current, ok := commitments[bundle.Assessment.CommitmentID]; ok {
				commitmentRecord = current
			}
			if commitmentRecord.CommitmentID == "" {
				return model.ImportRecord{}, fmt.Errorf("assessment import requires related commitment projection")
			}
			if err := store.AppendAssessment(*bundle.Assessment, commitmentRecord); err != nil {
				return model.ImportRecord{}, err
			}
		}
	}

	if err := store.AppendImport(importRecord); err != nil {
		return model.ImportRecord{}, err
	}
	if announce {
		fmt.Printf("Imported %s from %s\n", emptyFallback(bundle.Artifact.RelatedID, bundle.Artifact.ArtifactCID), inPath)
		if installSupport {
			fmt.Println("Support material installed: yes")
		} else {
			fmt.Println("Support material installed: no")
		}
	}
	_ = registry
	return importRecord, nil
}

func sendBundleFilename(now time.Time, ref string) string {
	return fmt.Sprintf("%s-%s.json", now.UTC().Format("20060102T150405Z"), sanitizeFilename(ref))
}

func sanitizeFilename(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "artifact"
	}
	return b.String()
}

type inspectView struct {
	Reference          string               `json:"reference"`
	Kind               string               `json:"kind"`
	RelatedID          string               `json:"related_id,omitempty"`
	RelatedCID         string               `json:"related_cid,omitempty"`
	ArtifactCID        string               `json:"artifact_cid,omitempty"`
	ProtocolName       string               `json:"protocol_name,omitempty"`
	ProtocolPCID       string               `json:"protocol_pcid,omitempty"`
	ProtocolPath       string               `json:"protocol_path,omitempty"`
	Signer             string               `json:"signer,omitempty"`
	SignerKeyID        string               `json:"signer_key_id,omitempty"`
	SignerKeyState     string               `json:"signer_key_state,omitempty"`
	SignerIdentityPath string               `json:"signer_identity_path,omitempty"`
	PayloadCID         string               `json:"payload_cid,omitempty"`
	ProofCID           string               `json:"proof_cid,omitempty"`
	ObservedAt         string               `json:"observed_at,omitempty"`
	RecordPath         string               `json:"record_path,omitempty"`
	Details            []string             `json:"details,omitempty"`
	ConformanceEntries []changelog.Entry    `json:"conformance_entries,omitempty"`
	LatestImport       *model.ImportRecord  `json:"latest_import,omitempty"`
	RelatedImports     []model.ImportRecord `json:"related_imports,omitempty"`
}

type verifyResult struct {
	Reference           string              `json:"reference"`
	Kind                string              `json:"kind"`
	ArtifactCID         string              `json:"artifact_cid"`
	EnvelopeCIDVerified bool                `json:"envelope_cid_verified"`
	PayloadCIDVerified  bool                `json:"payload_cid_verified"`
	ProofCIDVerified    bool                `json:"proof_cid_verified"`
	SignatureVerified   bool                `json:"signature_verified"`
	SignerIdentityOK    bool                `json:"signer_identity_verified"`
	Signer              string              `json:"signer"`
	SignerKeyID         string              `json:"signer_key_id"`
	SignerKeyState      string              `json:"signer_key_state,omitempty"`
	LocalIdentityFile   string              `json:"local_identity_file,omitempty"`
	IdentitySource      string              `json:"identity_source,omitempty"`
	ProtocolPCID        string              `json:"protocol_pcid"`
	LocalProtocolMatch  bool                `json:"local_protocol_match"`
	ProtocolName        string              `json:"protocol_name,omitempty"`
	ProtocolPath        string              `json:"protocol_path,omitempty"`
	ProtocolSource      string              `json:"protocol_source,omitempty"`
	PayloadCID          string              `json:"payload_cid"`
	ProofCID            string              `json:"proof_cid"`
	ObservedAt          string              `json:"observed_at,omitempty"`
	LatestImport        *model.ImportRecord `json:"latest_import,omitempty"`
	TrustPolicyFile     string              `json:"trust_policy_file"`
	TrustPolicyLoaded   bool                `json:"trust_policy_loaded"`
	SignerTrusted       bool                `json:"signer_trusted"`
	SignerReason        string              `json:"signer_reason"`
	ProtocolTrusted     bool                `json:"protocol_trusted"`
	ProtocolReason      string              `json:"protocol_reason"`
	ImportSourceTrusted *bool               `json:"import_source_trusted,omitempty"`
	ImportReason        string              `json:"import_reason"`
	OverallTrusted      bool                `json:"overall_trusted"`
}

type provenanceRow struct {
	ImportedAt              string                 `json:"imported_at"`
	Mode                    string                 `json:"mode"`
	SourcePath              string                 `json:"source_path"`
	ArtifactCID             string                 `json:"artifact_cid"`
	RelatedID               string                 `json:"related_id,omitempty"`
	ProtocolPCID            string                 `json:"protocol_pcid"`
	Signer                  string                 `json:"signer,omitempty"`
	SupportInstalled        bool                   `json:"support_installed"`
	InstalledProtocolPCID   string                 `json:"installed_protocol_pcid,omitempty"`
	InstalledSignerIdentity string                 `json:"installed_signer_identity,omitempty"`
	ReceiptArtifactCID      string                 `json:"receipt_artifact_cid,omitempty"`
	Receipts                []model.ArtifactRecord `json:"receipts,omitempty"`
}

func inspectReference(root string, registry protocol.Registry, ref string, artifacts []model.ArtifactRecord, commitments map[string]model.Commitment, evidenceItems []model.Evidence, assessments []model.Assessment, imports []model.ImportRecord) (inspectView, error) {
	artifactByCID := artifactIndexByCID(artifacts)

	if item, ok := commitments[ref]; ok {
		return buildCommitmentInspectView(root, registry, ref, item, artifactByCID[item.ArtifactCID], commitments, evidenceItems, assessments, imports), nil
	}
	for _, item := range evidenceItems {
		if item.EvidenceID == ref {
			return buildEvidenceInspectView(root, registry, ref, item, artifactByCID[item.ArtifactCID], commitments, imports), nil
		}
	}
	for _, item := range assessments {
		if item.AssessmentID == ref {
			return buildAssessmentInspectView(root, registry, ref, item, artifactByCID[item.ArtifactCID], commitments, imports), nil
		}
	}
	if artifact, ok := artifactByCID[ref]; ok {
		switch artifact.Kind {
		case "commitment_promise":
			if item, ok := commitments[artifact.RelatedID]; ok {
				return buildCommitmentInspectView(root, registry, ref, item, artifact, commitments, evidenceItems, assessments, imports), nil
			}
		case "commitment_evidence":
			for _, item := range evidenceItems {
				if item.EvidenceID == artifact.RelatedID {
					return buildEvidenceInspectView(root, registry, ref, item, artifact, commitments, imports), nil
				}
			}
		case "commitment_assessment":
			for _, item := range assessments {
				if item.AssessmentID == artifact.RelatedID {
					return buildAssessmentInspectView(root, registry, ref, item, artifact, commitments, imports), nil
				}
			}
		}
		return buildArtifactInspectView(root, registry, ref, artifact, imports), nil
	}
	for _, artifact := range artifacts {
		if artifact.RelatedID == ref {
			return buildArtifactInspectView(root, registry, ref, artifact, imports), nil
		}
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

func artifactByCID(artifacts []model.ArtifactRecord, artifactCID string) (model.ArtifactRecord, bool) {
	for _, item := range artifacts {
		if item.ArtifactCID == artifactCID {
			return item, true
		}
	}
	return model.ArtifactRecord{}, false
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
	for _, artifact := range artifacts {
		if artifact.RelatedID == ref {
			return artifact, nil
		}
	}
	return model.ArtifactRecord{}, fmt.Errorf("unknown verify reference %q", ref)
}

func buildProvenanceRows(imports []model.ImportRecord, artifacts []model.ArtifactRecord, artifactCID string, sourcePath string, signer string, mode string) []provenanceRow {
	receipts := receiptArtifactsByImportedArtifact(artifacts)
	rows := make([]provenanceRow, 0, len(imports))
	for i := len(imports) - 1; i >= 0; i-- {
		record := imports[i]
		if artifactCID != "" && record.ArtifactCID != artifactCID {
			continue
		}
		if sourcePath != "" && record.SourcePath != sourcePath {
			continue
		}
		if signer != "" && record.Signer != signer {
			continue
		}
		if mode != "" && record.Mode != mode {
			continue
		}
		rows = append(rows, provenanceRow{
			ImportedAt:              record.ImportedAt,
			Mode:                    record.Mode,
			SourcePath:              record.SourcePath,
			ArtifactCID:             record.ArtifactCID,
			RelatedID:               record.RelatedID,
			ProtocolPCID:            record.ProtocolPCID,
			Signer:                  record.Signer,
			SupportInstalled:        record.SupportInstalled,
			InstalledProtocolPCID:   record.InstalledProtocolPCID,
			InstalledSignerIdentity: record.InstalledSignerIdentity,
			ReceiptArtifactCID:      record.ReceiptArtifactCID,
			Receipts:                receipts[record.ArtifactCID],
		})
	}
	return rows
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

func sameArtifactRecord(a model.ArtifactRecord, b model.ArtifactRecord) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func sameEvidence(a model.Evidence, b model.Evidence) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func sameAssessment(a model.Assessment, b model.Assessment) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func evidenceByID(items []model.Evidence, evidenceID string) (model.Evidence, bool) {
	for _, item := range items {
		if item.EvidenceID == evidenceID {
			return item, true
		}
	}
	return model.Evidence{}, false
}

func assessmentByID(items []model.Assessment, assessmentID string) (model.Assessment, bool) {
	for _, item := range items {
		if item.AssessmentID == assessmentID {
			return item, true
		}
	}
	return model.Assessment{}, false
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
	if existing, err := os.ReadFile(docPath); err == nil {
		if string(existing) != string(data) {
			return fmt.Errorf("protocol support conflict for %s", support.ProtocolPCID)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read imported protocol doc %q: %w", docPath, err)
	}
	if err := os.WriteFile(docPath, data, 0o644); err != nil {
		return fmt.Errorf("write imported protocol doc: %w", err)
	}
	metaPath := importedProtocolMetaPath(root, support.ProtocolPCID)
	meta, err := json.MarshalIndent(support, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal protocol support: %w", err)
	}
	if existing, err := os.ReadFile(metaPath); err == nil {
		if string(existing) != string(meta) {
			return fmt.Errorf("protocol support metadata conflict for %s", support.ProtocolPCID)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read imported protocol metadata %q: %w", metaPath, err)
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
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) != string(payload) {
			return fmt.Errorf("signer support conflict for %s", support.Name)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read imported identity %q: %w", path, err)
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

type identityLocation struct {
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

func protocolSource(location protocolLocation) string {
	if location.Imported {
		return "imported support"
	}
	return "built-in frozen doc"
}

func resolveIdentityLocation(root string, name string) identityLocation {
	primary := filepath.Join(root, identityPathForName(name))
	if _, err := os.Stat(primary); err == nil {
		return identityLocation{Path: primary, Matched: true}
	}
	imported := filepath.Join(root, "config", "imported-identities", importedIdentityFilename(name))
	if _, err := os.Stat(imported); err == nil {
		return identityLocation{Path: imported, Matched: true, Imported: true}
	}
	return identityLocation{Path: imported}
}

func (l identityLocation) Source() string {
	if l.Imported {
		return "imported support"
	}
	return "primary local identity"
}

func identitySourceLabel(match identityMatch) string {
	switch match.KeyState {
	case "archived":
		return "archived local identity"
	case "imported":
		return "imported support"
	default:
		return "primary local identity"
	}
}

func latestImportForArtifact(items []model.ImportRecord, artifactCID string) *model.ImportRecord {
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].ArtifactCID == artifactCID {
			record := items[i]
			return &record
		}
	}
	return nil
}

func relatedImportsForCommitment(commitmentID string, evidenceItems []model.Evidence, assessments []model.Assessment, imports []model.ImportRecord) []model.ImportRecord {
	artifactSet := map[string]struct{}{}
	for _, item := range evidenceItems {
		if item.CommitmentID == commitmentID && item.ArtifactCID != "" {
			artifactSet[item.ArtifactCID] = struct{}{}
		}
	}
	for _, item := range assessments {
		if item.CommitmentID == commitmentID && item.ArtifactCID != "" {
			artifactSet[item.ArtifactCID] = struct{}{}
		}
	}
	out := []model.ImportRecord{}
	for _, record := range imports {
		if _, ok := artifactSet[record.ArtifactCID]; ok {
			out = append(out, record)
		}
	}
	return out
}

func buildCommitmentInspectView(root string, registry protocol.Registry, ref string, item model.Commitment, artifact model.ArtifactRecord, commitments map[string]model.Commitment, evidenceItems []model.Evidence, assessments []model.Assessment, imports []model.ImportRecord) inspectView {
	view := buildArtifactInspectView(root, registry, ref, artifact, imports)
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
	view.RelatedImports = relatedImportsForCommitment(item.CommitmentID, evidenceItems, assessments, imports)
	return enrichInspectView(root, registry, view)
}

func buildEvidenceInspectView(root string, registry protocol.Registry, ref string, item model.Evidence, artifact model.ArtifactRecord, commitments map[string]model.Commitment, imports []model.ImportRecord) inspectView {
	view := buildArtifactInspectView(root, registry, ref, artifact, imports)
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
	return enrichInspectView(root, registry, view)
}

func buildAssessmentInspectView(root string, registry protocol.Registry, ref string, item model.Assessment, artifact model.ArtifactRecord, commitments map[string]model.Commitment, imports []model.ImportRecord) inspectView {
	view := buildArtifactInspectView(root, registry, ref, artifact, imports)
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
	return enrichInspectView(root, registry, view)
}

func buildArtifactInspectView(root string, registry protocol.Registry, ref string, artifact model.ArtifactRecord, imports []model.ImportRecord) inspectView {
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
		LatestImport: latestImportForArtifact(imports, artifact.ArtifactCID),
	}
	if artifact.Signer != "" && artifact.SignerKeyID != "" {
		if match, err := resolveIdentityMatch(root, artifact.Signer, artifact.SignerKeyID); err == nil {
			view.SignerKeyState = match.KeyState
			view.SignerIdentityPath = match.Info.Path
		}
	}
	return enrichInspectView(root, registry, view)
}

func enrichInspectView(root string, registry protocol.Registry, view inspectView) inspectView {
	if view.ProtocolPCID == "" {
		return view
	}
	location := resolveProtocolLocation(root, registry, view.ProtocolPCID)
	if location.Matched {
		view.ProtocolName = location.Name
		view.ProtocolPath = location.Path
	}
	if entries, err := changelog.MatchSpec(root, view.ProtocolPCID); err == nil {
		view.ConformanceEntries = entries
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
	if view.SignerKeyState != "" {
		fmt.Printf("Signer Key State: %s\n", view.SignerKeyState)
	}
	if view.SignerIdentityPath != "" {
		fmt.Printf("Signer Identity Path: %s\n", view.SignerIdentityPath)
	}
	fmt.Printf("Payload CID: %s\n", emptyFallback(view.PayloadCID, "(none)"))
	fmt.Printf("Proof CID: %s\n", emptyFallback(view.ProofCID, "(none)"))
	fmt.Printf("Observed At: %s\n", emptyFallback(view.ObservedAt, "(none)"))
	if view.RecordPath != "" {
		fmt.Printf("Record Path: %s\n", view.RecordPath)
	}
	if view.LatestImport != nil {
		fmt.Printf("Latest Import Mode: %s\n", view.LatestImport.Mode)
		fmt.Printf("Latest Import Source: %s\n", view.LatestImport.SourcePath)
		fmt.Printf("Latest Import At: %s\n", view.LatestImport.ImportedAt)
		fmt.Printf("Import Support Installed: %s\n", yesNo(view.LatestImport.SupportInstalled))
	}
	if len(view.RelatedImports) > 0 {
		fmt.Printf("Related Imported Artifacts: %d\n", len(view.RelatedImports))
		for _, record := range view.RelatedImports {
			fmt.Printf("Related Import: %s via %s from %s\n", emptyFallback(record.RelatedID, record.ArtifactCID), record.Mode, record.SourcePath)
		}
	}
	if len(view.ConformanceEntries) > 0 {
		for _, entry := range view.ConformanceEntries {
			fmt.Printf("Conformance Claim: %s (scope=%s, breaking-change=%s)\n", entry.Claim, entry.Scope, entry.BreakingChange)
			if entry.Notes != "" {
				fmt.Printf("Conformance Notes: %s\n", entry.Notes)
			}
		}
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

func buildVerifyResult(root string, registry protocol.Registry, artifact model.ArtifactRecord, decoded grid.DecodedArtifact, match identityMatch, latestImport *model.ImportRecord, policy trust.Policy) verifyResult {
	location := resolveProtocolLocation(root, registry, decoded.ProtocolPCID)
	identityLocation := identityLocation{Path: match.Info.Path, Matched: true, Imported: match.Info.Source == "imported"}
	evaluation := trust.Evaluate(policy, decoded.Proof.Signer, match.Info.Source == "primary", decoded.ProtocolPCID, location.Matched && !location.Imported, latestImport)
	out := verifyResult{
		Reference:           emptyFallback(artifact.RelatedID, artifact.ArtifactCID),
		Kind:                emptyFallback(artifact.Kind, "(unknown)"),
		ArtifactCID:         artifact.ArtifactCID,
		EnvelopeCIDVerified: true,
		PayloadCIDVerified:  true,
		ProofCIDVerified:    true,
		SignatureVerified:   true,
		SignerIdentityOK:    true,
		Signer:              decoded.Proof.Signer,
		SignerKeyID:         decoded.Proof.KeyID,
		SignerKeyState:      match.KeyState,
		LocalIdentityFile:   identityLocation.Path,
		ProtocolPCID:        decoded.ProtocolPCID,
		LocalProtocolMatch:  location.Matched,
		PayloadCID:          decoded.PayloadCID,
		ProofCID:            decoded.ProofCID,
		ObservedAt:          artifact.ObservedAt,
		LatestImport:        latestImport,
		TrustPolicyFile:     policy.Path,
		TrustPolicyLoaded:   policy.Found,
		SignerTrusted:       evaluation.SignerTrusted,
		SignerReason:        evaluation.SignerReason,
		ProtocolTrusted:     evaluation.ProtocolTrusted,
		ProtocolReason:      evaluation.ProtocolReason,
		ImportReason:        evaluation.ImportReason,
		OverallTrusted:      evaluation.OverallTrusted,
	}
	if identityLocation.Matched {
		out.IdentitySource = identitySourceLabel(match)
	}
	if location.Matched {
		out.ProtocolName = location.Name
		out.ProtocolPath = location.Path
		out.ProtocolSource = protocolSource(location)
	}
	if evaluation.ImportApplies {
		out.ImportSourceTrusted = boolPtr(evaluation.ImportTrusted)
	}
	return out
}

func printVerifyResult(result verifyResult) {
	fmt.Printf("Reference: %s\n", result.Reference)
	fmt.Printf("Kind: %s\n", result.Kind)
	fmt.Printf("Artifact CID: %s\n", result.ArtifactCID)
	fmt.Printf("Envelope CID Verified: %s\n", yesNo(result.EnvelopeCIDVerified))
	fmt.Printf("Payload CID Verified: %s\n", yesNo(result.PayloadCIDVerified))
	fmt.Printf("Proof CID Verified: %s\n", yesNo(result.ProofCIDVerified))
	fmt.Printf("Signature Verified: %s\n", yesNo(result.SignatureVerified))
	fmt.Printf("Signer Identity Verified: %s\n", yesNo(result.SignerIdentityOK))
	fmt.Printf("Signer: %s\n", result.Signer)
	fmt.Printf("Signer Key ID: %s\n", result.SignerKeyID)
	if result.SignerKeyState != "" {
		fmt.Printf("Signer Key State: %s\n", result.SignerKeyState)
	}
	fmt.Printf("Local Identity File: %s\n", result.LocalIdentityFile)
	if result.IdentitySource != "" {
		fmt.Printf("Identity Source: %s\n", result.IdentitySource)
	}
	fmt.Printf("Protocol pCID: %s\n", result.ProtocolPCID)
	if result.LocalProtocolMatch {
		fmt.Printf("Local Protocol Match: yes\n")
		fmt.Printf("Protocol: %s\n", result.ProtocolName)
		fmt.Printf("Protocol Doc: %s\n", result.ProtocolPath)
		fmt.Printf("Protocol Source: %s\n", result.ProtocolSource)
	} else {
		fmt.Printf("Local Protocol Match: no\n")
	}
	fmt.Printf("Payload CID: %s\n", result.PayloadCID)
	fmt.Printf("Proof CID: %s\n", result.ProofCID)
	fmt.Printf("Observed At: %s\n", emptyFallback(result.ObservedAt, "(none)"))
	if result.LatestImport != nil {
		fmt.Printf("Latest Import Mode: %s\n", result.LatestImport.Mode)
		fmt.Printf("Latest Import Source: %s\n", result.LatestImport.SourcePath)
		fmt.Printf("Latest Import At: %s\n", result.LatestImport.ImportedAt)
		fmt.Printf("Import Support Installed: %s\n", yesNo(result.LatestImport.SupportInstalled))
	}
	fmt.Printf("Trust Policy File: %s\n", result.TrustPolicyFile)
	fmt.Printf("Trust Policy Loaded: %s\n", yesNo(result.TrustPolicyLoaded))
	fmt.Printf("Signer Trusted: %s (%s)\n", yesNo(result.SignerTrusted), result.SignerReason)
	fmt.Printf("Protocol Trusted: %s (%s)\n", yesNo(result.ProtocolTrusted), result.ProtocolReason)
	if result.ImportSourceTrusted != nil {
		fmt.Printf("Import Source Trusted: %s (%s)\n", yesNo(*result.ImportSourceTrusted), result.ImportReason)
	} else {
		fmt.Printf("Import Source Trusted: n/a (%s)\n", result.ImportReason)
	}
	fmt.Printf("Overall Trust: %s\n", yesNo(result.OverallTrusted))
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

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func printExchangeStatus(policy trust.Policy, imports []model.ImportRecord, artifacts []model.ArtifactRecord) {
	summary := summarizeImports(policy, imports, artifacts)
	fmt.Printf("Total imports: %d\n", summary.Total)
	fmt.Printf("Unique imported artifacts: %d\n", summary.UniqueArtifacts)
	fmt.Printf("Unique import sources: %d\n", summary.UniqueSources)
	fmt.Printf("Support installed: %d\n", summary.SupportInstalled)
	fmt.Printf("Trusted imports: %d\n", summary.Trusted)
	fmt.Printf("Untrusted imports: %d\n", summary.Untrusted)
	fmt.Printf("Receipt artifacts: %d\n", summary.ReceiptArtifacts)
	fmt.Printf("Imported artifacts with receipts: %d\n", summary.ImportedArtifactsWithReceipts)
	for _, mode := range summary.Modes {
		fmt.Printf("Mode %s: %d\n", mode, summary.ByMode[mode])
	}
}

func printImportReport(policy trust.Policy, imports []model.ImportRecord, artifacts []model.ArtifactRecord) {
	summary := summarizeImports(policy, imports, artifacts)
	fmt.Printf("Total imports: %d\n", summary.Total)
	for _, source := range summary.Sources {
		item := summary.BySource[source]
		fmt.Printf("Source: %s\n", source)
		fmt.Printf("Imports: %d\n", item.Count)
		fmt.Printf("Trusted: %s\n", yesNo(item.Trusted))
		fmt.Printf("Modes: %s\n", strings.Join(item.Modes, ", "))
		fmt.Printf("Receipt Artifacts: %d\n", item.ReceiptArtifacts)
		if len(item.ReceiptSigners) > 0 {
			fmt.Printf("Receipt Signers: %s\n", strings.Join(item.ReceiptSigners, ", "))
		}
		fmt.Printf("Last Imported At: %s\n", item.LastImportedAt)
	}
}

type importSummary struct {
	Total                         int                            `json:"total"`
	UniqueArtifacts               int                            `json:"unique_artifacts"`
	UniqueSources                 int                            `json:"unique_sources"`
	SupportInstalled              int                            `json:"support_installed"`
	Trusted                       int                            `json:"trusted"`
	Untrusted                     int                            `json:"untrusted"`
	ReceiptArtifacts              int                            `json:"receipt_artifacts"`
	ImportedArtifactsWithReceipts int                            `json:"imported_artifacts_with_receipts"`
	ByMode                        map[string]int                 `json:"by_mode"`
	Modes                         []string                       `json:"modes"`
	BySource                      map[string]importSourceSummary `json:"by_source"`
	Sources                       []string                       `json:"sources"`
}

type importSourceSummary struct {
	Count            int      `json:"count"`
	Trusted          bool     `json:"trusted"`
	Modes            []string `json:"modes"`
	ReceiptArtifacts int      `json:"receipt_artifacts"`
	ReceiptSigners   []string `json:"receipt_signers,omitempty"`
	LastImportedAt   string   `json:"last_imported_at"`
}

func summarizeImports(policy trust.Policy, imports []model.ImportRecord, artifacts []model.ArtifactRecord) importSummary {
	out := importSummary{
		ByMode:   map[string]int{},
		BySource: map[string]importSourceSummary{},
	}
	artifactSet := map[string]struct{}{}
	sourceSet := map[string]struct{}{}
	modeSet := map[string]struct{}{}
	receiptArtifacts := receiptArtifactsByImportedArtifact(artifacts)
	for _, record := range imports {
		out.Total++
		if record.SupportInstalled {
			out.SupportInstalled++
		}
		if _, ok := artifactSet[record.ArtifactCID]; !ok {
			artifactSet[record.ArtifactCID] = struct{}{}
		}
		if _, ok := sourceSet[record.SourcePath]; !ok {
			sourceSet[record.SourcePath] = struct{}{}
		}
		out.ByMode[record.Mode]++
		modeSet[record.Mode] = struct{}{}
		evaluation := trust.Evaluate(policy, record.Signer, false, record.ProtocolPCID, false, &record)
		if evaluation.OverallTrusted {
			out.Trusted++
		} else {
			out.Untrusted++
		}
		sourceSummary := out.BySource[record.SourcePath]
		sourceSummary.Count++
		sourceSummary.Trusted = sourceSummary.Trusted || evaluation.OverallTrusted
		if !stringSliceContains(sourceSummary.Modes, record.Mode) {
			sourceSummary.Modes = append(sourceSummary.Modes, record.Mode)
			sort.Strings(sourceSummary.Modes)
		}
		if receipts := receiptArtifacts[record.ArtifactCID]; len(receipts) > 0 {
			sourceSummary.ReceiptArtifacts += len(receipts)
			signerSet := map[string]struct{}{}
			for _, receipt := range receipts {
				signerSet[receipt.Signer] = struct{}{}
			}
			sourceSummary.ReceiptSigners = sortedKeys(signerSet)
		}
		if record.ImportedAt > sourceSummary.LastImportedAt {
			sourceSummary.LastImportedAt = record.ImportedAt
		}
		out.BySource[record.SourcePath] = sourceSummary
	}
	receiptCounted := map[string]struct{}{}
	importedWithReceipts := map[string]struct{}{}
	for importedCID, receipts := range receiptArtifacts {
		if len(receipts) > 0 {
			importedWithReceipts[importedCID] = struct{}{}
		}
		for _, receipt := range receipts {
			if _, ok := receiptCounted[receipt.ArtifactCID]; ok {
				continue
			}
			receiptCounted[receipt.ArtifactCID] = struct{}{}
			out.ReceiptArtifacts++
		}
	}
	out.ImportedArtifactsWithReceipts = len(importedWithReceipts)
	out.UniqueArtifacts = len(artifactSet)
	out.UniqueSources = len(sourceSet)
	for mode := range modeSet {
		out.Modes = append(out.Modes, mode)
	}
	for source := range out.BySource {
		out.Sources = append(out.Sources, source)
	}
	sort.Strings(out.Modes)
	sort.Strings(out.Sources)
	return out
}

func receiptArtifactsByImportedArtifact(artifacts []model.ArtifactRecord) map[string][]model.ArtifactRecord {
	out := map[string][]model.ArtifactRecord{}
	for _, artifact := range artifacts {
		if artifact.Kind != "exchange_receipt" || artifact.RelatedCID == "" {
			continue
		}
		out[artifact.RelatedCID] = append(out[artifact.RelatedCID], artifact)
	}
	return out
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func boolPtr(value bool) *bool {
	return &value
}

func stringSliceContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

type doctorSummary struct {
	Artifacts           int      `json:"artifacts"`
	PrimaryIdentities   int      `json:"primary_identities"`
	ImportedIdentities  int      `json:"imported_identities"`
	ImportedProtocols   int      `json:"imported_protocols"`
	Warnings            []string `json:"warnings"`
	Errors              []string `json:"errors"`
	RepairableErrors    []string `json:"repairable_errors"`
	NonRepairableErrors []string `json:"non_repairable_errors"`
	RepairHints         []string `json:"repair_hints"`
}

func doctorReport(root string, store *ledger.Store, registry protocol.Registry) (doctorSummary, error) {
	summary := doctorSummary{
		Warnings:            []string{},
		Errors:              []string{},
		RepairableErrors:    []string{},
		NonRepairableErrors: []string{},
		RepairHints:         []string{},
	}
	imports, err := store.LoadImports()
	if err != nil {
		return summary, err
	}
	importedArtifacts := map[string]struct{}{}
	importedProtocols := map[string]struct{}{}
	importedSigners := map[string]struct{}{}
	for _, record := range imports {
		importedArtifacts[record.ArtifactCID] = struct{}{}
		if record.InstalledProtocolPCID != "" {
			importedProtocols[record.InstalledProtocolPCID] = struct{}{}
		}
		if record.InstalledSignerIdentity != "" {
			importedSigners[record.InstalledSignerIdentity] = struct{}{}
		}
	}
	artifacts, err := store.LoadArtifacts()
	if err != nil {
		return summary, err
	}
	summary.Artifacts = len(artifacts)
	for _, artifact := range artifacts {
		envelope, err := store.CAS.Get(artifact.ArtifactCID)
		if err != nil {
			message := fmt.Sprintf("artifact %s missing CAS bytes: %v", artifact.ArtifactCID, err)
			_, repairable := importedArtifacts[artifact.ArtifactCID]
			hint := ""
			if repairable {
				hint = "run repair --import-artifacts to restore missing imported artifact envelopes from saved bundle paths"
			}
			addDoctorError(&summary, message, repairable, hint)
			continue
		}
		decoded, err := grid.DecodeEnvelope(envelope)
		if err != nil {
			addDoctorError(&summary, fmt.Sprintf("artifact %s decode failed: %v", artifact.ArtifactCID, err), false, "")
			continue
		}
		if decoded.EnvelopeCID != artifact.ArtifactCID {
			addDoctorError(&summary, fmt.Sprintf("artifact %s envelope CID mismatch", artifact.ArtifactCID), false, "")
		}
		if decoded.ProtocolPCID != artifact.ProtocolPCID {
			addDoctorError(&summary, fmt.Sprintf("artifact %s protocol pCID mismatch", artifact.ArtifactCID), false, "")
		}
		if decoded.PayloadCID != artifact.PayloadCID {
			addDoctorError(&summary, fmt.Sprintf("artifact %s payload CID mismatch", artifact.ArtifactCID), false, "")
		}
		if decoded.ProofCID != artifact.ProofCID {
			addDoctorError(&summary, fmt.Sprintf("artifact %s proof CID mismatch", artifact.ArtifactCID), false, "")
		}
	}
	if err := doctorIdentityDir(root, filepath.Join(root, "config", "identities"), true, &summary, importedSigners); err != nil {
		return summary, err
	}
	if err := doctorIdentityDir(root, filepath.Join(root, "config", "imported-identities"), false, &summary, importedSigners); err != nil {
		return summary, err
	}
	if err := doctorImportedProtocols(root, &summary, importedProtocols); err != nil {
		return summary, err
	}
	checkExpectedImportedSupport(root, &summary, importedProtocols, importedSigners)
	if _, err := os.Stat(changelog.Path(root)); err != nil {
		if os.IsNotExist(err) {
			summary.Warnings = append(summary.Warnings, "CHANGELOG.md not found")
		} else {
			return summary, err
		}
	}
	_ = registry
	return summary, nil
}

func addDoctorError(summary *doctorSummary, message string, repairable bool, hint string) {
	summary.Errors = append(summary.Errors, message)
	if repairable {
		summary.RepairableErrors = append(summary.RepairableErrors, message)
	} else {
		summary.NonRepairableErrors = append(summary.NonRepairableErrors, message)
	}
	if hint != "" && !stringSliceContains(summary.RepairHints, hint) {
		summary.RepairHints = append(summary.RepairHints, hint)
	}
}

func doctorIdentityDir(root string, dir string, primary bool, summary *doctorSummary, importedSigners map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read identity dir %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if primary {
			if _, _, _, err := identity.Load(root, name); err != nil {
				addDoctorError(summary, fmt.Sprintf("primary identity %s invalid: %v", name, err), false, "")
				continue
			}
			summary.PrimaryIdentities++
			continue
		}
		if _, _, err := identity.LoadVerifier(root, name); err != nil {
			_, repairable := importedSigners[name]
			hint := ""
			if repairable {
				hint = "run repair --import-support to restore imported signer support files from saved bundle paths"
			}
			addDoctorError(summary, fmt.Sprintf("imported identity %s invalid: %v", name, err), repairable, hint)
			continue
		}
		summary.ImportedIdentities++
	}
	return nil
}

func doctorImportedProtocols(root string, summary *doctorSummary, importedProtocols map[string]struct{}) error {
	dir := filepath.Join(root, "data", "imported-protocols")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read imported protocol dir %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		pcid := strings.TrimSuffix(entry.Name(), ".json")
		support, err := loadImportedProtocolSupport(root, pcid)
		if err != nil {
			_, repairable := importedProtocols[pcid]
			hint := ""
			if repairable {
				hint = "run repair --import-support to restore imported protocol support files from saved bundle paths"
			}
			addDoctorError(summary, fmt.Sprintf("imported protocol %s metadata invalid: %v", pcid, err), repairable, hint)
			continue
		}
		data, err := os.ReadFile(importedProtocolDocPath(root, pcid))
		if err != nil {
			_, repairable := importedProtocols[pcid]
			hint := ""
			if repairable {
				hint = "run repair --import-support to restore imported protocol support files from saved bundle paths"
			}
			addDoctorError(summary, fmt.Sprintf("imported protocol %s doc missing: %v", pcid, err), repairable, hint)
			continue
		}
		if got := protocol.SupportPCID(data); got != support.ProtocolPCID {
			_, repairable := importedProtocols[pcid]
			hint := ""
			if repairable {
				hint = "run repair --import-support to restore imported protocol support files from saved bundle paths"
			}
			addDoctorError(summary, fmt.Sprintf("imported protocol %s pCID mismatch", pcid), repairable, hint)
			continue
		}
		summary.ImportedProtocols++
	}
	return nil
}

func checkExpectedImportedSupport(root string, summary *doctorSummary, importedProtocols map[string]struct{}, importedSigners map[string]struct{}) {
	for pcid := range importedProtocols {
		metaPath := importedProtocolMetaPath(root, pcid)
		docPath := importedProtocolDocPath(root, pcid)
		if _, err := os.Stat(metaPath); err != nil || statFileMissing(docPath) {
			addDoctorError(summary, fmt.Sprintf("imported protocol %s support files missing", pcid), true, "run repair --import-support to restore imported protocol support files from saved bundle paths")
		}
	}
	for name := range importedSigners {
		path := filepath.Join(root, "config", "imported-identities", importedIdentityFilename(name))
		if statFileMissing(path) {
			addDoctorError(summary, fmt.Sprintf("imported identity %s support file missing", name), true, "run repair --import-support to restore imported signer support files from saved bundle paths")
		}
	}
}

func statFileMissing(path string) bool {
	_, err := os.Stat(path)
	return err != nil
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

func emitExchangeReceiptArtifact(root string, store *ledger.Store, registry protocol.Registry, record model.ImportRecord, receiver string, now time.Time) (string, error) {
	ident, pub, priv, err := identity.LoadOrCreate(root, receiver)
	if err != nil {
		return "", err
	}
	receiptID := fmt.Sprintf("RECEIPT-%s-%s", now.Format("20060102"), sanitizeFilename(record.ArtifactCID))
	payloadBytes, err := protocol.MarshalPayload(protocol.ExchangeReceiptPayload{
		Kind:                    "exchange_receipt",
		ReceiptID:               receiptID,
		ReceivedArtifactCID:     record.ArtifactCID,
		RelatedID:               record.RelatedID,
		SourcePath:              record.SourcePath,
		Receiver:                ident.Name,
		ReceivedAt:              now.Format(time.RFC3339),
		SupportInstalled:        record.SupportInstalled,
		InstalledProtocolPCID:   record.InstalledProtocolPCID,
		InstalledSignerIdentity: record.InstalledSignerIdentity,
	})
	if err != nil {
		return "", err
	}
	artifact, err := grid.Build(registry.MustPCID(protocol.ExchangeReceipt), payloadBytes, ident.Name, ident.KeyID, pub, priv)
	if err != nil {
		return "", err
	}
	if err := store.AppendArtifact(model.ArtifactRecord{
		ArtifactCID:  artifact.EnvelopeCID,
		ProtocolPCID: artifact.ProtocolPCID,
		Kind:         "exchange_receipt",
		Signer:       ident.Name,
		SignerKeyID:  ident.KeyID,
		PayloadCID:   artifact.PayloadCID,
		ProofCID:     artifact.ProofCID,
		ObservedAt:   now.Format(time.RFC3339),
		RelatedID:    receiptID,
		RelatedCID:   record.ArtifactCID,
	}, artifact.Envelope); err != nil {
		return "", err
	}
	return artifact.EnvelopeCID, nil
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
		registry.MustPCID(protocol.ExchangeReceipt),
		registry.MustPCID(protocol.ImplementationConformance),
	}
	emitted := []string{
		registry.MustPCID(protocol.CommitmentPromise),
		registry.MustPCID(protocol.CommitmentEvidence),
		registry.MustPCID(protocol.CommitmentAssessment),
		registry.MustPCID(protocol.ExchangeReceipt),
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
			"receive may emit local exchange_receipt artifacts acknowledging imported bundle processing.",
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
