package cas

import (
	"fmt"
	"os"
	"path/filepath"

	"commitment-ledger/internal/cid"
)

type Store struct {
	Root    string
	Profile string
}

func New(root string) *Store {
	return &Store{
		Root:    root,
		Profile: DefaultProfile,
	}
}

func (s *Store) Put(data []byte) (string, error) {
	id := cid.Sum(data)
	path := s.Path(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("mkdir cas path for %q: %w", id, err)
	}
	if _, err := os.Stat(path); err == nil {
		return id, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat cas path %q: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write cas object %q: %w", id, err)
	}
	return id, nil
}

func (s *Store) Get(id string) ([]byte, error) {
	paths := []string{s.Path(id)}
	if legacy := s.LegacyPath(id); legacy != paths[0] {
		paths = append(paths, legacy)
	}
	var lastErr error
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read cas object %q: %w", id, err)
		}
	}
	return nil, fmt.Errorf("read cas object %q: %w", id, lastErr)
}

func (s *Store) Path(id string) string {
	return s.pathForProfile(s.Profile, id)
}

func (s *Store) LegacyPath(id string) string {
	prefix := id
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}
	return filepath.Join(s.Root, "data", "cas", prefix, id+".bin")
}

func (s *Store) pathForProfile(profile string, id string) string {
	prefix := id
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}
	base := filepath.Join(s.Root, "data", "cas")
	if profile != "" {
		base = filepath.Join(base, profile)
	}
	return filepath.Join(base, prefix, id+".bin")
}
