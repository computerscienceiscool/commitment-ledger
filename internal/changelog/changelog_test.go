package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"commitment-ledger/internal/protocol"
)

func TestWriteManagedCreatesReplaceableSection(t *testing.T) {
	root := t.TempDir()
	copyProtocolDocs(t, root)
	registry, err := protocol.Load(root)
	if err != nil {
		t.Fatalf("protocol.Load: %v", err)
	}

	if err := WriteManaged(root, registry, "v0.1.0"); err != nil {
		t.Fatalf("WriteManaged initial: %v", err)
	}
	path := Path(root)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read changelog: %v", err)
	}
	text := string(body)
	if strings.Count(text, managedStart) != 1 || strings.Count(text, managedEnd) != 1 {
		t.Fatalf("managed markers missing or duplicated:\n%s", text)
	}
	if !strings.Contains(text, "Current commitment-ledger v0.1.0 emission for local frozen `commitment-evidence-v2`.") {
		t.Fatalf("missing generated entry:\n%s", text)
	}

	if err := WriteManaged(root, registry, "v0.2.0"); err != nil {
		t.Fatalf("WriteManaged second: %v", err)
	}
	body, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read changelog second: %v", err)
	}
	text = string(body)
	if strings.Contains(text, "v0.1.0") {
		t.Fatalf("expected managed section to be replaced, still found old version:\n%s", text)
	}
	if !strings.Contains(text, "Current commitment-ledger v0.2.0 emission for local frozen `implementation-conformance-v1`.") {
		t.Fatalf("missing updated generated entry:\n%s", text)
	}
}

func TestMatchSpecFindsStructuredEntries(t *testing.T) {
	root := t.TempDir()
	path := Path(root)
	if err := os.WriteFile(path, []byte("# CHANGELOG\n\n## Unreleased\n\n"+RenderManagedSection([]Entry{{
		Claim:          "implements",
		Spec:           "bafy-test",
		Scope:          "full",
		BreakingChange: "false",
		Notes:          "demo",
	}})), 0o644); err != nil {
		t.Fatalf("write changelog: %v", err)
	}

	entries, err := MatchSpec(root, "bafy-test")
	if err != nil {
		t.Fatalf("MatchSpec: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if entries[0].Claim != "implements" || entries[0].Notes != "demo" {
		t.Fatalf("unexpected parsed entry: %#v", entries[0])
	}
}

func copyProtocolDocs(t *testing.T, root string) {
	t.Helper()
	sourceDir := filepath.Join(repoRoot(t), "docs", "protocols")
	destDir := filepath.Join(root, "docs", "protocols")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir protocol dir: %v", err)
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatalf("read protocol dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
		if err != nil {
			t.Fatalf("read protocol doc %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(destDir, entry.Name()), data, 0o644); err != nil {
			t.Fatalf("write protocol doc %s: %v", entry.Name(), err)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
