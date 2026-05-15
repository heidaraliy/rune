package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldWriteClipboard := writeClipboard
	t.Cleanup(func() { writeClipboard = oldWriteClipboard })
	var copied string
	writeClipboard = func(value string) error {
		copied = value
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
	if got := strings.TrimSpace(stdout.String()); got != "Yanked "+id+" for $rune-agent." {
		t.Fatalf("yank stdout = %q", got)
	}
	for _, want := range []string{
		"# Rune Ticket: fix stuns",
		"- ID: " + id,
		"- Status: open",
		"- Tags: #combat",
		"first line",
		"appended detail",
		"implement this ticket, $rune-agent\n",
	} {
		if !strings.Contains(copied, want) {
			t.Fatalf("copied ticket missing %q:\n%s", want, copied)
		}
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
