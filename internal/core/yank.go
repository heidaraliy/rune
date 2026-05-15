package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	YankAgent       = "$rune-agent"
	YankInstruction = "implement this ticket, " + YankAgent + "\n"
)

func YankTicketText(item *Item, home string) string {
	if item == nil {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "# Rune Ticket: %s\n\n", item.Title)
	fmt.Fprintf(&out, "- ID: %s\n", item.DisplayID)
	fmt.Fprintf(&out, "- Status: %s\n", ItemStatus(item))
	if item.Heading != "" {
		fmt.Fprintf(&out, "- Heading: %s\n", item.Heading)
	}
	if len(item.Tags) > 0 {
		fmt.Fprintf(&out, "- Tags: #%s\n", strings.Join(item.Tags, " #"))
	}
	if item.Source != "" {
		fmt.Fprintf(&out, "- Source: %s:%d\n", SourceLabel(item, home), item.Line+1)
	}
	out.WriteString("\n## Context\n\n```markdown\n")
	out.WriteString(strings.Join(TicketContextLines(item), "\n"))
	out.WriteString("\n```\n\n")
	out.WriteString(YankInstruction)
	return out.String()
}

func ItemStatus(item *Item) string {
	if item == nil {
		return ""
	}
	if item.Type != ItemTask {
		return "note"
	}
	if item.Done {
		return "done"
	}
	return "open"
}

func SourceLabel(item *Item, home string) string {
	if item == nil || item.Source == "" {
		return ""
	}
	if home != "" {
		if rel, err := filepath.Rel(home, item.Source); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return item.Source
}

func TicketContextLines(item *Item) []string {
	if item == nil || item.Doc == nil || item.Line < 0 || item.Line >= len(item.Doc.Lines) {
		return nil
	}
	var lines []string
	for _, ancestor := range ticketAncestors(item) {
		if ancestor.Line >= 0 && ancestor.Line < len(ancestor.Doc.Lines) {
			lines = append(lines, ancestor.Doc.Lines[ancestor.Line])
		}
	}
	return append(lines, TicketSubtreeLines(item)...)
}

func ticketAncestors(item *Item) []*Item {
	if item == nil || item.Doc == nil || item.Depth <= 0 {
		return nil
	}
	byDepth := make(map[int]*Item)
	for _, candidate := range item.Doc.Items {
		if candidate.Line >= item.Line {
			break
		}
		if candidate.Depth < item.Depth {
			byDepth[candidate.Depth] = candidate
		}
	}
	var ancestors []*Item
	for depth := 0; depth < item.Depth; depth++ {
		if ancestor := byDepth[depth]; ancestor != nil {
			ancestors = append(ancestors, ancestor)
		}
	}
	return ancestors
}

func TicketSubtreeLines(item *Item) []string {
	if item == nil || item.Doc == nil || item.Line < 0 || item.Line >= len(item.Doc.Lines) {
		return nil
	}
	endItem := item
	for _, candidate := range item.Doc.Items {
		if candidate.Line <= item.Line {
			continue
		}
		if candidate.Depth <= item.Depth {
			break
		}
		endItem = candidate
	}
	end := endItem.BodyEnd
	for end < len(item.Doc.Lines) && strings.TrimSpace(item.Doc.Lines[end]) == "" {
		end++
	}
	return append([]string(nil), item.Doc.Lines[item.Line:end]...)
}
