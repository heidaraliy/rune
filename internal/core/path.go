package core

import (
	"encoding/json"
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
	root, hasGitRoot := findGitRoot(abs)
	scope := Scope{Home: home, CWD: abs, Global: global, Project: cleanKey(project)}
	if scope.Project == "" {
		if hasGitRoot {
			scope.Project = cleanKey(filepath.Base(root))
		}
	}
	if strings.TrimSpace(os.Getenv("RUNE_HOME")) == "" {
		if localRoot, config, ok, err := findLocalStore(abs); err != nil {
			return Scope{}, err
		} else if ok {
			scope.Home = filepath.Join(localRoot, ".rune")
			scope.Layout = StoreLayoutFiles
			scope.ProjectRoot = localRoot
			if config.Layout != "" {
				scope.Layout = config.layout()
			}
			if scope.Project == "" && config.Project != "" {
				scope.Project = cleanKey(config.Project)
			}
			if cleanKey(project) == "" && config.Project != "" {
				scope.Project = cleanKey(config.Project)
			}
		} else if hasGitRoot {
			scope.ProjectRoot = root
		}
	} else if hasGitRoot {
		scope.ProjectRoot = root
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

type LocalConfig struct {
	Version int    `json:"version"`
	Project string `json:"project"`
	Layout  string `json:"layout"`
}

func defaultLocalConfig(project string) LocalConfig {
	return LocalConfig{
		Version: 1,
		Project: cleanKey(project),
		Layout:  string(StoreLayoutFiles),
	}
}

func (c LocalConfig) layout() StoreLayout {
	switch StoreLayout(strings.TrimSpace(c.Layout)) {
	case StoreLayoutFiles:
		return StoreLayoutFiles
	default:
		return StoreLayoutMarkdown
	}
}

func LocalStoreDir(root string) string {
	return filepath.Join(root, ".rune")
}

func LocalConfigPath(root string) string {
	return filepath.Join(LocalStoreDir(root), "config.json")
}

func ReadLocalConfig(root string) (LocalConfig, bool, error) {
	content, err := os.ReadFile(LocalConfigPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LocalConfig{}, false, nil
		}
		return LocalConfig{}, false, err
	}
	var config LocalConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return LocalConfig{}, false, err
	}
	config.Project = cleanKey(config.Project)
	config.Layout = strings.TrimSpace(config.Layout)
	if config.Layout == "" {
		config.Layout = string(StoreLayoutFiles)
	}
	if config.layout() != StoreLayoutFiles {
		return LocalConfig{}, false, fmt.Errorf("unsupported rune layout %q in %s", config.Layout, LocalConfigPath(root))
	}
	return config, true, nil
}

func findLocalStore(start string) (string, LocalConfig, bool, error) {
	dir := start
	for {
		config, ok, err := ReadLocalConfig(dir)
		if err != nil {
			return "", LocalConfig{}, false, err
		}
		if ok {
			return dir, config, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", LocalConfig{}, false, nil
		}
		dir = parent
	}
}

func ProjectRoot(cwd string) (string, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	if root, ok := findGitRoot(abs); ok {
		return root, nil
	}
	return abs, nil
}

func InitLocalStore(cwd, project string) (LocalConfig, string, error) {
	root, err := ProjectRoot(cwd)
	if err != nil {
		return LocalConfig{}, "", err
	}
	project = cleanKey(project)
	if project == "" {
		project = cleanKey(filepath.Base(root))
	}
	if project == "" {
		return LocalConfig{}, "", errors.New("project is required")
	}
	config := defaultLocalConfig(project)
	dir := LocalStoreDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return LocalConfig{}, "", err
	}
	if err := EnsureStoreLayout(dir, StoreLayoutFiles); err != nil {
		return LocalConfig{}, "", err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return LocalConfig{}, "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(LocalConfigPath(root), data, 0o644); err != nil {
		return LocalConfig{}, "", err
	}
	return config, dir, nil
}

func ProjectPath(home, project string) string {
	return filepath.Join(home, "projects", cleanKey(project)+".md")
}

func ProjectDir(home, project string) string {
	return filepath.Join(home, "projects", cleanKey(project))
}

func ProjectItemPath(home, project, id, title string) string {
	return filepath.Join(ProjectDir(home, project), noteFileName(id, title))
}

func ProjectItemPathWithPrefix(home, project, prefix, id, title string) string {
	return filepath.Join(ProjectDir(home, project), noteFileNameWithPrefix(prefix, id, title))
}

func ArchivePath(home string, now time.Time) string {
	year, week := now.ISOWeek()
	return filepath.Join(home, "archive", fmt.Sprintf("%04d-W%02d.md", year, week))
}

func ArchiveDir(home, project string, now time.Time) string {
	year, week := now.ISOWeek()
	return filepath.Join(home, "archive", cleanKey(project), fmt.Sprintf("%04d-W%02d", year, week))
}

func ArchiveItemPath(home, project, id, title string, now time.Time) string {
	return filepath.Join(ArchiveDir(home, project, now), noteFileName(id, title))
}

func EnsureStore(home string) error {
	return EnsureStoreLayout(home, StoreLayoutMarkdown)
}

func EnsureStoreLayout(home string, layout StoreLayout) error {
	if strings.TrimSpace(home) == "" {
		return errors.New("empty rune home")
	}
	for _, dir := range []string{
		home,
		filepath.Join(home, "projects"),
		filepath.Join(home, "archive"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func noteFileName(id, title string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	slug := cleanKey(title)
	if slug == "" {
		return id + ".md"
	}
	if len(slug) > 48 {
		slug = strings.Trim(slug[:48], "-")
	}
	if id == "" {
		return slug + ".md"
	}
	return id + "-" + slug + ".md"
}

func noteFileNameWithPrefix(prefix, id, title string) string {
	prefix = cleanKey(prefix)
	name := noteFileName(id, title)
	if prefix == "" {
		return name
	}
	return prefix + "-" + name
}

func createdFilePrefix(created time.Time) string {
	created = created.UTC()
	return created.Format("20060102150405") + fmt.Sprintf("%09d", created.Nanosecond())
}
