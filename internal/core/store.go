package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	Home string
	Now  func() time.Time
}

func NewStore(home string) Store {
	return Store{Home: home, Now: func() time.Time { return time.Now().UTC() }}
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Store) LoadScope(scope Scope) ([]*Document, error) {
	if err := EnsureStore(scope.Home); err != nil {
		return nil, err
	}
	var docs []*Document
	switch {
	case scope.Global:
		return s.LoadAll()
	case scope.Project == "":
		return nil, errors.New("project context required; run from a git project or pass --project")
	default:
		doc, err := s.LoadPath(ProjectPath(scope.Home, scope.Project), "project", scope.Project, scope.Project)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	s.applyDisplay(docs)
	return docs, nil
}

func (s Store) LoadAll() ([]*Document, error) {
	if err := EnsureStore(s.Home); err != nil {
		return nil, err
	}
	var docs []*Document
	for _, root := range []struct {
		dir  string
		kind string
	}{
		{filepath.Join(s.Home, "projects"), "project"},
		{filepath.Join(s.Home, "archive"), "archive"},
	} {
		entries, err := os.ReadDir(root.dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			key := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			doc, err := s.LoadPath(filepath.Join(root.dir, entry.Name()), root.kind, key, key)
			if err != nil {
				return nil, err
			}
			docs = append(docs, doc)
		}
	}
	s.applyDisplay(docs)
	return docs, nil
}

func (s Store) LoadPath(path, kind, key, title string) (*Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewDocument(path, kind, key, title), nil
		}
		return nil, err
	}
	doc, err := ParseDocument(path, kind, key, content)
	if err != nil {
		return nil, err
	}
	if doc.Title == "" {
		doc.Title = title
	}
	return doc, nil
}

func (s Store) Save(doc *Document) error {
	if doc == nil || !doc.changed {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(doc.Path), 0o755); err != nil {
		return err
	}
	content := strings.Join(doc.Lines, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(doc.Path, []byte(content), 0o644)
}

func (s Store) SaveAll(docs []*Document) error {
	for _, doc := range docs {
		if err := s.Save(doc); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) assignMissingIDs(doc *Document, items []*Item, existing map[string]bool) error {
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.ID != "" {
			continue
		}
		id, err := NewID(existing)
		if err != nil {
			return err
		}
		existing[id] = true
		item.ID = id
		if item.Created.IsZero() {
			item.Created = s.now()
		}
		doc.updateMeta(item)
	}
	return nil
}

func (s Store) Add(scope Scope, opts AddOptions) (*Item, error) {
	if opts.Title = strings.TrimSpace(opts.Title); opts.Title == "" {
		return nil, errors.New("title is required")
	}
	docs, err := s.LoadScope(scope)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, errors.New("no document loaded")
	}
	doc := docs[0]
	allDocs, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	existing := existingIDs(allDocs)
	id, err := NewID(existing)
	if err != nil {
		return nil, err
	}
	created := opts.Created.UTC()
	if created.IsZero() {
		created = s.now()
	}
	itemType := ItemTask
	if opts.AsNote {
		itemType = ItemNote
	}
	item := &Item{
		ID:      id,
		Type:    itemType,
		Title:   opts.Title,
		Tags:    normalizeTags(opts.Tags),
		Created: created,
		Project: doc.Key,
		Source:  doc.Path,
		Doc:     doc,
	}
	doc.appendItem(item, opts.Body)
	inserted := findByID(doc, id)
	if inserted == nil {
		return nil, errors.New("inserted item could not be found")
	}
	existing[id] = true
	if err := s.assignMissingIDs(doc, doc.subtreeItems(inserted), existing); err != nil {
		return nil, err
	}
	inserted = findByID(doc, id)
	s.applyDisplay([]*Document{doc})
	if err := s.Save(doc); err != nil {
		return nil, err
	}
	return inserted, nil
}

func (s Store) AddNear(scope Scope, anchorID string, above bool, opts AddOptions, global bool) (*Item, error) {
	if strings.TrimSpace(anchorID) == "" {
		return s.Add(scope, opts)
	}
	if opts.Title = strings.TrimSpace(opts.Title); opts.Title == "" {
		return nil, errors.New("title is required")
	}
	anchor, _, err := s.Resolve(scope, anchorID, global)
	if err != nil {
		return nil, err
	}
	allDocs, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	id, err := NewID(existingIDs(allDocs))
	if err != nil {
		return nil, err
	}
	created := opts.Created.UTC()
	if created.IsZero() {
		created = s.now()
	}
	itemType := ItemTask
	if opts.AsNote {
		itemType = ItemNote
	}
	item := &Item{
		ID:      id,
		Type:    itemType,
		Title:   opts.Title,
		Tags:    normalizeTags(opts.Tags),
		Created: created,
		Depth:   anchor.Depth,
		Project: anchor.Project,
		Source:  anchor.Source,
		Doc:     anchor.Doc,
	}
	anchor.Doc.insertItemNear(anchor, item, opts.Body, above)
	inserted := findByID(anchor.Doc, id)
	if inserted == nil {
		return nil, errors.New("inserted item could not be found")
	}
	existing := existingIDs(allDocs)
	existing[id] = true
	if err := s.assignMissingIDs(anchor.Doc, anchor.Doc.subtreeItems(inserted), existing); err != nil {
		return nil, err
	}
	inserted = findByID(anchor.Doc, id)
	ApplyDisplayIDs(anchor.Doc.Items)
	if err := s.Save(anchor.Doc); err != nil {
		return nil, err
	}
	return inserted, nil
}

func (s Store) Items(scope Scope, opts ListOptions) ([]*Item, []*Document, error) {
	if opts.Global {
		scope.Global = true
	}
	if opts.Project != "" {
		scope.Project = cleanKey(opts.Project)
		scope.Global = false
	}
	docs, err := s.LoadScope(scope)
	if err != nil {
		return nil, nil, err
	}
	var items []*Item
	for _, doc := range docs {
		items = append(items, doc.Items...)
	}
	ApplyDisplayIDs(items)
	items = FilterItems(items, opts)
	return items, docs, nil
}

func FilterItems(items []*Item, opts ListOptions) []*Item {
	tag := cleanKey(opts.Tag)
	query := strings.ToLower(strings.TrimSpace(opts.Query))
	var out []*Item
	for _, item := range items {
		done := itemEffectivelyDone(item)
		if !opts.All {
			if opts.Done {
				if !done {
					continue
				}
			} else if done {
				continue
			}
		}
		if tag != "" && !hasTag(item.Tags, tag) {
			continue
		}
		if query != "" && !itemMatches(item, query) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func itemEffectivelyDone(item *Item) bool {
	if item == nil {
		return false
	}
	if item.IsDone() {
		return true
	}
	if item.Doc == nil {
		return false
	}
	return itemHasDoneTaskAncestor(item)
}

func itemHasDoneTaskAncestor(item *Item) bool {
	if item == nil || item.Doc == nil {
		return false
	}
	for _, candidate := range item.Doc.Items {
		if candidate.Line >= item.Line {
			break
		}
		if candidate.Type == ItemTask && candidate.Done && candidate.Depth < item.Depth {
			covered := true
			for _, between := range item.Doc.Items {
				if between.Line <= candidate.Line {
					continue
				}
				if between.Line >= item.Line {
					break
				}
				if between.Depth <= candidate.Depth {
					covered = false
					break
				}
			}
			if covered {
				return true
			}
		}
	}
	return false
}

func itemMatches(item *Item, query string) bool {
	if strings.Contains(strings.ToLower(item.Title), query) ||
		strings.Contains(strings.ToLower(item.Body()), query) ||
		strings.Contains(strings.ToLower(item.Heading), query) ||
		strings.Contains(strings.ToLower(strings.Join(item.Tags, ",")), query) {
		return true
	}
	return false
}

func (s Store) Resolve(scope Scope, prefix string, global bool) (*Item, []*Document, error) {
	scope.Global = global
	docs, err := s.LoadScope(scope)
	if err != nil {
		return nil, nil, err
	}
	var items []*Item
	for _, doc := range docs {
		items = append(items, doc.Items...)
	}
	ApplyDisplayIDs(items)
	item, err := ResolvePrefix(items, prefix)
	return item, docs, err
}

func (s Store) Edit(scope Scope, prefix string, opts EditOptions, global bool) (*Item, error) {
	item, _, err := s.Resolve(scope, prefix, global)
	if err != nil {
		return nil, err
	}
	doc := item.Doc
	changedBody := false
	if opts.Title != "" {
		item.Title = strings.TrimSpace(opts.Title)
		doc.updateItemLine(item)
		item = findByID(doc, item.ID)
	}
	if opts.Append != "" {
		doc.appendBody(item, opts.Append)
		item = findByID(doc, item.ID)
		changedBody = true
	}
	if opts.ReplaceBody != "" {
		doc.replaceBody(item, opts.ReplaceBody)
		item = findByID(doc, item.ID)
		changedBody = true
	}
	if len(opts.Tags) > 0 || len(opts.Untags) > 0 {
		tags := make(map[string]bool)
		for _, tag := range item.Tags {
			tags[tag] = true
		}
		for _, tag := range normalizeTags(opts.Tags) {
			tags[tag] = true
		}
		for _, tag := range normalizeTags(opts.Untags) {
			delete(tags, tag)
		}
		item.Tags = nil
		for tag := range tags {
			item.Tags = append(item.Tags, tag)
		}
		item.Tags = normalizeTags(item.Tags)
		doc.updateMeta(item)
		item = findByID(doc, item.ID)
	}
	if changedBody {
		allDocs, err := s.LoadAll()
		if err != nil {
			return nil, err
		}
		if err := s.assignMissingIDs(doc, doc.subtreeItems(item), existingIDs(allDocs)); err != nil {
			return nil, err
		}
		item = findByID(doc, item.ID)
	}
	if err := s.Save(doc); err != nil {
		return nil, err
	}
	ApplyDisplayIDs(doc.Items)
	return item, nil
}

func (s Store) SetDone(scope Scope, prefix string, done bool, toggle bool, global bool) (*Item, error) {
	item, _, err := s.Resolve(scope, prefix, global)
	if err != nil {
		return nil, err
	}
	if item.Type != ItemTask {
		return nil, fmt.Errorf("%s is not a task", item.DisplayID)
	}
	if toggle {
		item.Done = !item.Done
	} else {
		item.Done = done
	}
	item.Doc.updateItemLine(item)
	if item.Done {
		item = findByID(item.Doc, item.ID)
		for _, child := range item.Doc.taskDescendants(item) {
			if !child.Done {
				item.Doc.setTaskDone(child, true)
			}
		}
	}
	if err := s.Save(item.Doc); err != nil {
		return nil, err
	}
	item = findByID(item.Doc, item.ID)
	ApplyDisplayIDs(item.Doc.Items)
	return item, nil
}

func (s Store) ArchiveDone(scope Scope) (int, string, error) {
	if scope.Project == "" {
		return 0, "", errors.New("archive requires a project context or --project")
	}
	doc, err := s.LoadPath(ProjectPath(scope.Home, scope.Project), "project", scope.Project, scope.Project)
	if err != nil {
		return 0, "", err
	}
	var done []*Item
	for _, item := range doc.Items {
		if item.IsDone() {
			done = append(done, item)
		}
	}
	if len(done) == 0 {
		return 0, "", nil
	}
	var roots []*Item
	for _, item := range done {
		if !itemHasDoneTaskAncestor(item) {
			roots = append(roots, item)
		}
	}
	blocks := doc.removeSubtrees(roots)
	archivePath := ArchivePath(scope.Home, s.now())
	archive, err := s.LoadPath(archivePath, "archive", "archive", "Archive")
	if err != nil {
		return 0, "", err
	}
	if len(archive.Lines) > 0 && strings.TrimSpace(archive.Lines[len(archive.Lines)-1]) != "" {
		archive.Lines = append(archive.Lines, "")
	}
	archive.Lines = append(archive.Lines, "## "+scope.Project+" - "+s.now().Format("2006-01-02"), "")
	for i := len(blocks) - 1; i >= 0; i-- {
		archive.Lines = append(archive.Lines, blocks[i]...)
		if len(archive.Lines) == 0 || strings.TrimSpace(archive.Lines[len(archive.Lines)-1]) != "" {
			archive.Lines = append(archive.Lines, "")
		}
	}
	archive.changed = true
	parseItems(archive)
	if err := s.Save(doc); err != nil {
		return 0, "", err
	}
	if err := s.Save(archive); err != nil {
		return 0, "", err
	}
	return len(done), archive.Path, nil
}

func (s Store) RestoreArchivedProject(scope Scope) (int, []string, error) {
	if scope.Project == "" {
		return 0, nil, errors.New("restore requires a project context or --project")
	}
	project := cleanKey(scope.Project)
	target, err := s.LoadPath(ProjectPath(scope.Home, project), "project", project, project)
	if err != nil {
		return 0, nil, err
	}
	archiveDir := filepath.Join(scope.Home, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	total := 0
	var touched []string
	var changedArchives []*Document
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		path := filepath.Join(archiveDir, entry.Name())
		archive, err := s.LoadPath(path, "archive", "archive", "Archive")
		if err != nil {
			return total, touched, err
		}
		remaining, sections := extractProjectArchiveSections(archive.Lines, project)
		if len(sections) == 0 {
			continue
		}
		for _, section := range sections {
			lines := trimBlankLines(section.Lines)
			if len(lines) == 0 {
				continue
			}
			if len(target.Lines) > 0 && strings.TrimSpace(target.Lines[len(target.Lines)-1]) != "" {
				target.Lines = append(target.Lines, "")
			}
			target.Lines = append(target.Lines, "## Done "+section.Date, "")
			target.Lines = append(target.Lines, lines...)
			if strings.TrimSpace(target.Lines[len(target.Lines)-1]) != "" {
				target.Lines = append(target.Lines, "")
			}
			total += section.Count
		}
		archive.Lines = trimTrailingBlankLines(remaining)
		archive.changed = true
		parseItems(archive)
		changedArchives = append(changedArchives, archive)
		touched = append(touched, path)
	}
	if total == 0 {
		return 0, touched, nil
	}
	target.changed = true
	parseItems(target)
	if err := s.Save(target); err != nil {
		return total, touched, err
	}
	for _, archive := range changedArchives {
		if err := s.Save(archive); err != nil {
			return total, touched, err
		}
	}
	return total, touched, nil
}

type archivedProjectSection struct {
	Date  string
	Lines []string
	Count int
}

func extractProjectArchiveSections(lines []string, project string) ([]string, []archivedProjectSection) {
	var out []string
	var sections []archivedProjectSection
	for i := 0; i < len(lines); {
		date, ok := archiveProjectDate(lines[i], project)
		if !ok {
			out = append(out, lines[i])
			i++
			continue
		}
		end := i + 1
		for end < len(lines) && !strings.HasPrefix(lines[end], "## ") {
			end++
		}
		body := append([]string(nil), lines[i+1:end]...)
		sections = append(sections, archivedProjectSection{
			Date:  date,
			Lines: body,
			Count: countItemLines(body),
		})
		i = end
	}
	return out, sections
}

func archiveProjectDate(line, project string) (string, bool) {
	if !strings.HasPrefix(line, "## ") {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
	prefix := project + " - "
	if !strings.HasPrefix(title, prefix) {
		return "", false
	}
	date := strings.TrimSpace(strings.TrimPrefix(title, prefix))
	if date == "" {
		date = "archive"
	}
	return date, true
}

func countItemLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if _, _, _, _, ok := parseItemLine(line); ok {
			count++
		}
	}
	return count
}

func trimBlankLines(lines []string) []string {
	out := append([]string(nil), lines...)
	for len(out) > 0 && strings.TrimSpace(out[0]) == "" {
		out = out[1:]
	}
	return trimTrailingBlankLines(out)
}

func trimTrailingBlankLines(lines []string) []string {
	out := append([]string(nil), lines...)
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return out
}

func (s Store) Import(path, project string) (int, string, error) {
	project = cleanKey(project)
	if project == "" {
		return 0, "", errors.New("--project is required")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	imported, err := ParseDocument(path, "import", project, content)
	if err != nil {
		return 0, "", err
	}
	target, err := s.LoadPath(ProjectPath(s.Home, project), "project", project, project)
	if err != nil {
		return 0, "", err
	}
	allDocs, _ := s.LoadAll()
	existing := existingIDs(allDocs)
	count := 0
	for i := len(imported.Items) - 1; i >= 0; i-- {
		item := imported.Items[i]
		if item.ID == "" {
			id, err := NewID(existing)
			if err != nil {
				return 0, "", err
			}
			item.ID = id
			item.Created = s.now()
			imported.updateMeta(item)
			count++
		}
	}
	if len(target.Lines) > 0 && strings.TrimSpace(target.Lines[len(target.Lines)-1]) != "" {
		target.Lines = append(target.Lines, "")
	}
	target.Lines = append(target.Lines, "## Imported "+s.now().Format("2006-01-02"), "")
	target.Lines = append(target.Lines, imported.Lines...)
	target.changed = true
	parseItems(target)
	return count, target.Path, s.Save(target)
}

type DoctorReport struct {
	MissingIDs   int
	DuplicateIDs []string
	Fixed        int
}

func (s Store) Doctor(fix bool) (DoctorReport, error) {
	docs, err := s.LoadAll()
	if err != nil {
		return DoctorReport{}, err
	}
	report := DoctorReport{}
	seen := make(map[string]*Item)
	existing := existingIDs(docs)
	for _, doc := range docs {
		for i := len(doc.Items) - 1; i >= 0; i-- {
			item := doc.Items[i]
			if item.ID == "" {
				report.MissingIDs++
				if fix {
					id, err := NewID(existing)
					if err != nil {
						return report, err
					}
					item.ID = id
					item.Created = s.now()
					doc.updateMeta(item)
					report.Fixed++
				}
				continue
			}
			if other := seen[item.ID]; other != nil {
				report.DuplicateIDs = append(report.DuplicateIDs, item.ID)
				if fix {
					id, err := NewID(existing)
					if err != nil {
						return report, err
					}
					item.ID = id
					item.Created = s.now()
					doc.updateMeta(item)
					report.Fixed++
				}
			} else {
				seen[item.ID] = item
			}
		}
	}
	if fix {
		if err := s.SaveAll(docs); err != nil {
			return report, err
		}
	}
	sort.Strings(report.DuplicateIDs)
	return report, nil
}

func (s Store) ProjectNames() ([]string, error) {
	if err := EnsureStore(s.Home); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(s.Home, "projects"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
		}
	}
	sort.Strings(names)
	return names, nil
}

func (s Store) TagCounts() (map[string]int, error) {
	docs, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, doc := range docs {
		for _, item := range doc.Items {
			for _, tag := range item.Tags {
				counts[tag]++
			}
		}
	}
	return counts, nil
}

type JSONItem struct {
	ID         string   `json:"id"`
	InternalID string   `json:"internal_id"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Done       bool     `json:"done"`
	Tags       []string `json:"tags"`
	Project    string   `json:"project"`
	Source     string   `json:"source"`
	Line       int      `json:"line"`
	Depth      int      `json:"depth"`
	Body       string   `json:"body,omitempty"`
}

func ItemsJSON(items []*Item) ([]byte, error) {
	out := make([]JSONItem, 0, len(items))
	for _, item := range items {
		out = append(out, JSONItem{
			ID:         item.DisplayID,
			InternalID: item.ID,
			Type:       item.Type,
			Title:      item.Title,
			Done:       item.Done,
			Tags:       item.Tags,
			Project:    item.Project,
			Source:     item.Source,
			Line:       item.Line + 1,
			Depth:      item.Depth,
			Body:       item.Body(),
		})
	}
	return json.MarshalIndent(out, "", "  ")
}

func existingIDs(docs []*Document) map[string]bool {
	existing := make(map[string]bool)
	for _, doc := range docs {
		for _, item := range doc.Items {
			if item.ID != "" {
				existing[item.ID] = true
			}
		}
	}
	return existing
}

func (s Store) applyDisplay(docs []*Document) {
	var items []*Item
	for _, doc := range docs {
		items = append(items, doc.Items...)
	}
	ApplyDisplayIDs(items)
}

func findByID(doc *Document, id string) *Item {
	for _, item := range doc.Items {
		if item.ID == id {
			return item
		}
	}
	return nil
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
