package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"commitment-ledger/internal/assessment"
	"commitment-ledger/internal/commitment"
	"commitment-ledger/internal/config"
	"commitment-ledger/internal/evidence"
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
	fmt.Println("usage: commitment-ledger <scan|status|commit|evidence|assess|conformance|expire|report> [flags]")
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
		if err := store.AppendWorkItems(items); err != nil {
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
	claimed := []string{
		registry.MustPCID(protocol.CommitmentPromise),
		registry.MustPCID(protocol.CommitmentEvidence),
		registry.MustPCID(protocol.CommitmentAssessment),
		registry.MustPCID(protocol.ImplementationConformance),
	}
	payloadBytes, err := protocol.MarshalPayload(protocol.ImplementationConformancePayload{
		Kind:                 "implementation_conformance",
		Implementation:       "commitment-ledger",
		Version:              version,
		ClaimedProtocolPCIDs: claimed,
		ProjectionRules: []string{
			"JSONL files are append-only local indexes over artifact history.",
			"Markdown records are human-readable projections retaining artifact CIDs and protocol pCIDs.",
		},
		ClaimedAt: now.Format(time.RFC3339),
	})
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

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}
