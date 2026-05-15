package handoff

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const TmuxTicketBuffer = "rune-ticket"

type ClipboardWriter func(string) error
type TmuxBufferWriter func(string, string) error

type YankResult struct {
	Copied        bool
	TmuxAttempted bool
	TmuxCopied    bool
	TmuxBuffer    string
	TmuxErr       error
}

func YankTicket(text string, writeClipboard ClipboardWriter, tmuxActive bool, writeTmuxBuffer TmuxBufferWriter) (YankResult, error) {
	if writeClipboard == nil {
		return YankResult{}, fmt.Errorf("clipboard writer unavailable")
	}
	if err := writeClipboard(text); err != nil {
		return YankResult{}, err
	}
	result := YankResult{Copied: true}
	if !tmuxActive {
		return result, nil
	}
	result.TmuxAttempted = true
	result.TmuxBuffer = TmuxTicketBuffer
	if writeTmuxBuffer == nil {
		result.TmuxErr = fmt.Errorf("tmux buffer writer unavailable")
		return result, nil
	}
	if err := writeTmuxBuffer(TmuxTicketBuffer, text); err != nil {
		result.TmuxErr = err
		return result, nil
	}
	result.TmuxCopied = true
	return result, nil
}

func YankStatus(id, agent string, result YankResult) string {
	status := fmt.Sprintf("Yanked %s for %s.", id, agent)
	if result.TmuxCopied {
		return status + " tmux buffer ready."
	}
	if result.TmuxErr != nil {
		return status + " tmux buffer unavailable: " + result.TmuxErr.Error()
	}
	return status
}

func IsTmuxSession() bool {
	return strings.TrimSpace(os.Getenv("TMUX")) != ""
}

func LoadTmuxBuffer(name, text string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("empty tmux buffer name")
	}
	cmd := exec.Command("tmux", "load-buffer", "-b", name, "-")
	cmd.Stdin = strings.NewReader(text)
	if output, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, message)
	}
	return nil
}

func RunCodex(cwd, prompt string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command("codex", prompt)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
