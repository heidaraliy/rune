package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

type tuiState struct {
	Collapsed []string `json:"collapsed"`
}

func tuiStatePath(home string) string {
	return filepath.Join(home, ".rune", "tui-state.json")
}

func loadCollapsedState(home string) (map[string]bool, error) {
	collapsed := make(map[string]bool)
	if home == "" {
		return collapsed, nil
	}
	content, err := os.ReadFile(tuiStatePath(home))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return collapsed, nil
		}
		return nil, err
	}
	var state tuiState
	if err := json.Unmarshal(content, &state); err != nil {
		return collapsed, nil
	}
	for _, id := range state.Collapsed {
		if id != "" {
			collapsed[id] = true
		}
	}
	return collapsed, nil
}

func (m Model) saveCollapsedState() error {
	if m.scope.Home == "" {
		return nil
	}
	ids := make([]string, 0, len(m.collapsed))
	for id, isCollapsed := range m.collapsed {
		if isCollapsed {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	content, err := json.MarshalIndent(tuiState{Collapsed: ids}, "", "  ")
	if err != nil {
		return err
	}
	path := tuiStatePath(m.scope.Home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o644)
}
