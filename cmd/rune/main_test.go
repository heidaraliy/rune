package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	runCodex = func(cwd, prompt string, stdin io.Reader, stdout, stderr io.Writer) error {
		launchedCWD = cwd
		launchedPrompt = prompt
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
	if stdout.Len() != 0 {
		t.Fatalf("codex stdout = %q", stdout.String())
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
