package ledger

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	legacyReferenceSetName       = "ledger-heads"
	workObservationReferenceSet  = "work-observation-heads"
	commitmentStateReferenceSet  = "commitment-state-heads"
	artifactExchangeReferenceSet = "artifact-exchange-heads"
)

type referenceSet struct {
	Version string                    `json:"version"`
	Name    string                    `json:"name"`
	Entries map[string]referenceEntry `json:"entries"`
}

type referenceEntry struct {
	CID string `json:"cid"`
}

func (s *Store) referenceSetRefPath(name string) string {
	return s.refPath("reference-sets", name+".ref")
}

func (s *Store) referenceSetCachePath(name string) string {
	return s.refPath("reference-sets", name+".json")
}

func (s *Store) loadReferenceSet(name string) (referenceSet, bool, error) {
	var set referenceSet
	refBody, err := os.ReadFile(s.referenceSetRefPath(name))
	if err == nil {
		id := string(trimTrailingWhitespace(refBody))
		if id != "" {
			data, casErr := s.CAS.Get(id)
			if casErr == nil {
				if err := json.Unmarshal(data, &set); err != nil {
					return referenceSet{}, false, fmt.Errorf("decode reference set %s cas object %q: %w", name, id, err)
				}
				return set, true, nil
			}
		}
	} else if !os.IsNotExist(err) {
		return referenceSet{}, false, fmt.Errorf("read reference set ref %s: %w", name, err)
	}

	cacheBody, err := os.ReadFile(s.referenceSetCachePath(name))
	if err == nil {
		if err := json.Unmarshal(cacheBody, &set); err != nil {
			return referenceSet{}, false, fmt.Errorf("decode reference set cache %s: %w", name, err)
		}
		return set, true, nil
	}
	if !os.IsNotExist(err) {
		return referenceSet{}, false, fmt.Errorf("read reference set cache %s: %w", name, err)
	}
	return referenceSet{}, false, nil
}

func (s *Store) persistReferenceSet(name string, set referenceSet) error {
	body, err := json.Marshal(set)
	if err != nil {
		return fmt.Errorf("marshal reference set %s: %w", name, err)
	}
	id, err := s.CAS.Put(body)
	if err != nil {
		return fmt.Errorf("store reference set %s in cas: %w", name, err)
	}
	if err := writeBytesFile(s.referenceSetRefPath(name), []byte(id+"\n")); err != nil {
		return fmt.Errorf("write reference set ref %s: %w", name, err)
	}
	if err := writeBytesFile(s.referenceSetCachePath(name), body); err != nil {
		return fmt.Errorf("write reference set cache %s: %w", name, err)
	}
	return nil
}

func (s *Store) loadOrInitReferenceSet(name string) (referenceSet, error) {
	set, ok, err := s.loadReferenceSet(name)
	if err != nil {
		return referenceSet{}, err
	}
	if ok {
		if set.Entries == nil {
			set.Entries = map[string]referenceEntry{}
		}
		return set, nil
	}
	return referenceSet{
		Version: "reference-set-v1",
		Name:    name,
		Entries: map[string]referenceEntry{},
	}, nil
}

func trimTrailingWhitespace(body []byte) []byte {
	end := len(body)
	for end > 0 {
		switch body[end-1] {
		case ' ', '\n', '\r', '\t':
			end--
		default:
			return body[:end]
		}
	}
	return body[:end]
}

func referenceSetNameForIndex(indexName string) string {
	switch indexName {
	case "work-items-latest", "snapshots-latest":
		return workObservationReferenceSet
	case "commitments-latest", "evidence-log", "assessments-log":
		return commitmentStateReferenceSet
	case "artifacts-log", "imports-log":
		return artifactExchangeReferenceSet
	default:
		return legacyReferenceSetName
	}
}

func referenceSetDirectoryForIndex(indexName string) string {
	switch referenceSetNameForIndex(indexName) {
	case workObservationReferenceSet:
		return "work-observation"
	case commitmentStateReferenceSet:
		return "commitment-state"
	case artifactExchangeReferenceSet:
		return "artifact-exchange"
	default:
		return "legacy"
	}
}
