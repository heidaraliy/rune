package core

import (
	"path/filepath"
	"strings"
	"time"
)

const (
	ItemTask = "task"
	ItemNote = "note"
)

type Scope struct {
	Home    string
	CWD     string
	Project string
	Global  bool
	Inbox   bool
	Today   bool
}

type Document struct {
	Path    string
	Kind    string
	Key     string
	Title   string
	Lines   []string
	Items   []*Item
	changed bool
}

type Item struct {
	ID        string
	DisplayID string
	Type      string
	Title     string
	Done      bool
	Tags      []string
	Created   time.Time
	Heading   string
	Depth     int
	Project   string
	Source    string
	Line      int
	MetaLine  int
	BodyStart int
	BodyEnd   int
	Doc       *Document `json:"-"`
}

type AddOptions struct {
	Title   string
	Body    string
	Tags    []string
	AsNote  bool
	Created time.Time
}

type ListOptions struct {
	All     bool
	Done    bool
	Tag     string
	Query   string
	Global  bool
	Project string
}

type EditOptions struct {
	Title       string
	Append      string
	ReplaceBody string
	Tags        []string
	Untags      []string
}

func (i Item) IsDone() bool {
	return i.Type == ItemTask && i.Done
}

func (i Item) Body() string {
	if i.Doc == nil || i.BodyStart < 0 || i.BodyEnd < i.BodyStart || i.BodyEnd > len(i.Doc.Lines) {
		return ""
	}
	lines := append([]string(nil), i.Doc.Lines[i.BodyStart:i.BodyEnd]...)
	for idx, line := range lines {
		lines[idx] = strings.TrimPrefix(line, "  ")
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func (d Document) RelPath(home string) string {
	if rel, err := filepath.Rel(home, d.Path); err == nil {
		return rel
	}
	return d.Path
}
