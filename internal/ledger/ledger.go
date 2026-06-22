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
	for _, item := range items {
		if err := appendJSONL(s.dataPath("work_items.jsonl"), item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) LoadLatestWorkItems() (map[string]model.WorkItem, error) {
	items := map[string]model.WorkItem{}
	err := readJSONL(s.dataPath("work_items.jsonl"), func(line []byte) error {
		var item model.WorkItem
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode work item: %w", err)
		}
		items[model.WorkTarget(item.Repo, item.Branch, item.WorkID)] = item
		return nil
	})
	return items, err
}

func (s *Store) AppendSnapshot(snapshot model.Snapshot) error {
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
	return appendJSONL(s.dataPath("artifacts.jsonl"), record)
}

func (s *Store) LoadArtifacts() ([]model.ArtifactRecord, error) {
	var items []model.ArtifactRecord
	err := readJSONL(s.dataPath("artifacts.jsonl"), func(line []byte) error {
		var item model.ArtifactRecord
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode artifact: %w", err)
		}
		items = append(items, item)
		return nil
	})
	return items, err
}

func (s *Store) LoadLatestSnapshots() (map[string]model.Snapshot, error) {
	snapshots := map[string]model.Snapshot{}
	err := readJSONL(s.dataPath("snapshots.jsonl"), func(line []byte) error {
		var item model.Snapshot
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode snapshot: %w", err)
		}
		snapshots[item.Repo+"/"+item.Branch] = item
		return nil
	})
	return snapshots, err
}

func (s *Store) AppendCommitment(commitment model.Commitment) error {
	if err := appendJSONL(s.dataPath("commitments.jsonl"), commitment); err != nil {
		return err
	}
	return s.WriteCommitmentRecord(commitment)
}

func (s *Store) LoadLatestCommitments() (map[string]model.Commitment, error) {
	commitments := map[string]model.Commitment{}
	err := readJSONL(s.dataPath("commitments.jsonl"), func(line []byte) error {
		var item model.Commitment
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode commitment: %w", err)
		}
		commitments[item.CommitmentID] = item
		return nil
	})
	return commitments, err
}

func (s *Store) AppendEvidence(evidence model.Evidence) error {
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
	ev, err := s.LoadEvidenceForCommitment(evidence.CommitmentID)
	if err != nil {
		return err
	}
	return writeTextFile(s.recordPath("commitments", commitment.CommitmentID), CommitmentMarkdown(commitment, ev))
}

func (s *Store) LoadEvidence() ([]model.Evidence, error) {
	var items []model.Evidence
	err := readJSONL(s.dataPath("evidence.jsonl"), func(line []byte) error {
		var item model.Evidence
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode evidence: %w", err)
		}
		items = append(items, item)
		return nil
	})
	return items, err
}

func (s *Store) LoadEvidenceForCommitment(commitmentID string) ([]model.Evidence, error) {
	items, err := s.LoadEvidence()
	if err != nil {
		return nil, err
	}
	filtered := make([]model.Evidence, 0, len(items))
	for _, item := range items {
		if item.CommitmentID == commitmentID {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ObservedAt < filtered[j].ObservedAt
	})
	return filtered, nil
}

func (s *Store) AppendAssessment(assessment model.Assessment, commitment model.Commitment) error {
	if err := appendJSONL(s.dataPath("assessments.jsonl"), assessment); err != nil {
		return err
	}
	if err := writeTextFile(s.recordPath("assessments", assessment.AssessmentID), AssessmentMarkdown(assessment, commitment)); err != nil {
		return err
	}
	return s.AppendCommitment(commitment)
}

func (s *Store) LoadAssessments() ([]model.Assessment, error) {
	var items []model.Assessment
	err := readJSONL(s.dataPath("assessments.jsonl"), func(line []byte) error {
		var item model.Assessment
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("decode assessment: %w", err)
		}
		items = append(items, item)
		return nil
	})
	return items, err
}

func (s *Store) WriteCommitmentRecord(commitment model.Commitment) error {
	evidence, err := s.LoadEvidenceForCommitment(commitment.CommitmentID)
	if err != nil {
		return err
	}
	return writeTextFile(s.recordPath("commitments", commitment.CommitmentID), CommitmentMarkdown(commitment, evidence))
}

func writeTextFile(path string, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}
