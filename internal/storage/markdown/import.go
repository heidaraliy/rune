package markdown

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/heidaraliy/rune/internal/core"
	"github.com/heidaraliy/rune/internal/domain"
)

type ImportItem struct {
	Entity         domain.Entity
	ParentLegacyID string
}

type Bundle struct {
	SourcePath string
	Project    string
	Items      []ImportItem
	Warnings   []string
}

func ReadFile(path, project string, now time.Time) (Bundle, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Bundle{}, err
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return Bundle{}, err
	}
	project = normalizeProject(project)
	if project == "" {
		project = strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
		project = normalizeProject(project)
	}
	doc, err := core.ParseDocument(absPath, "project", project, content)
	if err != nil {
		return Bundle{}, fmt.Errorf("parse Markdown %s: %w", absPath, err)
	}

	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	bundle := Bundle{SourcePath: absPath, Project: project}
	legacySeen := make(map[string]struct{}, len(doc.Items))
	parents := make(map[int]string)
	for _, item := range doc.Items {
		legacyID := legacyID(absPath, item)
		if _, exists := legacySeen[legacyID]; exists {
			return Bundle{}, fmt.Errorf("duplicate legacy item id %q in %s near line %d", legacyID, absPath, item.Line+1)
		}
		legacySeen[legacyID] = struct{}{}
		if strings.TrimSpace(item.ID) == "" {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("assigned deterministic legacy id %q near line %d", legacyID, item.Line+1))
		}
		for depth := range parents {
			if depth >= item.Depth {
				delete(parents, depth)
			}
		}
		parentLegacyID := ""
		if item.Depth > 0 {
			parentLegacyID = parents[item.Depth-1]
		}
		createdAt := item.Created
		if createdAt.IsZero() {
			createdAt = now
		}
		entityID, err := domain.NewID()
		if err != nil {
			return Bundle{}, err
		}
		entity := domain.Entity{
			ID:           entityID,
			Kind:         domain.KindNote,
			WorkspaceID:  "local",
			Project:      project,
			Title:        item.Title,
			Body:         item.Body(),
			Heading:      item.Heading,
			Tags:         domain.NormalizeTags(item.Tags),
			LegacyID:     legacyID,
			LegacySource: absPath,
			CreatedAt:    createdAt.UTC(),
			UpdatedAt:    now,
			Revision:     1,
		}
		if item.Type == core.ItemTask {
			entity.Kind = domain.KindTask
			entity.Status = domain.StatusDraft
			if item.Done {
				entity.Status = domain.StatusCompleted
			}
			if !item.Finished.IsZero() {
				finishedAt := item.Finished.UTC()
				entity.FinishedAt = &finishedAt
			}
		}
		if err := entity.Validate(); err != nil {
			return Bundle{}, fmt.Errorf("legacy item %q: %w", legacyID, err)
		}
		bundle.Items = append(bundle.Items, ImportItem{Entity: entity, ParentLegacyID: parentLegacyID})
		parents[item.Depth] = legacyID
	}
	return bundle, nil
}

func legacyID(source string, item *core.Item) string {
	if item != nil && strings.TrimSpace(item.ID) != "" {
		return strings.TrimSpace(item.ID)
	}
	title := ""
	line := -1
	if item != nil {
		title = item.Title
		line = item.Line
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", source, line, title)))
	return "generated-" + hex.EncodeToString(digest[:8])
}

func normalizeProject(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', r == '.', r == ' ':
			if !lastDash {
				out.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(out.String(), "-")
}
