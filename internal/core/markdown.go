package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const metaPrefix = "<!-- rune:"

func ParseDocument(path, kind, key string, content []byte) (*Document, error) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	doc := &Document{Path: path, Kind: kind, Key: key, Title: key, Lines: lines}
	parseItems(doc)
	return doc, nil
}

func NewDocument(path, kind, key, title string) *Document {
	if title == "" {
		title = key
	}
	if title == "" {
		title = "Rune"
	}
	doc := &Document{
		Path:  path,
		Kind:  kind,
		Key:   key,
		Title: title,
		Lines: []string{"# " + title, ""},
	}
	parseItems(doc)
	doc.changed = true
	return doc
}

func parseItems(doc *Document) {
	doc.Items = nil
	heading := ""
	for i := 0; i < len(doc.Lines); i++ {
		line := doc.Lines[i]
		if title, ok := parseHeading(line); ok {
			heading = title
			continue
		}
		itemType, title, done, depth, ok := parseItemLine(line)
		if !ok {
			continue
		}
		metaLine := -1
		meta := map[string]string{}
		if found := findItemMetaLine(doc.Lines, i); found >= 0 {
			metaLine = found
			if metaLine != i+1 {
				metaText := doc.Lines[metaLine]
				doc.Lines = append(doc.Lines[:metaLine], doc.Lines[metaLine+1:]...)
				insertAt := i + 1
				doc.Lines = append(doc.Lines[:insertAt], append([]string{metaText}, doc.Lines[insertAt:]...)...)
				doc.changed = true
				metaLine = insertAt
			}
			meta, _ = parseMeta(doc.Lines[metaLine])
		}
		if v := meta["type"]; v == ItemTask || v == ItemNote {
			itemType = v
		}
		item := &Item{
			ID:        meta["id"],
			Type:      itemType,
			Title:     title,
			Done:      done,
			Tags:      parseTags(meta["tags"]),
			Created:   parseTime(firstNonEmpty(meta["created"], meta["created_at"])),
			Finished:  parseTime(firstNonEmpty(meta["finished_at"], meta["finished"])),
			Heading:   heading,
			Depth:     depth,
			Project:   doc.Key,
			Source:    doc.Path,
			Line:      i,
			MetaLine:  metaLine,
			BodyStart: i + 1,
			Doc:       doc,
		}
		if metaLine >= 0 {
			item.BodyStart = metaLine + 1
		}
		doc.Items = append(doc.Items, item)
	}
	for idx, item := range doc.Items {
		end := len(doc.Lines)
		for j := idx + 1; j < len(doc.Items); j++ {
			if doc.Items[j].Line > item.Line {
				end = doc.Items[j].Line
				break
			}
		}
		for end > item.BodyStart && strings.TrimSpace(doc.Lines[end-1]) == "" {
			end--
		}
		item.BodyEnd = end
	}
}

func findItemMetaLine(lines []string, itemLine int) int {
	for i := itemLine + 1; i < len(lines); i++ {
		if _, _, _, _, ok := parseItemLine(lines[i]); ok {
			return -1
		}
		if _, ok := parseMeta(lines[i]); ok {
			return i
		}
	}
	return -1
}

func parseHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	level := 0
	for level < len(trimmed) && level < 6 && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level >= len(trimmed) || trimmed[level] != ' ' {
		return "", false
	}
	return strings.TrimSpace(trimmed[level+1:]), true
}

func parseItemLine(line string) (itemType, title string, done bool, depth int, ok bool) {
	depth = listDepth(line)
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "- [ ] ") || strings.HasPrefix(trimmed, "* [ ] ") {
		return ItemTask, strings.TrimSpace(trimmed[6:]), false, depth, true
	}
	if strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") ||
		strings.HasPrefix(trimmed, "* [x] ") || strings.HasPrefix(trimmed, "* [X] ") {
		return ItemTask, strings.TrimSpace(trimmed[6:]), true, depth, true
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return ItemNote, strings.TrimSpace(trimmed[2:]), false, depth, true
	}
	return "", "", false, 0, false
}

func listDepth(line string) int {
	cols := 0
	for _, r := range line {
		switch r {
		case ' ':
			cols++
		case '\t':
			cols += 4
		default:
			if cols == 0 {
				return 0
			}
			return (cols + 3) / 4
		}
	}
	return 0
}

func parseMeta(line string) (map[string]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, metaPrefix) || !strings.HasSuffix(trimmed, "-->") {
		return nil, false
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, metaPrefix), "-->"))
	fields := strings.Fields(body)
	meta := make(map[string]string)
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		meta[key] = strings.Trim(value, `"`)
	}
	return meta, true
}

func renderMeta(item *Item) string {
	created := item.Created
	if created.IsZero() {
		created = time.Now().UTC()
	}
	tags := strings.Join(normalizeTags(item.Tags), ",")
	fields := []string{
		fmt.Sprintf("id=%s", item.ID),
		fmt.Sprintf("type=%s", item.Type),
		fmt.Sprintf("tags=%s", tags),
		fmt.Sprintf("created=%s", created.UTC().Format(time.RFC3339)),
	}
	if !item.Finished.IsZero() {
		fields = append(fields, fmt.Sprintf("finished_at=%s", item.Finished.UTC().Format(time.RFC3339)))
	}
	return "<!-- rune:" + strings.Join(fields, " ") + " -->"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func parseTags(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	return normalizeTags(parts)
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, tag := range tags {
		tag = cleanKey(strings.TrimPrefix(strings.TrimSpace(tag), "#"))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, value)
	return t
}

func (d *Document) appendItem(item *Item, body string) {
	insertAt := d.appendItemIndex()
	block := itemBlock(item, body)
	if insertAt == len(d.Lines) {
		if len(d.Lines) > 0 && strings.TrimSpace(d.Lines[len(d.Lines)-1]) != "" {
			d.Lines = append(d.Lines, "")
		}
		d.Lines = append(d.Lines, block...)
	} else {
		if insertAt > 0 && strings.TrimSpace(d.Lines[insertAt-1]) != "" {
			block = append([]string{""}, block...)
		}
		if len(block) > 0 && strings.TrimSpace(block[len(block)-1]) != "" && strings.TrimSpace(d.Lines[insertAt]) != "" {
			block = append(block, "")
		}
		d.Lines = append(append([]string{}, d.Lines[:insertAt]...), append(block, d.Lines[insertAt:]...)...)
	}
	d.changed = true
	parseItems(d)
}

func (d *Document) appendItemIndex() int {
	for idx, line := range d.Lines {
		heading, ok := parseHeading(line)
		if !ok {
			continue
		}
		if isDoneSectionHeading(heading) {
			return idx
		}
	}
	return len(d.Lines)
}

func isDoneSectionHeading(heading string) bool {
	normalized := strings.ToLower(strings.TrimSpace(heading))
	return normalized == "done" ||
		strings.HasPrefix(normalized, "done ") ||
		normalized == "restored done" ||
		strings.HasPrefix(normalized, "restored done ")
}

func (d *Document) insertItemNear(anchor *Item, item *Item, body string, above bool) {
	if anchor == nil {
		d.appendItem(item, body)
		return
	}
	item.Depth = anchor.Depth
	insertAt := anchor.Line
	if !above {
		insertAt = d.subtreeEnd(anchor)
	}
	block := itemBlock(item, body)
	d.Lines = append(append([]string{}, d.Lines[:insertAt]...), append(block, d.Lines[insertAt:]...)...)
	d.changed = true
	parseItems(d)
}

func itemBlock(item *Item, body string) []string {
	indent := strings.Repeat("    ", maxInt(0, item.Depth))
	prefix := "- "
	if item.Type == ItemTask {
		prefix = "- [ ] "
		if item.Done {
			prefix = "- [x] "
		}
	}
	lines := []string{indent + prefix + item.Title, renderMeta(item)}
	if strings.TrimRight(body, "\n") != "" {
		lines = append(lines, "")
		bodyIndent := indent + "  "
		for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
			if line == "" {
				lines = append(lines, "")
			} else {
				lines = append(lines, bodyIndent+line)
			}
		}
	}
	return lines
}

func (d *Document) subtreeEnd(anchor *Item) int {
	endItem := anchor
	for _, item := range d.Items {
		if item.Line <= anchor.Line {
			continue
		}
		if item.Depth <= anchor.Depth {
			break
		}
		endItem = item
	}
	return d.blockEnd(endItem)
}

func (d *Document) blockEnd(item *Item) int {
	end := item.BodyEnd
	for end < len(d.Lines) && strings.TrimSpace(d.Lines[end]) == "" {
		end++
	}
	return end
}

func (d *Document) updateMeta(item *Item) {
	indent := ""
	if item.Line >= 0 && item.Line < len(d.Lines) {
		line := d.Lines[item.Line]
		indent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	}
	if item.MetaLine >= 0 && item.MetaLine < len(d.Lines) {
		d.Lines[item.MetaLine] = indent + renderMeta(item)
	} else {
		insertAt := item.Line + 1
		d.Lines = append(d.Lines[:insertAt], append([]string{indent + renderMeta(item)}, d.Lines[insertAt:]...)...)
	}
	d.changed = true
	parseItems(d)
}

func (d *Document) updateItemLine(item *Item) {
	if item.Line < 0 || item.Line >= len(d.Lines) {
		return
	}
	line := d.Lines[item.Line]
	indentLen := len(line) - len(strings.TrimLeft(line, " \t"))
	indent := line[:indentLen]
	if item.Type == ItemTask {
		marker := "[ ]"
		if item.Done {
			marker = "[x]"
		}
		d.Lines[item.Line] = indent + "- " + marker + " " + item.Title
	} else {
		d.Lines[item.Line] = indent + "- " + item.Title
	}
	d.changed = true
	parseItems(d)
}

func (d *Document) setTaskDoneAt(item *Item, done bool, finishedAt time.Time) *Item {
	if item == nil || item.Type != ItemTask {
		return item
	}
	id := item.ID
	item.Done = done
	if done {
		if item.Finished.IsZero() {
			item.Finished = finishedAt.UTC()
		}
	} else {
		item.Finished = time.Time{}
	}
	d.updateItemLine(item)
	item = findByID(d, id)
	if item == nil {
		return nil
	}
	if done {
		if item.Finished.IsZero() {
			item.Finished = finishedAt.UTC()
		}
	} else {
		item.Finished = time.Time{}
	}
	d.updateMeta(item)
	return findByID(d, id)
}

func (d *Document) taskDescendants(parent *Item) []*Item {
	if parent == nil {
		return nil
	}
	var descendants []*Item
	for _, item := range d.Items {
		if item.Line <= parent.Line {
			continue
		}
		if item.Depth <= parent.Depth {
			break
		}
		if item.Type == ItemTask {
			descendants = append(descendants, item)
		}
	}
	return descendants
}

func (d *Document) subtreeItems(parent *Item) []*Item {
	if parent == nil {
		return nil
	}
	var items []*Item
	for _, item := range d.Items {
		if item.Line < parent.Line {
			continue
		}
		if item.Line > parent.Line && item.Depth <= parent.Depth {
			break
		}
		items = append(items, item)
	}
	return items
}

func (d *Document) replaceBody(item *Item, body string) {
	lines := bodyLines(body)
	replacement := append([]string(nil), lines...)
	d.Lines = append(append([]string{}, d.Lines[:item.BodyStart]...), append(replacement, d.Lines[item.BodyEnd:]...)...)
	d.changed = true
	parseItems(d)
}

func (d *Document) appendBody(item *Item, body string) {
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return
	}
	insert := bodyLines(body)
	if item.BodyEnd > item.BodyStart && strings.TrimSpace(d.Lines[item.BodyEnd-1]) != "" {
		insert = append([]string{""}, insert...)
	} else if item.BodyEnd == item.BodyStart {
		insert = append([]string{""}, insert...)
	}
	d.Lines = append(append([]string{}, d.Lines[:item.BodyEnd]...), append(insert, d.Lines[item.BodyEnd:]...)...)
	d.changed = true
	parseItems(d)
}

func bodyLines(body string) []string {
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if line == "" {
			out = append(out, "")
		} else {
			out = append(out, "  "+line)
		}
	}
	return out
}

func (d *Document) rawBlock(item *Item) []string {
	return append([]string(nil), d.Lines[item.Line:d.blockEnd(item)]...)
}

func (d *Document) RawBlock(item *Item) []string {
	return d.rawBlock(item)
}

func (d *Document) rawSubtree(item *Item) []string {
	return append([]string(nil), d.Lines[item.Line:d.subtreeEnd(item)]...)
}

func (d *Document) removeBlocks(items []*Item) [][]string {
	sort.Slice(items, func(i, j int) bool { return items[i].Line > items[j].Line })
	var blocks [][]string
	for _, item := range items {
		block := d.rawBlock(item)
		blocks = append(blocks, block)
		end := item.BodyEnd
		for end < len(d.Lines) && strings.TrimSpace(d.Lines[end]) == "" {
			end++
		}
		d.Lines = append(d.Lines[:item.Line], d.Lines[end:]...)
	}
	d.changed = true
	parseItems(d)
	return blocks
}

func (d *Document) removeSubtrees(items []*Item) [][]string {
	sort.Slice(items, func(i, j int) bool { return items[i].Line > items[j].Line })
	var blocks [][]string
	for _, item := range items {
		block := d.rawSubtree(item)
		blocks = append(blocks, block)
		end := d.subtreeEnd(item)
		d.Lines = append(d.Lines[:item.Line], d.Lines[end:]...)
	}
	d.changed = true
	parseItems(d)
	return blocks
}

func decodeEscapes(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			out.WriteByte(value[i])
			continue
		}
		i++
		switch value[i] {
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case '\\':
			out.WriteByte('\\')
		default:
			out.WriteByte('\\')
			out.WriteByte(value[i])
		}
	}
	return out.String()
}

func DecodeEscapes(value string) string {
	return decodeEscapes(value)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
