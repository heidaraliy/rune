package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heidaraliy/rune/internal/domain"
)

func TestReadFileBuildsTypedItemsWithoutChangingSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lune.md")
	original := strings.Join([]string{
		"# lune",
		"",
		"- [ ] parent task",
		"<!-- rune:id=parent01 type=task tags=agent created=2026-07-01T10:00:00Z -->",
		"  parent detail",
		"    - [x] child task",
		"    <!-- rune:id=child001 type=task tags=bug created=2026-07-01T11:00:00Z finished_at=2026-07-01T12:00:00Z -->",
		"- idea note",
		"<!-- rune:id=note0001 type=note tags=idea created=2026-07-01T13:00:00Z -->",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := ReadFile(path, "Lune", time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(bundle.Items))
	}
	if bundle.Project != "lune" || bundle.SourcePath != path {
		t.Fatalf("bundle identity = %#v", bundle)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatal("import changed the source Markdown")
	}
	parent := bundle.Items[0]
	child := bundle.Items[1]
	if parent.Entity.Kind != domain.KindTask || parent.Entity.Status != domain.StatusDraft {
		t.Fatalf("parent = %#v", parent.Entity)
	}
	if child.ParentLegacyID != "parent01" || child.Entity.Status != domain.StatusCompleted {
		t.Fatalf("child = %#v", child)
	}
	if bundle.Items[2].Entity.Kind != domain.KindNote || bundle.Items[2].Entity.Status != "" {
		t.Fatalf("note = %#v", bundle.Items[2].Entity)
	}
}

func TestReadFileAssignsDeterministicLegacyIDWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ideas.md")
	content := "# ideas\n\n- [ ] missing metadata id\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	first, err := ReadFile(path, "ideas", now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReadFile(path, "ideas", now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Items[0].Entity.LegacyID != second.Items[0].Entity.LegacyID {
		t.Fatalf("legacy IDs differ: %q != %q", first.Items[0].Entity.LegacyID, second.Items[0].Entity.LegacyID)
	}
	if !strings.HasPrefix(first.Items[0].Entity.LegacyID, "generated-") {
		t.Fatalf("legacy ID = %q", first.Items[0].Entity.LegacyID)
	}
	if len(first.Warnings) != 1 || !strings.Contains(first.Warnings[0], "assigned deterministic legacy id") {
		t.Fatalf("warnings = %#v", first.Warnings)
	}
}
