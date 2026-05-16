package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddListEditAndDone(t *testing.T) {
	home := t.TempDir()
	store := NewStore(home)
	store.Now = func() time.Time { return time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC) }
	scope := Scope{Home: home, Project: "lune"}

	item, err := store.Add(scope, AddOptions{
		Title: "fix stuns",
		Body:  "animation plays\n\tbut mob still walks",
		Tags:  []string{"combat", "bug"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(item.ID) != 8 || len(item.DisplayID) != 3 {
		t.Fatalf("id/display = %q/%q", item.ID, item.DisplayID)
	}

	items, _, err := store.Items(scope, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "fix stuns" || !hasTag(items[0].Tags, "combat") {
		t.Fatalf("items = %#v", items)
	}
	if body := items[0].Body(); !strings.Contains(body, "animation plays") || !strings.Contains(body, "\tbut mob still walks") {
		t.Fatalf("body = %q", body)
	}

	updated, err := store.Edit(scope, item.DisplayID, EditOptions{Append: "next line"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if body := updated.Body(); !strings.Contains(body, "\n\nnext line") {
		t.Fatalf("appended body = %q", body)
	}

	done, err := store.SetDone(scope, item.DisplayID, true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !done.Done {
		t.Fatalf("done item = %#v", done)
	}
	openItems, _, err := store.Items(scope, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(openItems) != 0 {
		t.Fatalf("open items after done = %#v", openItems)
	}
	allItems, _, err := store.Items(scope, ListOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(allItems) != 1 || !allItems[0].Done {
		t.Fatalf("all items = %#v", allItems)
	}
}

func TestSetDoneCompletesNestedTaskDescendants(t *testing.T) {
	home := t.TempDir()
	path := ProjectPath(home, "meeco")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"# meeco",
		"",
		"- [ ] parent ticket",
		"<!-- rune:id=parent00 type=task tags= created=2026-05-16T00:00:00Z -->",
		"    - [ ] child ticket",
		"    <!-- rune:id=child000 type=task tags= created=2026-05-16T00:00:00Z -->",
		"        - [ ] grandchild ticket",
		"        <!-- rune:id=grand000 type=task tags= created=2026-05-16T00:00:00Z -->",
		"- [ ] sibling ticket",
		"<!-- rune:id=sibling0 type=task tags= created=2026-05-16T00:00:00Z -->",
	}, "\n")
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(home)
	scope := Scope{Home: home, Project: "meeco"}
	if _, err := store.SetDone(scope, "parent", true, false, false); err != nil {
		t.Fatal(err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(updated)
	for _, want := range []string{
		"- [x] parent ticket",
		"    - [x] child ticket",
		"        - [x] grandchild ticket",
		"- [ ] sibling ticket",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("updated markdown missing %q:\n%s", want, got)
		}
	}
}

func TestAddAssignsIDsToNestedBodyTasks(t *testing.T) {
	home := t.TempDir()
	store := NewStore(home)
	scope := Scope{Home: home, Project: "meeco"}
	item, err := store.Add(scope, AddOptions{
		Title: "parent tracker",
		Body: strings.Join([]string{
			"Goal: track child work.",
			"",
			"- [ ] first child",
			"- [ ] second child",
			"",
			"Acceptance:",
			"- plain detail",
		}, "\n"),
	})
	if err != nil {
		t.Fatal(err)
	}

	all, _, err := store.Items(scope, ListOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("items = %d, want parent, two children, plain detail: %#v", len(all), all)
	}
	for _, title := range []string{"first child", "second child"} {
		var child *Item
		for _, candidate := range all {
			if candidate.Title == title {
				child = candidate
				break
			}
		}
		if child == nil {
			t.Fatalf("missing child %q in %#v", title, all)
		}
		if child.ID == "" || child.DisplayID == "" {
			t.Fatalf("child %q has no ID/display ID: %#v", title, child)
		}
	}
	if _, err := store.SetDone(scope, all[1].DisplayID, true, false, false); err != nil {
		t.Fatalf("nested child was not individually completable: %v", err)
	}
	parent, err := store.SetDone(scope, item.DisplayID, true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !parent.Done {
		t.Fatalf("parent not done: %#v", parent)
	}
}

func TestOpenFilterHidesDescendantsOfDoneTask(t *testing.T) {
	home := t.TempDir()
	path := ProjectPath(home, "meeco")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"# meeco",
		"",
		"- [x] completed parent",
		"<!-- rune:id=parent00 type=task tags= created=2026-05-16T00:00:00Z -->",
		"  Details:",
		"  - plain detail without checkbox",
		"    - [ ] unfinished child",
		"    <!-- rune:id=child000 type=task tags= created=2026-05-16T00:00:00Z -->",
		"- [ ] open sibling",
		"<!-- rune:id=sibling0 type=task tags= created=2026-05-16T00:00:00Z -->",
	}, "\n")
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(home)
	scope := Scope{Home: home, Project: "meeco"}
	openItems, _, err := store.Items(scope, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(openItems) != 1 || openItems[0].Title != "open sibling" {
		t.Fatalf("open items = %#v, want only sibling", openItems)
	}

	doneItems, _, err := store.Items(scope, ListOptions{Done: true})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, item := range doneItems {
		titles = append(titles, item.Title)
	}
	for _, want := range []string{"completed parent", "plain detail without checkbox", "unfinished child"} {
		if !containsString(titles, want) {
			t.Fatalf("done titles = %#v, missing %q", titles, want)
		}
	}
}

func TestArchiveDoneMovesCompletedSubtree(t *testing.T) {
	home := t.TempDir()
	store := NewStore(home)
	store.Now = func() time.Time { return time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC) }
	scope := Scope{Home: home, Project: "meeco"}
	path := ProjectPath(home, "meeco")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"# meeco",
		"",
		"- [x] completed parent",
		"<!-- rune:id=parent00 type=task tags= created=2026-05-16T00:00:00Z -->",
		"  - plain detail without checkbox",
		"    - [x] completed child",
		"    <!-- rune:id=child000 type=task tags= created=2026-05-16T00:00:00Z -->",
		"- [ ] open sibling",
		"<!-- rune:id=sibling0 type=task tags= created=2026-05-16T00:00:00Z -->",
	}, "\n")
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	count, archivePath, err := store.ArchiveDone(scope)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("archive count = %d, want 2", count)
	}
	project, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(project); strings.Contains(got, "completed parent") ||
		strings.Contains(got, "plain detail without checkbox") ||
		strings.Contains(got, "completed child") ||
		!strings.Contains(got, "open sibling") {
		t.Fatalf("project after archive:\n%s", got)
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(archive); !strings.Contains(got, "completed parent") ||
		!strings.Contains(got, "plain detail without checkbox") ||
		!strings.Contains(got, "completed child") {
		t.Fatalf("archive missing completed subtree:\n%s", got)
	}
}

func TestAddRequiresProjectScopeAndDoesNotCreateInbox(t *testing.T) {
	home := t.TempDir()
	store := NewStore(home)
	if _, err := store.Add(Scope{Home: home}, AddOptions{Title: "orphan task"}); err == nil {
		t.Fatal("Add without project scope succeeded, want error")
	}
	if _, err := os.Stat(filepath.Join(home, "inbox.md")); !os.IsNotExist(err) {
		t.Fatalf("inbox.md stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(home, "today")); !os.IsNotExist(err) {
		t.Fatalf("today stat error = %v, want not exist", err)
	}
}

func TestAddCreatesProjectFileWithoutInboxHeading(t *testing.T) {
	home := t.TempDir()
	store := NewStore(home)
	scope := Scope{Home: home, Project: "lune"}
	if _, err := store.Add(scope, AddOptions{Title: "project task"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(ProjectPath(home, "lune"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if strings.Contains(strings.ToLower(got), "## inbox") {
		t.Fatalf("project file kept inbox heading:\n%s", got)
	}
	if !strings.Contains(got, "# lune\n\n- [ ] project task\n") {
		t.Fatalf("project file content:\n%s", got)
	}
}

func TestResolvePrefixAllowsShortestUniqueAndReportsAmbiguity(t *testing.T) {
	items := []*Item{
		{ID: "1hc9fq2a", Title: "networking idea"},
		{ID: "1hcz0000", Title: "lyric fragment"},
		{ID: "a8v00000", Title: "fire elementalist"},
	}
	ApplyDisplayIDs(items)
	if items[0].DisplayID != "1hc9" {
		t.Fatalf("display id = %q", items[0].DisplayID)
	}
	if items[2].DisplayID != "a8v" {
		t.Fatalf("display id = %q", items[2].DisplayID)
	}
	if got, err := ResolvePrefix(items, "a8"); err != nil || got.ID != "a8v00000" {
		t.Fatalf("ResolvePrefix(a8) = %#v, %v", got, err)
	}
	if _, err := ResolvePrefix(items, "1hc"); err == nil {
		t.Fatal("ResolvePrefix(1hc) succeeded, want ambiguity")
	}
}

func TestImportAssignsIDsToExistingMarkdown(t *testing.T) {
	home := t.TempDir()
	store := NewStore(home)
	store.Now = func() time.Time { return time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC) }
	src := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(src, []byte("# lune todo\n\n- [ ] fix stuns\n- [x] weather\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	count, path, err := store.Import(src, "lune")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(content), "<!-- rune:id="); got != 2 {
		t.Fatalf("metadata count = %d\n%s", got, content)
	}
}

func TestParseDocumentPreservesNestedTaskDepth(t *testing.T) {
	doc, err := ParseDocument("todo.md", "project", "lune", []byte(strings.Join([]string{
		"# todo",
		"",
		"- [ ] parent",
		"    - [x] child",
		"        - [ ] grandchild",
		"",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(doc.Items))
	}
	for idx, want := range []int{0, 1, 2} {
		if doc.Items[idx].Depth != want {
			t.Fatalf("item %d depth = %d, want %d", idx, doc.Items[idx].Depth, want)
		}
	}
}

func TestParseDocumentRecoversMetadataDisplacedBelowBody(t *testing.T) {
	doc, err := ParseDocument("todo.md", "project", "lune", []byte(strings.Join([]string{
		"# todo",
		"",
		"- [ ] selected ticket",
		"",
		"updated plan:",
		"",
		"  1. first step",
		"<!-- rune:id=abc12345 type=task tags=combat,bug created=2026-05-14T00:00:00Z -->",
		"- [ ] next ticket",
		"<!-- rune:id=def67890 type=task tags= created=2026-05-14T00:00:00Z -->",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(doc.Items))
	}
	item := doc.Items[0]
	if item.ID != "abc12345" {
		t.Fatalf("id = %q, want abc12345", item.ID)
	}
	if got := item.Body(); !strings.Contains(got, "updated plan:") || strings.Contains(got, "rune:id") {
		t.Fatalf("body = %q", got)
	}
	if got := strings.TrimSpace(doc.Lines[item.Line+1]); !strings.HasPrefix(got, "<!-- rune:id=abc12345") {
		t.Fatalf("metadata was not normalized under item: %q", got)
	}
}

func TestEditAppendSavesRecoveredMetadataBeforeBody(t *testing.T) {
	home := t.TempDir()
	path := ProjectPath(home, "lune")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"# lune",
		"",
		"- [ ] selected ticket",
		"",
		"updated plan:",
		"<!-- rune:id=abc12345 type=task tags= created=2026-05-14T00:00:00Z -->",
		"- [ ] next ticket",
		"<!-- rune:id=def67890 type=task tags= created=2026-05-14T00:00:00Z -->",
	}, "\n")
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(home)
	scope := Scope{Home: home, Project: "lune"}
	if _, err := store.Edit(scope, "abc", EditOptions{Append: "implementation notes"}, false); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(updated), "\n"), "\n")
	if got := strings.TrimSpace(lines[3]); !strings.HasPrefix(got, "<!-- rune:id=abc12345") {
		t.Fatalf("metadata line = %q, want normalized under item\n%s", got, string(updated))
	}
	if body := string(updated); !strings.Contains(body, "updated plan:") || !strings.Contains(body, "implementation notes") {
		t.Fatalf("updated body lost content:\n%s", body)
	}
}

func TestAddNearInsertsAboveAndBelowAnchor(t *testing.T) {
	home := t.TempDir()
	store := NewStore(home)
	store.Now = func() time.Time { return time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC) }
	scope := Scope{Home: home, Project: "lune"}

	first, err := store.Add(scope, AddOptions{Title: "first"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddNear(scope, first.ID, false, AddOptions{Title: "below first"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddNear(scope, first.ID, true, AddOptions{Title: "above first"}, false); err != nil {
		t.Fatal(err)
	}

	items, _, err := store.Items(scope, ListOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(items))
	for idx, item := range items {
		got[idx] = item.Title
	}
	want := []string{"above first", "first", "below first"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
	for _, item := range items {
		if item.Depth != 0 {
			t.Fatalf("%q depth = %d, want 0", item.Title, item.Depth)
		}
	}
}

func TestAddInsertsBeforeDoneSections(t *testing.T) {
	for _, heading := range []string{"## Restored done 2026-05-15", "## Done 2026-05-15"} {
		t.Run(heading, func(t *testing.T) {
			home := t.TempDir()
			store := NewStore(home)
			store.Now = func() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }
			scope := Scope{Home: home, Project: "lune"}
			projectPath := ProjectPath(home, "lune")
			if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
				t.Fatal(err)
			}
			content := strings.Join([]string{
				"# lune",
				"",
				"- [ ] existing open",
				"<!-- rune:id=open0000 type=task tags= created=2026-05-14T00:00:00Z -->",
				"",
				heading,
				"",
				"- [x] restored done",
				"<!-- rune:id=done0000 type=task tags= created=2026-05-14T00:00:00Z -->",
				"",
			}, "\n")
			if err := os.WriteFile(projectPath, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			added, err := store.Add(scope, AddOptions{Title: "fresh open", Body: "new body"})
			if err != nil {
				t.Fatal(err)
			}
			if added.Title != "fresh open" || added.Done {
				t.Fatalf("added item = %#v, want fresh open task", added)
			}
			updated, err := os.ReadFile(projectPath)
			if err != nil {
				t.Fatal(err)
			}
			got := string(updated)
			existingOpen := strings.Index(got, "- [ ] existing open")
			freshOpen := strings.Index(got, "- [ ] fresh open")
			doneHeading := strings.Index(got, heading)
			restoredDone := strings.Index(got, "- [x] restored done")
			if existingOpen < 0 || freshOpen < 0 || doneHeading < 0 || restoredDone < 0 {
				t.Fatalf("project missing expected content:\n%s", got)
			}
			if !(existingOpen < freshOpen && freshOpen < doneHeading && doneHeading < restoredDone) {
				t.Fatalf("unexpected order existing=%d fresh=%d heading=%d done=%d:\n%s", existingOpen, freshOpen, doneHeading, restoredDone, got)
			}
			if !strings.Contains(got, "\n  new body\n") {
				t.Fatalf("body was not preserved:\n%s", got)
			}
			if count := strings.Count(got, "<!-- rune:id="); count != 3 {
				t.Fatalf("metadata count = %d, want 3:\n%s", count, got)
			}
		})
	}
}

func TestRestoreArchivedProjectMovesArchivedSectionBackToProject(t *testing.T) {
	home := t.TempDir()
	store := NewStore(home)
	scope := Scope{Home: home, Project: "lune"}
	projectPath := ProjectPath(home, "lune")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("# lune\n\n- [ ] open task\n<!-- rune:id=open0000 type=task tags= created=2026-05-14T00:00:00Z -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(home, "archive", "2026-W20.md")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := strings.Join([]string{
		"# Archive",
		"",
		"## lune - 2026-05-15",
		"",
		"- [x] done task",
		"<!-- rune:id=done0000 type=task tags= created=2026-05-14T00:00:00Z -->",
		"",
		"    - [x] done child",
		"<!-- rune:id=chil0000 type=task tags= created=2026-05-14T00:00:00Z -->",
		"",
	}, "\n")
	if err := os.WriteFile(archivePath, []byte(archive), 0o644); err != nil {
		t.Fatal(err)
	}
	count, paths, err := store.RestoreArchivedProject(scope)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || len(paths) != 1 {
		t.Fatalf("restore count/paths = %d/%#v, want 2/1", count, paths)
	}
	project, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(project); !strings.Contains(got, "## Done 2026-05-15") ||
		strings.Contains(got, "## Restored done 2026-05-15") ||
		!strings.Contains(got, "- [x] done task") ||
		!strings.Contains(got, "    - [x] done child") {
		t.Fatalf("project after restore:\n%s", got)
	}
	archiveAfter, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(archiveAfter), "done task") ||
		strings.Contains(string(archiveAfter), "## lune - 2026-05-15") {
		t.Fatalf("archive still contains restored section:\n%s", string(archiveAfter))
	}
}

func TestDecodeEscapes(t *testing.T) {
	got := DecodeEscapes(`hello\n\tworld \\ ok`)
	want := "hello\n\tworld \\ ok"
	if got != want {
		t.Fatalf("DecodeEscapes = %q, want %q", got, want)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
