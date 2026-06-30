package ledger

import (
	"fmt"
	"os"
	"strings"
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
			issues = append(issues, fmt.Sprintf("reference set %s missing local state files: %s", familySet, strings.Join(missing, ", ")))
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
