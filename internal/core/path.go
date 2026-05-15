package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Home() (string, error) {
	if home := strings.TrimSpace(os.Getenv("RUNE_HOME")); home != "" {
		return filepath.Abs(home)
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userHome, "notes"), nil
}

func ResolveScope(cwd string, global bool, project string) (Scope, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return Scope{}, err
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return Scope{}, err
	}
	home, err := Home()
	if err != nil {
		return Scope{}, err
	}
	scope := Scope{Home: home, CWD: abs, Global: global, Project: cleanKey(project)}
	if scope.Project == "" {
		if root, ok := findGitRoot(abs); ok {
			scope.Project = cleanKey(filepath.Base(root))
		}
	}
	if scope.Project == "" && !global {
		scope.Inbox = true
	}
	return scope, nil
}

func cleanKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			lastDash = false
		case r == '_' || r == '-' || r == '.' || r == ' ':
			if !lastDash {
				out.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(out.String(), "-")
}

func findGitRoot(start string) (string, bool) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func ProjectPath(home, project string) string {
	return filepath.Join(home, "projects", cleanKey(project)+".md")
}

func InboxPath(home string) string {
	return filepath.Join(home, "inbox.md")
}

func TodayPath(home string, now time.Time) string {
	return filepath.Join(home, "today", now.Format("2006-01-02")+".md")
}

func ArchivePath(home string, now time.Time) string {
	year, week := now.ISOWeek()
	return filepath.Join(home, "archive", fmt.Sprintf("%04d-W%02d.md", year, week))
}

func EnsureStore(home string) error {
	if strings.TrimSpace(home) == "" {
		return errors.New("empty rune home")
	}
	for _, dir := range []string{
		home,
		filepath.Join(home, "projects"),
		filepath.Join(home, "today"),
		filepath.Join(home, "archive"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
