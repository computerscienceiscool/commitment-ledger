package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"commitment-ledger/internal/cas"
	"commitment-ledger/internal/model"
)

type Store struct {
	Root string
	CAS  *cas.Store
}

func NewStore(root string) *Store {
	return &Store{
		Root: root,
		CAS:  cas.New(root),
	}
}

func (s *Store) dataPath(name string) string {
	return filepath.Join(s.Root, "data", name)
}

func (s *Store) recordPath(dir string, id string) string {
	return filepath.Join(s.Root, "records", dir, id+".md")
}

func (s *Store) AppendWorkItems(items []model.WorkItem) error {
	current, err := s.LoadLatestWorkItems()
	if err != nil {
		return err
	}
	for _, item := range items {
		target := model.WorkTarget(item.Repo, item.Branch, item.WorkID)
		if item.Removed {
			delete(current, target)
			continue
		}
		current[target] = item
	}
	if err := s.persistWorkItemsIndex(current); err != nil {
		return err
	}
	for _, item := range items {
		if err := appendJSONL(s.dataPath("work_items.jsonl"), item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) LoadLatestWorkItems() (map[string]model.WorkItem, error) {
	if items, ok, err := s.loadWorkItemsIndex(); err != nil {
		return nil, err
	} else if ok {
		return items, nil
	}
	items := map[string]model.WorkItem{}
	err := readJSONL(s.dataPath("work_items.jsonl"), func(line []byte) error {
		var item model.WorkItem
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode work item: %w", err)
		}
		target := model.WorkTarget(item.Repo, item.Branch, item.WorkID)
		if item.Removed {
			delete(items, target)
			return nil
		}
		items[target] = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistWorkItemsIndex(items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) AppendSnapshot(snapshot model.Snapshot) error {
	current, err := s.LoadLatestSnapshots()
	if err != nil {
		return err
	}
	current[snapshot.Repo+"/"+snapshot.Branch] = snapshot
	if err := s.persistSnapshotsIndex(current); err != nil {
		return err
	}
	return appendJSONL(s.dataPath("snapshots.jsonl"), snapshot)
}

func (s *Store) AppendArtifact(record model.ArtifactRecord, envelope []byte) error {
	storedCID, err := s.CAS.Put(envelope)
	if err != nil {
		return err
	}
	if storedCID != record.ArtifactCID {
		return fmt.Errorf("cas stored cid %q does not match artifact record cid %q", storedCID, record.ArtifactCID)
	}
	items, err := s.LoadArtifacts()
	if err != nil {
		return err
	}
	items = append(items, record)
	if err := s.persistArtifactsIndex(items); err != nil {
		return err
	}
	return appendJSONL(s.dataPath("artifacts.jsonl"), record)
}

func (s *Store) LoadArtifacts() ([]model.ArtifactRecord, error) {
	if items, ok, err := s.loadArtifactsIndex(); err != nil {
		return nil, err
	} else if ok {
		return items, nil
	}
	var items []model.ArtifactRecord
	err := readJSONL(s.dataPath("artifacts.jsonl"), func(line []byte) error {
		var item model.ArtifactRecord
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode artifact: %w", err)
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistArtifactsIndex(items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) AppendImport(record model.ImportRecord) error {
	items, err := s.LoadImports()
	if err != nil {
		return err
	}
	items = append(items, record)
	if err := s.persistImportsIndex(items); err != nil {
		return err
	}
	return appendJSONL(s.dataPath("imports.jsonl"), record)
}

func (s *Store) LoadImports() ([]model.ImportRecord, error) {
	if items, ok, err := s.loadImportsIndex(); err != nil {
		return nil, err
	} else if ok {
		return items, nil
	}
	var items []model.ImportRecord
	err := readJSONL(s.dataPath("imports.jsonl"), func(line []byte) error {
		var item model.ImportRecord
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode import: %w", err)
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistImportsIndex(items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) LoadLatestSnapshots() (map[string]model.Snapshot, error) {
	if snapshots, ok, err := s.loadSnapshotsIndex(); err != nil {
		return nil, err
	} else if ok {
		return snapshots, nil
	}
	snapshots := map[string]model.Snapshot{}
	err := readJSONL(s.dataPath("snapshots.jsonl"), func(line []byte) error {
		var item model.Snapshot
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode snapshot: %w", err)
		}
		snapshots[item.Repo+"/"+item.Branch] = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistSnapshotsIndex(snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (s *Store) AppendCommitment(commitment model.Commitment) error {
	current, err := s.LoadLatestCommitments()
	if err != nil {
		return err
	}
	current[commitment.CommitmentID] = commitment
	if err := s.persistCommitmentsIndex(current); err != nil {
		return err
	}
	if err := appendJSONL(s.dataPath("commitments.jsonl"), commitment); err != nil {
		return err
	}
	return s.WriteCommitmentRecord(commitment)
}

func (s *Store) LoadLatestCommitments() (map[string]model.Commitment, error) {
	if commitments, ok, err := s.loadCommitmentsIndex(); err != nil {
		return nil, err
	} else if ok {
		return commitments, nil
	}
	commitments := map[string]model.Commitment{}
	err := readJSONL(s.dataPath("commitments.jsonl"), func(line []byte) error {
		var item model.Commitment
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode commitment: %w", err)
		}
		commitments[item.CommitmentID] = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistCommitmentsIndex(commitments); err != nil {
		return nil, err
	}
	return commitments, nil
}

func (s *Store) AppendEvidence(evidence model.Evidence) error {
	existingEvidence, err := s.LoadEvidence()
	if err != nil {
		return err
	}
	existingEvidence = append(existingEvidence, evidence)
	if err := s.persistEvidenceIndex(existingEvidence); err != nil {
		return err
	}
	if err := appendJSONL(s.dataPath("evidence.jsonl"), evidence); err != nil {
		return err
	}
	commitments, err := s.LoadLatestCommitments()
	if err != nil {
		return err
	}
	commitment, ok := commitments[evidence.CommitmentID]
	if !ok {
		return nil
	}
	return writeTextFile(s.recordPath("commitments", commitment.CommitmentID), CommitmentMarkdown(commitment, filterEvidenceForCommitment(existingEvidence, evidence.CommitmentID)))
}

func (s *Store) LoadEvidence() ([]model.Evidence, error) {
	if items, ok, err := s.loadEvidenceIndex(); err != nil {
		return nil, err
	} else if ok {
		return items, nil
	}
	var items []model.Evidence
	err := readJSONL(s.dataPath("evidence.jsonl"), func(line []byte) error {
		var item model.Evidence
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode evidence: %w", err)
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistEvidenceIndex(items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) LoadEvidenceForCommitment(commitmentID string) ([]model.Evidence, error) {
	items, err := s.LoadEvidence()
	if err != nil {
		return nil, err
	}
	return filterEvidenceForCommitment(items, commitmentID), nil
}

func (s *Store) AppendAssessment(assessment model.Assessment, commitment model.Commitment) error {
	items, err := s.LoadAssessments()
	if err != nil {
		return err
	}
	items = append(items, assessment)
	if err := s.persistAssessmentsIndex(items); err != nil {
		return err
	}
	if err := appendJSONL(s.dataPath("assessments.jsonl"), assessment); err != nil {
		return err
	}
	if err := writeTextFile(s.recordPath("assessments", assessment.AssessmentID), AssessmentMarkdown(assessment, commitment)); err != nil {
		return err
	}
	return s.AppendCommitment(commitment)
}

func (s *Store) LoadAssessments() ([]model.Assessment, error) {
	if items, ok, err := s.loadAssessmentsIndex(); err != nil {
		return nil, err
	} else if ok {
		return items, nil
	}
	var items []model.Assessment
	err := readJSONL(s.dataPath("assessments.jsonl"), func(line []byte) error {
		var item model.Assessment
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode assessment: %w", err)
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistAssessmentsIndex(items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) WriteCommitmentRecord(commitment model.Commitment) error {
	evidence, err := s.LoadEvidenceForCommitment(commitment.CommitmentID)
	if err != nil {
		return err
	}
	return writeTextFile(s.recordPath("commitments", commitment.CommitmentID), CommitmentMarkdown(commitment, evidence))
}

func filterEvidenceForCommitment(items []model.Evidence, commitmentID string) []model.Evidence {
	filtered := make([]model.Evidence, 0, len(items))
	for _, item := range items {
		if item.CommitmentID == commitmentID {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].ObservedAt != filtered[j].ObservedAt {
			return filtered[i].ObservedAt < filtered[j].ObservedAt
		}
		return filtered[i].EvidenceID < filtered[j].EvidenceID
	})
	return filtered
}

func writeBytesFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func writeTextFile(path string, body string) error {
	return writeBytesFile(path, []byte(body))
}
