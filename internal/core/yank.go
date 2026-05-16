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

type YankOptions struct {
	Agent       string
	Instruction string
}

func YankTicketText(item *Item, home string) string {
	return YankTicketTextWithOptions(item, home, YankOptionsForItem(item))
}

func YankTicketTextWithOptions(item *Item, home string, options YankOptions) string {
	if item == nil {
		return ""
	}
	options = normalizeYankOptions(options, itemYankProject(item))
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
	out.WriteString(options.Instruction)
	return out.String()
}

func YankOptionsForItem(item *Item) YankOptions {
	project := itemYankProject(item)
	options := YankOptions{Agent: DefaultYankAgent(project)}
	if item == nil || item.Doc == nil {
		return normalizeYankOptions(options, project)
	}
	if item.Doc.Kind != "project" {
		return normalizeYankOptions(YankOptions{}, "")
	}
	for _, line := range item.Doc.Lines {
		if _, _, _, _, ok := parseItemLine(line); ok {
			break
		}
		if value, ok := parseYankComment(line, "rune-ticket-agent"); ok {
			options.Agent = value
		}
		if value, ok := parseYankComment(line, "rune-ticket-instruction"); ok {
			options.Instruction = DecodeEscapes(value)
		}
	}
	return normalizeYankOptions(options, project)
}

func DefaultYankAgent(project string) string {
	project = cleanKey(project)
	if project == "" {
		return YankAgent
	}
	return "$" + project + "-agent"
}

func itemYankProject(item *Item) string {
	if item == nil {
		return ""
	}
	if item.Doc != nil && item.Doc.Kind == "project" && item.Doc.Key != "" {
		return item.Doc.Key
	}
	return item.Project
}

func normalizeYankOptions(options YankOptions, project string) YankOptions {
	options.Agent = strings.TrimSpace(options.Agent)
	if options.Agent == "" {
		options.Agent = DefaultYankAgent(project)
	}
	options.Instruction = strings.TrimSpace(options.Instruction)
	if options.Instruction == "" {
		options.Instruction = "implement this ticket, " + options.Agent
	} else if agent := yankAgentFromInstruction(options.Instruction); agent != "" && options.Agent == DefaultYankAgent(project) {
		options.Agent = agent
	}
	options.Instruction = strings.TrimRight(options.Instruction, "\n") + "\n"
	return options
}

func parseYankComment(line, key string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	prefix := "<!-- " + key + ":"
	if !strings.HasPrefix(trimmed, prefix) || !strings.HasSuffix(trimmed, "-->") {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, prefix), "-->"))
	return value, value != ""
}

func yankAgentFromInstruction(instruction string) string {
	for _, field := range strings.Fields(instruction) {
		field = strings.Trim(field, ".,;:!?()[]{}\"'")
		if strings.HasPrefix(field, "$") && len(field) > 1 {
			return field
		}
	}
	return ""
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
