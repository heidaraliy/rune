package handoff

import (
	"errors"
	"strings"
	"testing"
)

func TestYankTicketCopiesClipboardAndTmuxBuffer(t *testing.T) {
	var clipboardText, tmuxName, tmuxText string
	result, err := YankTicket(
		"ticket text",
		func(value string) error {
			clipboardText = value
			return nil
		},
		true,
		func(name, value string) error {
			tmuxName = name
			tmuxText = value
			return nil
		},
	)
	if err != nil {
		t.Fatalf("YankTicket error = %v", err)
	}
	if clipboardText != "ticket text" || tmuxText != "ticket text" {
		t.Fatalf("copied values = clipboard %q, tmux %q", clipboardText, tmuxText)
	}
	if !result.Copied || !result.TmuxAttempted || !result.TmuxCopied || result.TmuxBuffer != TmuxTicketBuffer {
		t.Fatalf("result = %#v", result)
	}
	if tmuxName != TmuxTicketBuffer {
		t.Fatalf("tmux buffer = %q", tmuxName)
	}
}

func TestYankTicketTreatsTmuxCopyAsBestEffort(t *testing.T) {
	tmuxErr := errors.New("tmux unavailable")
	result, err := YankTicket(
		"ticket text",
		func(string) error { return nil },
		true,
		func(string, string) error { return tmuxErr },
	)
	if err != nil {
		t.Fatalf("YankTicket error = %v", err)
	}
	if !result.Copied || !result.TmuxAttempted || result.TmuxCopied || !errors.Is(result.TmuxErr, tmuxErr) {
		t.Fatalf("result = %#v", result)
	}
	status := YankStatus("abc", "$rune-agent", result)
	if !strings.Contains(status, "Yanked abc for $rune-agent.") || !strings.Contains(status, "tmux buffer unavailable") {
		t.Fatalf("status = %q", status)
	}
}

func TestYankTicketReturnsClipboardFailure(t *testing.T) {
	clipboardErr := errors.New("clipboard unavailable")
	_, err := YankTicket(
		"ticket text",
		func(string) error { return clipboardErr },
		true,
		func(string, string) error {
			t.Fatal("tmux should not be attempted after clipboard failure")
			return nil
		},
	)
	if !errors.Is(err, clipboardErr) {
		t.Fatalf("YankTicket error = %v", err)
	}
}

func TestCodexCommandArgsIncludesReasoningOverride(t *testing.T) {
	args := CodexCommandArgs("ticket prompt", CodexOptions{ReasoningEffort: CodexReasoningXHigh})
	want := []string{"-c", `model_reasoning_effort="xhigh"`, "ticket prompt"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("CodexCommandArgs = %#v, want %#v", args, want)
	}
}

func TestParseCodexReasoningEffort(t *testing.T) {
	effort, err := ParseCodexReasoningEffort("HIGH")
	if err != nil {
		t.Fatalf("ParseCodexReasoningEffort error = %v", err)
	}
	if effort != CodexReasoningHigh {
		t.Fatalf("effort = %q, want high", effort)
	}
	if _, err := ParseCodexReasoningEffort("huge"); err == nil {
		t.Fatal("ParseCodexReasoningEffort accepted unsupported value")
	}
}
