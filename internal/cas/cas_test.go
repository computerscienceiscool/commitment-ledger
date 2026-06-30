package cas

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPutWritesToProfileAwarePath(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	id, err := store.Put([]byte("hello"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err := os.Stat(store.Path(id)); err != nil {
		t.Fatalf("profile path missing: %v", err)
	}
	if _, err := os.Stat(store.LegacyPath(id)); !os.IsNotExist(err) {
		t.Fatalf("legacy path should not be written, got err=%v", err)
	}
}

func TestGetFallsBackToLegacyPath(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	id := "bafkreic3n4w6f5h6c4jkzqv4e3m2n1p0q9r8s7t6u5v4w3x2y1z0abcd"
	legacy := store.LegacyPath(id)

	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	data, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(data) != "legacy" {
		t.Fatalf("Get returned %q, want legacy", string(data))
	}
}
