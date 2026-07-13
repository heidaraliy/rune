package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	v2app "github.com/heidaraliy/rune/internal/app/v2"
	"github.com/heidaraliy/rune/internal/handoff"
)

type captureProgram struct {
	model tea.Model
}

func (p captureProgram) Run() (tea.Model, error) {
	return p.model, nil
}

func gitProjectDir(t *testing.T, name string) string {
	t.Helper()
	cwd := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return cwd
}

func TestRunAddListEditShowWithInterspersedFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "fix stuns", "--tag", "combat,bug"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	fields := strings.Fields(stdout.String())
	if len(fields) < 2 {
		t.Fatalf("add stdout = %q", stdout.String())
	}
	id := fields[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"list"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("list code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fix stuns") || !strings.Contains(stdout.String(), "#bug #combat") {
		t.Fatalf("list stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"edit", id, "--end", `hello\n\tworld with ` + "`code`"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("edit code = %d, stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"show", id}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("show code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "hello\n\tworld with `code`") {
		t.Fatalf("show stdout = %q", stdout.String())
	}
}

func TestRunV2LocalStructuredCaptureLinkAndSearch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	db := filepath.Join(home, "rune-v2.db")
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := run([]string{"v2", "capture", "build foundation", "--project", "rune", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("capture code = %d, stderr=%q", code, stderr.String())
	}
	taskID := strings.Fields(stdout.String())[1]
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "capture", "architecture note", "--note", "--project", "rune", "--db", db}, &stdout, &stderr, strings.NewReader("details"), cwd)
	if code != 0 {
		t.Fatalf("note capture code = %d, stderr=%q", code, stderr.String())
	}
	noteID := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "status", taskID, "ready", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 || !strings.Contains(stdout.String(), "Status "+taskID) {
		t.Fatalf("status code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "link", taskID, noteID, "--kind", "references", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 || !strings.Contains(stdout.String(), "references") {
		t.Fatalf("link code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "search", "foundation", "--project", "rune", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 || !strings.Contains(stdout.String(), "build foundation") {
		t.Fatalf("search code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "show", taskID, "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("show code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"Kind: task", "Status: ready", "Links:", "references"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("show missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunV2SyncReportsLocalLedgerAndRejectsAuthoredMutation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	db := filepath.Join(home, "rune-v2.db")
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer

	if code := run([]string{"v2", "capture", "immutable task", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 {
		t.Fatalf("capture code=%d stderr=%q", code, stderr.String())
	}
	taskID := strings.Fields(stdout.String())[1]
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"v2", "edit", taskID, "--title", "changed", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd); code == 0 || !strings.Contains(stderr.String(), "append-only") {
		t.Fatalf("edit code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"v2", "delete", taskID, "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd); code == 0 || !strings.Contains(stderr.String(), "cannot be edited or deleted") {
		t.Fatalf("delete code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"v2", "sync", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 {
		t.Fatalf("sync code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"Workspace: local", "Remote: not-configured", "Local cursor: 1", "Pending changes: 1", "Open conflicts: 0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sync output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunV2QueueRunAndInspectArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	db := filepath.Join(home, "rune-v2.db")
	artifactRoot := filepath.Join(home, "artifacts")
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := run([]string{"v2", "capture", "execute from terminal", "--project", "rune", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("capture code=%d stderr=%q", code, stderr.String())
	}
	taskID := strings.Fields(stdout.String())[1]
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "queue", taskID, "--db", db, "--artifact-root", artifactRoot}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("queue code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	queueLines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	runID := strings.Fields(queueLines[0])[2]
	contextID := strings.Fields(queueLines[1])[2]
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "run", runID, "--db", db, "--artifact-root", artifactRoot}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 || !strings.Contains(stdout.String(), "completed") {
		t.Fatalf("run code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "artifacts", runID, "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 || !strings.Contains(stdout.String(), "context.json") || !strings.Contains(stdout.String(), "result.md") {
		t.Fatalf("artifacts code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "artifact", contextID, "--db", db, "--artifact-root", artifactRoot}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 || !strings.Contains(stdout.String(), "rune.context.v1") {
		t.Fatalf("artifact code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"v2", "show", taskID, "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 || !strings.Contains(stdout.String(), "Status: completed") {
		t.Fatalf("show code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunV2CancelMarksQueuedRunAndTask(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	db := filepath.Join(home, "rune-v2.db")
	artifactRoot := filepath.Join(home, "artifacts")
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"v2", "capture", "cancel me", "--project", "rune", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 {
		t.Fatalf("capture code=%d stderr=%q", code, stderr.String())
	}
	taskID := strings.Fields(stdout.String())[1]
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"v2", "queue", taskID, "--db", db, "--artifact-root", artifactRoot}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 {
		t.Fatalf("queue code=%d stderr=%q", code, stderr.String())
	}
	runID := strings.Fields(stdout.String())[2]
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"v2", "cancel", runID, "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 || !strings.Contains(stdout.String(), "canceled") {
		t.Fatalf("cancel code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"v2", "show", taskID, "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 || !strings.Contains(stdout.String(), "Status: canceled") {
		t.Fatalf("show code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunV2TUILaunchesStructuredClient(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	db := filepath.Join(home, "rune-v2.db")
	cwd := t.TempDir()
	oldProgram := newProgram
	defer func() { newProgram = oldProgram }()
	var captured tea.Model
	newProgram = func(model tea.Model) programRunner {
		captured = model
		return captureProgram{model: model}
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"v2", "tui", "--project", "rune", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("tui code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if _, ok := captured.(v2app.Model); !ok {
		t.Fatalf("captured model = %T, want v2 app model", captured)
	}
}

func TestRunV2ImportIsSourcePreservingAndIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	db := filepath.Join(home, "rune-v2.db")
	source := filepath.Join(t.TempDir(), "ideas.md")
	original := "# ideas\n\n- [ ] imported task\n<!-- rune:id=import01 type=task created=2026-07-01T00:00:00Z -->\n"
	if err := os.WriteFile(source, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer
	for attempt := 0; attempt < 2; attempt++ {
		stdout.Reset()
		stderr.Reset()
		code := run([]string{"v2", "import", source, "--project", "ideas", "--db", db}, &stdout, &stderr, strings.NewReader(""), cwd)
		if code != 0 {
			t.Fatalf("import attempt %d code=%d stderr=%q", attempt, code, stderr.String())
		}
		if attempt == 0 && !strings.Contains(stdout.String(), "Imported 1 item(s), skipped 0") {
			t.Fatalf("first import output=%q", stdout.String())
		}
		if attempt == 1 && !strings.Contains(stdout.String(), "Imported 0 item(s), skipped 1") {
			t.Fatalf("second import output=%q", stdout.String())
		}
	}
	if after, err := os.ReadFile(source); err != nil || string(after) != original {
		t.Fatalf("source changed err=%v:\n%s", err, string(after))
	}
}

func TestRunListFormatsReadableCards(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "fix list spacing by wrapping a very long description that would otherwise stretch across wide terminal panes", "--project", "pretty", "--tag", "ux,agent"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"add", "remember context", "--note", "--project", "pretty"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add note code = %d, stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"done", id, "--project", "pretty"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("done code = %d, stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"list", "--all", "--project", "pretty"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("list code = %d, stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"2 items",
		"[x] fix list spacing by wrapping a very long description that would",
		"otherwise stretch across wide terminal panes",
		"note remember context",
		"#agent #ux",
		"pretty",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("list output missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"ITEM", "SOURCE"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("list output kept table header %q:\n%s", unwanted, got)
		}
	}
	if strings.Count(got, "\n\n") < 1 {
		t.Fatalf("list output should separate cards with a blank line:\n%s", got)
	}
}

func TestRunListSortsByTimestamps(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	projectPath := filepath.Join(home, "projects", "pretty.md")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"# pretty",
		"",
		"- [x] first finished",
		"<!-- rune:id=first000 type=task tags= created=2026-05-17T09:00:00Z finished_at=2026-05-17T10:00:00Z -->",
		"- [x] second finished",
		"<!-- rune:id=second00 type=task tags= created=2026-05-17T08:00:00Z finished_at=2026-05-17T12:00:00Z -->",
		"- [ ] unfinished",
		"<!-- rune:id=open0000 type=task tags= created=2026-05-17T07:00:00Z -->",
	}, "\n")
	if err := os.WriteFile(projectPath, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"list", "--all", "--project", "pretty", "--sort", "created_at"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("created sort code = %d, stderr=%q", code, stderr.String())
	}
	assertOutputOrder(t, stdout.String(), "unfinished", "second finished", "first finished")

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"list", "--all", "--project", "pretty", "--sort", "finished_at"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("finished sort code = %d, stderr=%q", code, stderr.String())
	}
	assertOutputOrder(t, stdout.String(), "first finished", "second finished", "unfinished")
}

func TestRunShowFormatsHumanReadableDetail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "inspect terminal output", "--project", "pretty", "--tag", "ux,agent", "--body", "first line\n\tsecond line"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"show", id, "--project", "pretty"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("show code = %d, stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"[ ] inspect terminal output",
		"status",
		"open",
		"heading",
		"pretty",
		"tags",
		"#agent #ux",
		"source",
		"projects/pretty.md:",
		"first line\n\tsecond line",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("show output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "# Rune Ticket:") || strings.Contains(got, "implement this ticket") {
		t.Fatalf("show output should not look like an agent ticket:\n%s", got)
	}
}

func TestRunAddProjectFlagOutsideGitWritesProjectFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "fix remote trace", "--project", "Lune"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	projectPath := filepath.Join(home, "projects", "lune.md")
	content, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if !strings.Contains(got, "# lune\n\n- [ ] fix remote trace\n") {
		t.Fatalf("project file content:\n%s", got)
	}
	if strings.Contains(strings.ToLower(got), "## inbox") {
		t.Fatalf("project file kept inbox heading:\n%s", got)
	}
	for _, legacyPath := range []string{
		filepath.Join(home, "inbox.md"),
		filepath.Join(home, "today"),
	} {
		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			t.Fatalf("%s stat error = %v, want not exist", legacyPath, err)
		}
	}
}

func TestRunInitAndAddUsesProjectLocalFileStore(t *testing.T) {
	cwd := gitProjectDir(t, "lune")

	var stdout, stderr bytes.Buffer
	code := run([]string{"init", "--project", "Lune"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("init code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), filepath.Join(cwd, ".rune")) ||
		!strings.Contains(stdout.String(), "project lune") {
		t.Fatalf("init stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"add", "project local note", "--body", "detail"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	entries, err := os.ReadDir(filepath.Join(cwd, ".rune", "projects", "lune"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].IsDir() || !strings.Contains(entries[0].Name(), "project-local-note") {
		t.Fatalf("local project entries = %#v", entries)
	}
	content, err := os.ReadFile(filepath.Join(cwd, ".rune", "projects", "lune", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(content); !strings.Contains(got, "- [ ] project local note") || !strings.Contains(got, "  detail") {
		t.Fatalf("local note content:\n%s", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"path", "--store"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("path --store code = %d, stderr=%q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != filepath.Join(cwd, ".rune") {
		t.Fatalf("path --store = %q, want local .rune", got)
	}
}

func TestRunMigrateSplitsSourceIntoProjectLocalStore(t *testing.T) {
	cwd := gitProjectDir(t, "lune")
	source := filepath.Join(t.TempDir(), "lune.md")
	legacy := strings.Join([]string{
		"# lune",
		"",
		"- [ ] first migrated",
		"  body one",
		"- [ ] second migrated",
		"<!-- rune:id=second00 type=task tags= created=2026-05-14T00:00:00Z -->",
	}, "\n") + "\n"
	if err := os.WriteFile(source, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", source, "--project", "lune"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("migrate code = %d, stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"Migrated 2 item(s) into 2 file(s) for project lune",
		"Source left unchanged: " + source,
		"Assigned 1 missing id(s)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("migrate stdout missing %q:\n%s", want, got)
		}
	}
	if after, err := os.ReadFile(source); err != nil || string(after) != legacy {
		t.Fatalf("source changed or unreadable err=%v:\n%s", err, string(after))
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"list", "--all"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("list code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "first migrated") || !strings.Contains(stdout.String(), "second migrated") {
		t.Fatalf("list stdout = %q", stdout.String())
	}
}

func TestRunAddRequiresProjectOutsideGit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "orphan note"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code == 0 {
		t.Fatalf("add code = 0, stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "project context required") || !strings.Contains(stderr.String(), "--project") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, "inbox.md")); !os.IsNotExist(err) {
		t.Fatalf("inbox.md stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(home, "today")); !os.IsNotExist(err) {
		t.Fatalf("today stat error = %v, want not exist", err)
	}
}

func TestRunInboxAndTodayCommandsAreRemoved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()

	for _, command := range []string{"inbox", "today"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{command, "legacy capture"}, &stdout, &stderr, strings.NewReader(""), cwd)
		if code == 0 {
			t.Fatalf("%s code = 0, stdout=%q", command, stdout.String())
		}
		if !strings.Contains(stderr.String(), "unknown command") {
			t.Fatalf("%s stderr = %q", command, stderr.String())
		}
	}
}

func TestRunDoneHidesOpenListButAllShowsIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"add", "ship weather"}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"done", id}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 {
		t.Fatalf("done code = %d, stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"list"}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 {
		t.Fatalf("list code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No items.") {
		t.Fatalf("open list = %q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"list", "--all"}, &stdout, &stderr, strings.NewReader(""), cwd); code != 0 {
		t.Fatalf("list all code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "[x] ship weather") {
		t.Fatalf("all list = %q", stdout.String())
	}
}

func TestRunYankCopiesTicketToClipboard(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := gitProjectDir(t, "lune")
	oldWriteClipboard := writeClipboard
	oldTmuxSession := tmuxSession
	oldWriteTmuxBuffer := writeTmuxBuffer
	t.Cleanup(func() {
		writeClipboard = oldWriteClipboard
		tmuxSession = oldTmuxSession
		writeTmuxBuffer = oldWriteTmuxBuffer
	})
	var copied string
	writeClipboard = func(value string) error {
		copied = value
		return nil
	}
	tmuxSession = func() bool { return false }
	writeTmuxBuffer = func(string, string) error {
		t.Fatal("tmux buffer should not be loaded outside tmux")
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "fix stuns", "--tag", "combat", "--body", "first line"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"edit", id, "--end", "appended detail"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("edit code = %d, stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"yank", id}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("yank code = %d, stderr=%q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "Yanked "+id+" for $lune-agent." {
		t.Fatalf("yank stdout = %q", got)
	}
	for _, want := range []string{
		"# Rune Ticket: fix stuns",
		"- ID: " + id,
		"- Status: open",
		"- Tags: #combat",
		"first line",
		"appended detail",
		"implement this ticket, $lune-agent\n",
	} {
		if !strings.Contains(copied, want) {
			t.Fatalf("copied ticket missing %q:\n%s", want, copied)
		}
	}
}

func TestRunYankCopiesTicketToTmuxBuffer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := gitProjectDir(t, "lune")
	oldWriteClipboard := writeClipboard
	oldTmuxSession := tmuxSession
	oldWriteTmuxBuffer := writeTmuxBuffer
	t.Cleanup(func() {
		writeClipboard = oldWriteClipboard
		tmuxSession = oldTmuxSession
		writeTmuxBuffer = oldWriteTmuxBuffer
	})
	var copied string
	writeClipboard = func(value string) error {
		copied = value
		return nil
	}
	tmuxSession = func() bool { return true }
	var tmuxName, tmuxText string
	writeTmuxBuffer = func(name, value string) error {
		tmuxName = name
		tmuxText = value
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "copy me", "--body", "tmux detail"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"yank", id}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("yank code = %d, stderr=%q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "Yanked "+id+" for $lune-agent. tmux buffer ready." {
		t.Fatalf("yank stdout = %q", got)
	}
	if tmuxName != "rune-ticket" {
		t.Fatalf("tmux buffer = %q", tmuxName)
	}
	if copied == "" || copied != tmuxText {
		t.Fatalf("clipboard/tmux mismatch:\nclipboard=%q\ntmux=%q", copied, tmuxText)
	}
	if !strings.Contains(tmuxText, "tmux detail") {
		t.Fatalf("tmux ticket missing detail:\n%s", tmuxText)
	}
}

func TestRunYankPrintsTicketWithoutClipboard(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := gitProjectDir(t, "lune")
	oldWriteClipboard := writeClipboard
	oldTmuxSession := tmuxSession
	oldWriteTmuxBuffer := writeTmuxBuffer
	t.Cleanup(func() {
		writeClipboard = oldWriteClipboard
		tmuxSession = oldTmuxSession
		writeTmuxBuffer = oldWriteTmuxBuffer
	})
	writeClipboard = func(string) error {
		t.Fatal("clipboard should not be written for --print")
		return nil
	}
	tmuxSession = func() bool { return true }
	writeTmuxBuffer = func(string, string) error {
		t.Fatal("tmux buffer should not be loaded for --print")
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "print me", "--body", "stdout detail"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"yank", "--print", id}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("yank --print code = %d, stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "# Rune Ticket: print me") || !strings.Contains(got, "stdout detail") {
		t.Fatalf("printed ticket = %q", got)
	}
	if strings.Contains(got, "Yanked ") {
		t.Fatalf("printed ticket included status: %q", got)
	}
}

func TestRunTicketPrintsTicket(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := gitProjectDir(t, "lune")

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "ticket me", "--tag", "agent", "--body", "ticket detail"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"ticket", id}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("ticket code = %d, stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"# Rune Ticket: ticket me",
		"- Tags: #agent",
		"ticket detail",
		"implement this ticket, $lune-agent\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ticket output missing %q:\n%s", want, got)
		}
	}
}

func TestRunTicketUsesProjectInstructionComment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()
	path := filepath.Join(home, "projects", "lune.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"# lune",
		"",
		"<!-- rune-ticket-instruction: review this ticket with $lune-reviewer -->",
		"",
		"- [ ] custom handoff",
		"<!-- rune:id=abc12345 type=task tags= created=2026-05-14T00:00:00Z -->",
		"detail",
	}, "\n")
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"ticket", "abc", "--project", "lune"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("ticket code = %d, stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "review this ticket with $lune-reviewer\n") || strings.Contains(got, "implement this ticket") {
		t.Fatalf("ticket output = %q", got)
	}
}

func TestRunCodexLaunchesTicket(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldRunCodex := runCodex
	t.Cleanup(func() { runCodex = oldRunCodex })
	var launchedCWD, launchedPrompt string
	var launchedOptions handoff.CodexOptions
	runCodex = func(cwd, prompt string, options handoff.CodexOptions, stdin io.Reader, stdout, stderr io.Writer) error {
		launchedCWD = cwd
		launchedPrompt = prompt
		launchedOptions = options
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "launch codex", "--body", "codex detail"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"codex", id}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("codex code = %d, stderr=%q", code, stderr.String())
	}
	if launchedCWD != cwd {
		t.Fatalf("codex cwd = %q, want %q", launchedCWD, cwd)
	}
	if !strings.Contains(launchedPrompt, "# Rune Ticket: launch codex") || !strings.Contains(launchedPrompt, "codex detail") {
		t.Fatalf("codex prompt = %q", launchedPrompt)
	}
	if launchedOptions.ReasoningEffort != handoff.CodexReasoningDefault {
		t.Fatalf("codex reasoning = %q, want default", launchedOptions.ReasoningEffort)
	}
	if stdout.Len() != 0 {
		t.Fatalf("codex stdout = %q", stdout.String())
	}
}

func TestRunCodexLaunchesTicketWithReasoning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldRunCodex := runCodex
	t.Cleanup(func() { runCodex = oldRunCodex })
	var launchedOptions handoff.CodexOptions
	runCodex = func(cwd, prompt string, options handoff.CodexOptions, stdin io.Reader, stdout, stderr io.Writer) error {
		launchedOptions = options
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "launch xhigh"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"codex", id, "--xhigh"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("codex code = %d, stderr=%q", code, stderr.String())
	}
	if launchedOptions.ReasoningEffort != handoff.CodexReasoningXHigh {
		t.Fatalf("codex reasoning = %q, want xhigh", launchedOptions.ReasoningEffort)
	}
}

func TestRunCodexRejectsConflictingReasoning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNE_HOME", home)
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldRunCodex := runCodex
	t.Cleanup(func() { runCodex = oldRunCodex })
	runCodex = func(cwd, prompt string, options handoff.CodexOptions, stdin io.Reader, stdout, stderr io.Writer) error {
		t.Fatal("codex should not launch")
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"add", "bad reasoning"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%q", code, stderr.String())
	}
	id := strings.Fields(stdout.String())[1]

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"codex", id, "--low", "--xhigh"}, &stdout, &stderr, strings.NewReader(""), cwd)
	if code == 0 {
		t.Fatalf("codex succeeded with conflicting reasoning")
	}
	if !strings.Contains(stderr.String(), "choose only one Codex reasoning effort") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunVersion(t *testing.T) {
	old := version
	version = "v1.2.3"
	t.Cleanup(func() { version = old })

	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, &stdout, &stderr, strings.NewReader(""), "")
	if code != 0 {
		t.Fatalf("code = %d, stderr=%q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "rune 1.2.3" {
		t.Fatalf("stdout = %q", got)
	}
}

func assertOutputOrder(t *testing.T, output string, values ...string) {
	t.Helper()
	previous := -1
	for _, value := range values {
		next := strings.Index(output, value)
		if next < 0 {
			t.Fatalf("output missing %q:\n%s", value, output)
		}
		if next <= previous {
			t.Fatalf("output order expected %q after previous item:\n%s", value, output)
		}
		previous = next
	}
}
