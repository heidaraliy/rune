package core

import (
	"strings"
	"testing"
)

func TestYankOptionsDerivesAgentFromProjectDocument(t *testing.T) {
	doc, err := ParseDocument("/notes/projects/lune.md", "project", "lune", []byte(strings.Join([]string{
		"# lune",
		"",
		"- [ ] selected ticket",
		"<!-- rune:id=abc12345 type=task tags= created=2026-05-14T00:00:00Z -->",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	item := doc.Items[0]
	item.DisplayID = "abc"

	options := YankOptionsForItem(item)
	if options.Agent != "$lune-agent" {
		t.Fatalf("agent = %q, want $lune-agent", options.Agent)
	}
	text := YankTicketText(item, "/notes")
	if !strings.Contains(text, "implement this ticket, $lune-agent\n") {
		t.Fatalf("ticket text = %q", text)
	}
}

func TestYankOptionsReadsProjectInstructionComment(t *testing.T) {
	doc, err := ParseDocument("/notes/projects/lune.md", "project", "lune", []byte(strings.Join([]string{
		"# lune",
		"",
		"<!-- rune-ticket-instruction: review this ticket with $lune-reviewer -->",
		"",
		"- [ ] selected ticket",
		"<!-- rune:id=abc12345 type=task tags= created=2026-05-14T00:00:00Z -->",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	item := doc.Items[0]
	item.DisplayID = "abc"

	options := YankOptionsForItem(item)
	if options.Agent != "$lune-reviewer" {
		t.Fatalf("agent = %q, want $lune-reviewer", options.Agent)
	}
	if options.Instruction != "review this ticket with $lune-reviewer\n" {
		t.Fatalf("instruction = %q", options.Instruction)
	}
	text := YankTicketText(item, "/notes")
	if !strings.Contains(text, "review this ticket with $lune-reviewer\n") || strings.Contains(text, "implement this ticket") {
		t.Fatalf("ticket text = %q", text)
	}
}

func TestYankOptionsReadsProjectAgentComment(t *testing.T) {
	doc, err := ParseDocument("/notes/projects/lune.md", "project", "lune", []byte(strings.Join([]string{
		"# lune",
		"",
		"<!-- rune-ticket-agent: $lune-build-agent -->",
		"",
		"- [ ] selected ticket",
		"<!-- rune:id=abc12345 type=task tags= created=2026-05-14T00:00:00Z -->",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}

	options := YankOptionsForItem(doc.Items[0])
	if options.Agent != "$lune-build-agent" {
		t.Fatalf("agent = %q, want $lune-build-agent", options.Agent)
	}
	if options.Instruction != "implement this ticket, $lune-build-agent\n" {
		t.Fatalf("instruction = %q", options.Instruction)
	}
}

func TestYankOptionsFallsBackForArchiveDocument(t *testing.T) {
	doc, err := ParseDocument("/notes/archive/2026-W20.md", "archive", "2026-W20", []byte(strings.Join([]string{
		"# archive",
		"",
		"## lune - 2026-05-16",
		"",
		"- [x] archived ticket",
		"<!-- rune:id=abc12345 type=task tags= created=2026-05-14T00:00:00Z -->",
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}

	options := YankOptionsForItem(doc.Items[0])
	if options.Agent != "$rune-agent" {
		t.Fatalf("agent = %q, want $rune-agent", options.Agent)
	}
	if options.Instruction != YankInstruction {
		t.Fatalf("instruction = %q", options.Instruction)
	}
}
