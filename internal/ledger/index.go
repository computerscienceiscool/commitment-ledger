package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"commitment-ledger/internal/model"
)

type casIndexSnapshot[T any] struct {
	Version string `json:"version"`
	Items   []T    `json:"items"`
}

func (s *Store) refPath(parts ...string) string {
	segments := append([]string{s.Root, "data", "refs"}, parts...)
	return filepath.Join(segments...)
}

func (s *Store) indexPath(parts ...string) string {
	segments := append([]string{s.Root, "data", "indexes"}, parts...)
	return filepath.Join(segments...)
}

func (s *Store) persistIndex(name string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s index: %w", name, err)
	}
	id, err := s.CAS.Put(body)
	if err != nil {
		return fmt.Errorf("store %s index in cas: %w", name, err)
	}
	if err := writeBytesFile(s.refPath(name+".ref"), []byte(id+"\n")); err != nil {
		return fmt.Errorf("write %s ref: %w", name, err)
	}
	if err := writeBytesFile(s.indexPath(name+".json"), body); err != nil {
		return fmt.Errorf("write %s cache: %w", name, err)
	}
	return nil
}

func (s *Store) loadIndex(name string, out any) (bool, error) {
	refBody, err := os.ReadFile(s.refPath(name + ".ref"))
	if err == nil {
		id := strings.TrimSpace(string(refBody))
		if id != "" {
			data, casErr := s.CAS.Get(id)
			if casErr == nil {
				if err := json.Unmarshal(data, out); err != nil {
					return false, fmt.Errorf("decode %s cas index %q: %w", name, id, err)
				}
				return true, nil
			}
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s ref: %w", name, err)
	}

	cacheBody, err := os.ReadFile(s.indexPath(name + ".json"))
	if err == nil {
		if err := json.Unmarshal(cacheBody, out); err != nil {
			return false, fmt.Errorf("decode %s cache: %w", name, err)
		}
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s cache: %w", name, err)
	}
	return false, nil
}

func sortedWorkItems(items map[string]model.WorkItem) []model.WorkItem {
	out := make([]model.WorkItem, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		left := model.WorkTarget(out[i].Repo, out[i].Branch, out[i].WorkID)
		right := model.WorkTarget(out[j].Repo, out[j].Branch, out[j].WorkID)
		return left < right
	})
	return out
}

func sortedSnapshots(items map[string]model.Snapshot) []model.Snapshot {
	out := make([]model.Snapshot, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i].Repo + "/" + out[i].Branch
		right := out[j].Repo + "/" + out[j].Branch
		return left < right
	})
	return out
}

func sortedCommitments(items map[string]model.Commitment) []model.Commitment {
	out := make([]model.Commitment, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CommitmentID < out[j].CommitmentID
	})
	return out
}

func sortedEvidence(items []model.Evidence) []model.Evidence {
	out := append([]model.Evidence(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ObservedAt != out[j].ObservedAt {
			return out[i].ObservedAt < out[j].ObservedAt
		}
		return out[i].EvidenceID < out[j].EvidenceID
	})
	return out
}

func sortedAssessments(items []model.Assessment) []model.Assessment {
	out := append([]model.Assessment(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].AssessedAt != out[j].AssessedAt {
			return out[i].AssessedAt < out[j].AssessedAt
		}
		return out[i].AssessmentID < out[j].AssessmentID
	})
	return out
}

func sortedArtifacts(items []model.ArtifactRecord) []model.ArtifactRecord {
	out := append([]model.ArtifactRecord(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ObservedAt != out[j].ObservedAt {
			return out[i].ObservedAt < out[j].ObservedAt
		}
		return out[i].ArtifactCID < out[j].ArtifactCID
	})
	return out
}

func sortedImports(items []model.ImportRecord) []model.ImportRecord {
	out := append([]model.ImportRecord(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ImportedAt != out[j].ImportedAt {
			return out[i].ImportedAt < out[j].ImportedAt
		}
		if out[i].ArtifactCID != out[j].ArtifactCID {
			return out[i].ArtifactCID < out[j].ArtifactCID
		}
		return out[i].SourcePath < out[j].SourcePath
	})
	return out
}

func (s *Store) persistWorkItemsIndex(items map[string]model.WorkItem) error {
	return s.persistIndex("work-items-latest", casIndexSnapshot[model.WorkItem]{
		Version: "cas-index-v1",
		Items:   sortedWorkItems(items),
	})
}

func (s *Store) loadWorkItemsIndex() (map[string]model.WorkItem, bool, error) {
	var snapshot casIndexSnapshot[model.WorkItem]
	ok, err := s.loadIndex("work-items-latest", &snapshot)
	if err != nil || !ok {
		return nil, ok, err
	}
	items := map[string]model.WorkItem{}
	for _, item := range snapshot.Items {
		items[model.WorkTarget(item.Repo, item.Branch, item.WorkID)] = item
	}
	return items, true, nil
}

func (s *Store) persistSnapshotsIndex(items map[string]model.Snapshot) error {
	return s.persistIndex("snapshots-latest", casIndexSnapshot[model.Snapshot]{
		Version: "cas-index-v1",
		Items:   sortedSnapshots(items),
	})
}

func (s *Store) loadSnapshotsIndex() (map[string]model.Snapshot, bool, error) {
	var snapshot casIndexSnapshot[model.Snapshot]
	ok, err := s.loadIndex("snapshots-latest", &snapshot)
	if err != nil || !ok {
		return nil, ok, err
	}
	items := map[string]model.Snapshot{}
	for _, item := range snapshot.Items {
		items[item.Repo+"/"+item.Branch] = item
	}
	return items, true, nil
}

func (s *Store) persistCommitmentsIndex(items map[string]model.Commitment) error {
	return s.persistIndex("commitments-latest", casIndexSnapshot[model.Commitment]{
		Version: "cas-index-v1",
		Items:   sortedCommitments(items),
	})
}

func (s *Store) loadCommitmentsIndex() (map[string]model.Commitment, bool, error) {
	var snapshot casIndexSnapshot[model.Commitment]
	ok, err := s.loadIndex("commitments-latest", &snapshot)
	if err != nil || !ok {
		return nil, ok, err
	}
	items := map[string]model.Commitment{}
	for _, item := range snapshot.Items {
		items[item.CommitmentID] = item
	}
	return items, true, nil
}

func (s *Store) persistEvidenceIndex(items []model.Evidence) error {
	return s.persistIndex("evidence-log", casIndexSnapshot[model.Evidence]{
		Version: "cas-index-v1",
		Items:   sortedEvidence(items),
	})
}

func (s *Store) loadEvidenceIndex() ([]model.Evidence, bool, error) {
	var snapshot casIndexSnapshot[model.Evidence]
	ok, err := s.loadIndex("evidence-log", &snapshot)
	if err != nil || !ok {
		return nil, ok, err
	}
	return append([]model.Evidence(nil), snapshot.Items...), true, nil
}

func (s *Store) persistAssessmentsIndex(items []model.Assessment) error {
	return s.persistIndex("assessments-log", casIndexSnapshot[model.Assessment]{
		Version: "cas-index-v1",
		Items:   sortedAssessments(items),
	})
}

func (s *Store) loadAssessmentsIndex() ([]model.Assessment, bool, error) {
	var snapshot casIndexSnapshot[model.Assessment]
	ok, err := s.loadIndex("assessments-log", &snapshot)
	if err != nil || !ok {
		return nil, ok, err
	}
	return append([]model.Assessment(nil), snapshot.Items...), true, nil
}

func (s *Store) persistArtifactsIndex(items []model.ArtifactRecord) error {
	return s.persistIndex("artifacts-log", casIndexSnapshot[model.ArtifactRecord]{
		Version: "cas-index-v1",
		Items:   sortedArtifacts(items),
	})
}

func (s *Store) loadArtifactsIndex() ([]model.ArtifactRecord, bool, error) {
	var snapshot casIndexSnapshot[model.ArtifactRecord]
	ok, err := s.loadIndex("artifacts-log", &snapshot)
	if err != nil || !ok {
		return nil, ok, err
	}
	return append([]model.ArtifactRecord(nil), snapshot.Items...), true, nil
}

func (s *Store) persistImportsIndex(items []model.ImportRecord) error {
	return s.persistIndex("imports-log", casIndexSnapshot[model.ImportRecord]{
		Version: "cas-index-v1",
		Items:   sortedImports(items),
	})
}

func (s *Store) loadImportsIndex() ([]model.ImportRecord, bool, error) {
	var snapshot casIndexSnapshot[model.ImportRecord]
	ok, err := s.loadIndex("imports-log", &snapshot)
	if err != nil || !ok {
		return nil, ok, err
	}
	return append([]model.ImportRecord(nil), snapshot.Items...), true, nil
}
