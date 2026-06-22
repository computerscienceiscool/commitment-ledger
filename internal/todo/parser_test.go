package todo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"commitment-ledger/internal/model"
)

func TestParseSupportsNumericAndProquintWithSubtasks(t *testing.T) {
	root := t.TempDir()
	todoDir := filepath.Join(root, "TODO")
	if err := os.MkdirAll(todoDir, 0o755); err != nil {
		t.Fatalf("mkdir todo dir: %v", err)
	}

	indexPath := filepath.Join(todoDir, "TODO.md")
	detailPath := filepath.Join(todoDir, "TODO-ravud-ship-welcome-flow.md")
	if err := os.WriteFile(indexPath, []byte("001 - Add route support\n- [ ] TODO-ravud - Ship welcome flow (`TODO/TODO-ravud-ship-welcome-flow.md`)\n[x] TODO-kupun - Done item\n"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(detailPath, []byte("# TODO-ravud\n- [ ] 1. Add route\n- [x] 2.1 Add tests\n"), 0o644); err != nil {
		t.Fatalf("write detail: %v", err)
	}

	items, err := Parse("guide", "main", "abc123", indexPath, time.Date(2026, 6, 22, 11, 0, 0, 0, time.FixedZone("PDT", -7*3600)), map[string]model.WorkItem{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("got %d items, want 5", len(items))
	}

	got := map[string]model.WorkItem{}
	for _, item := range items {
		got[item.WorkID] = item
	}

	if got["001"].Status != "open" {
		t.Fatalf("numeric item status = %q, want open", got["001"].Status)
	}
	if got["TODO-ravud"].DetailFile != "TODO/TODO-ravud-ship-welcome-flow.md" {
		t.Fatalf("detail file = %q", got["TODO-ravud"].DetailFile)
	}
	if got["TODO-ravud/1"].ParentWork != "TODO-ravud" {
		t.Fatalf("parent work = %q", got["TODO-ravud/1"].ParentWork)
	}
	if got["TODO-ravud/2.1"].Status != "complete" {
		t.Fatalf("subtask status = %q, want complete", got["TODO-ravud/2.1"].Status)
	}
	if got["TODO-kupun"].Status != "complete" {
		t.Fatalf("checked item status = %q, want complete", got["TODO-kupun"].Status)
	}
}
