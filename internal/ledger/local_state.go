package ledger

import (
	"fmt"
	"os"
	"strings"

	"commitment-ledger/internal/cid"
)

type localStateDescriptor struct {
	IndexName string
	FamilySet string
	Source    string
}

var localStateDescriptors = []localStateDescriptor{
	{IndexName: "work-items-latest", FamilySet: workObservationReferenceSet, Source: "work_items.jsonl"},
	{IndexName: "snapshots-latest", FamilySet: workObservationReferenceSet, Source: "snapshots.jsonl"},
	{IndexName: "commitments-latest", FamilySet: commitmentStateReferenceSet, Source: "commitments.jsonl"},
	{IndexName: "evidence-log", FamilySet: commitmentStateReferenceSet, Source: "evidence.jsonl"},
	{IndexName: "assessments-log", FamilySet: commitmentStateReferenceSet, Source: "assessments.jsonl"},
	{IndexName: "artifacts-log", FamilySet: artifactExchangeReferenceSet, Source: "artifacts.jsonl"},
	{IndexName: "imports-log", FamilySet: artifactExchangeReferenceSet, Source: "imports.jsonl"},
}

func (s *Store) MissingLocalStateIssues() ([]string, error) {
	var issues []string
	trackedFamilies := map[string]bool{}
	familyMissingPaths := map[string]bool{}
	for _, descriptor := range localStateDescriptors {
		shouldTrack, err := s.shouldTrackLocalState(descriptor)
		if err != nil {
			return nil, err
		}
		if !shouldTrack {
			continue
		}
		trackedFamilies[descriptor.FamilySet] = true
		var missing []string
		for _, path := range s.indexLocalStatePaths(descriptor.IndexName) {
			if fileMissing(path) {
				missing = append(missing, path)
			}
		}
		if len(missing) > 0 {
			issues = append(issues, fmt.Sprintf("index %s missing local state files: %s", descriptor.IndexName, strings.Join(missing, ", ")))
		}
	}
	for familySet := range trackedFamilies {
		var missing []string
		for _, path := range s.referenceSetLocalStatePaths(familySet) {
			if fileMissing(path) {
				missing = append(missing, path)
			}
		}
		if len(missing) > 0 {
			familyMissingPaths[familySet] = true
			issues = append(issues, fmt.Sprintf("reference set %s missing local state files: %s", familySet, strings.Join(missing, ", ")))
		}
	}
	for _, descriptor := range localStateDescriptors {
		shouldTrack, err := s.shouldTrackLocalState(descriptor)
		if err != nil {
			return nil, err
		}
		if !shouldTrack || familyMissingPaths[descriptor.FamilySet] {
			continue
		}
		set, ok, err := s.loadReferenceSet(descriptor.FamilySet)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		entry, ok := set.Entries[descriptor.IndexName]
		if !ok || entry.CID == "" {
			issues = append(issues, fmt.Sprintf("reference set %s missing entry for %s", descriptor.FamilySet, descriptor.IndexName))
			continue
		}
		coherenceIssues, err := s.localStateCoherenceIssues(descriptor, entry.CID)
		if err != nil {
			return nil, err
		}
		issues = append(issues, coherenceIssues...)
	}
	return issues, nil
}

func (s *Store) localStateCoherenceIssues(descriptor localStateDescriptor, expectedCID string) ([]string, error) {
	var issues []string
	refBody, err := os.ReadFile(s.refPath(descriptor.IndexName + ".ref"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s ref: %w", descriptor.IndexName, err)
	}
	if err == nil {
		got := string(trimTrailingWhitespace(refBody))
		if got != "" && got != expectedCID {
			issues = append(issues, fmt.Sprintf("index %s loose ref CID %s does not match reference set CID %s", descriptor.IndexName, got, expectedCID))
		}
	}
	for _, cachePath := range []string{s.structuredIndexPath(descriptor.IndexName), s.legacyIndexPath(descriptor.IndexName)} {
		cacheBody, err := os.ReadFile(cachePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s cache %s: %w", descriptor.IndexName, cachePath, err)
		}
		got := cid.Sum(cacheBody)
		if got != expectedCID {
			issues = append(issues, fmt.Sprintf("index %s cache %s CID %s does not match reference set CID %s", descriptor.IndexName, cachePath, got, expectedCID))
		}
	}
	return issues, nil
}

func (s *Store) RebuildLocalState() (int, error) {
	rebuilt := 0
	for _, descriptor := range localStateDescriptors {
		shouldTrack, err := s.shouldTrackLocalState(descriptor)
		if err != nil {
			return rebuilt, err
		}
		if !shouldTrack {
			continue
		}
		if err := s.rebuildLocalStateDescriptor(descriptor); err != nil {
			return rebuilt, err
		}
		rebuilt++
	}
	return rebuilt, nil
}

func (s *Store) shouldTrackLocalState(descriptor localStateDescriptor) (bool, error) {
	if nonEmpty, err := s.sourceHasData(descriptor.Source); err != nil {
		return false, err
	} else if nonEmpty {
		return true, nil
	}
	for _, path := range s.indexLocalStatePaths(descriptor.IndexName) {
		if !fileMissing(path) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) sourceHasData(name string) (bool, error) {
	info, err := os.Stat(s.dataPath(name))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", name, err)
	}
	return info.Size() > 0, nil
}

func (s *Store) indexLocalStatePaths(indexName string) []string {
	return []string{
		s.refPath(indexName + ".ref"),
		s.structuredIndexPath(indexName),
		s.legacyIndexPath(indexName),
	}
}

func (s *Store) referenceSetLocalStatePaths(setName string) []string {
	return []string{
		s.referenceSetRefPath(setName),
		s.referenceSetCachePath(setName),
	}
}

func (s *Store) rebuildLocalStateDescriptor(descriptor localStateDescriptor) error {
	switch descriptor.IndexName {
	case "work-items-latest":
		items, err := s.LoadLatestWorkItems()
		if err != nil {
			return err
		}
		return s.persistWorkItemsIndex(items)
	case "snapshots-latest":
		items, err := s.LoadLatestSnapshots()
		if err != nil {
			return err
		}
		return s.persistSnapshotsIndex(items)
	case "commitments-latest":
		items, err := s.LoadLatestCommitments()
		if err != nil {
			return err
		}
		return s.persistCommitmentsIndex(items)
	case "evidence-log":
		items, err := s.LoadEvidence()
		if err != nil {
			return err
		}
		return s.persistEvidenceIndex(items)
	case "assessments-log":
		items, err := s.LoadAssessments()
		if err != nil {
			return err
		}
		return s.persistAssessmentsIndex(items)
	case "artifacts-log":
		items, err := s.LoadArtifacts()
		if err != nil {
			return err
		}
		return s.persistArtifactsIndex(items)
	case "imports-log":
		items, err := s.LoadImports()
		if err != nil {
			return err
		}
		return s.persistImportsIndex(items)
	default:
		return fmt.Errorf("unknown local state descriptor %q", descriptor.IndexName)
	}
}

func fileMissing(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}
