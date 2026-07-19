package v2

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	termansi "github.com/charmbracelet/x/ansi"
	"github.com/heidaraliy/rune/internal/app/theme"
)

var (
	v2TitleStyle       = lipgloss.NewStyle().Bold(true).Foreground(theme.CosmicBright)
	v2BodyStyle        = lipgloss.NewStyle().Foreground(theme.CosmicText)
	v2MetaStyle        = lipgloss.NewStyle().Foreground(theme.CosmicMuted)
	v2PanelStyle       = theme.PanelBoxStyle
	v2SelectedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#211b2d"))
	v2MarkerStyle      = lipgloss.NewStyle().Bold(true).Foreground(theme.CosmicViolet)
	v2DangerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
)

func renderStyledBox(width, height int, lines []string, boxStyle, fillStyle lipgloss.Style) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	if width < 4 || height < 3 {
		return fitLines(lines, width, height)
	}
	innerWidth := max(1, width-2)
	innerHeight := max(1, height-2)
	rows := make([]string, 0, innerHeight)
	for index := 0; index < innerHeight; index++ {
		line := ""
		if index < len(lines) {
			line = lines[index]
		}
		rows = append(rows, renderFilledLine(line, innerWidth, fillStyle))
	}
	boxed := boxStyle.Width(innerWidth).Height(innerHeight).Render(strings.Join(rows, "\n"))
	return strings.Split(boxed, "\n")
}

func renderStyledBoxString(width, height int, lines []string, boxStyle, fillStyle lipgloss.Style) string {
	return strings.Join(renderStyledBox(width, height, lines, boxStyle, fillStyle), "\n")
}

func renderFilledLine(line string, width int, fillStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	line = clipStyled(line, width)
	if missing := width - lipgloss.Width(line); missing > 0 {
		line += fillStyle.Render(strings.Repeat(" ", missing))
	}
	return line
}

func renderTwoColumnLine(width int, left, right string) string {
	if width <= 0 {
		return ""
	}
	right = clipStyled(right, width)
	rightWidth := lipgloss.Width(right)
	leftWidth := max(1, width-rightWidth-1)
	left = clipStyled(left, leftWidth)
	remaining := width - lipgloss.Width(left) - rightWidth
	if remaining < 1 {
		return clipStyled(left+" "+right, width)
	}
	return left + strings.Repeat(" ", remaining) + right
}

func renderRule(width int) string {
	if width <= 0 {
		return ""
	}
	return theme.DimStyle.Render(strings.Repeat("─", width))
}

func renderBadge(value string, style lipgloss.Style) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "_", " "))
	if value == "" {
		value = "—"
	}
	return style.Render(" " + value + " ")
}

func lifecycleBadge(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	style := lipgloss.NewStyle().Bold(true)
	switch value {
	case "complete", "completed":
		style = style.Foreground(theme.CosmicBase).Background(theme.CosmicGreen)
	case "ready", "review":
		style = style.Foreground(theme.CosmicBase).Background(theme.CosmicAmber)
	case "in_progress", "running", "queued":
		style = style.Foreground(theme.CosmicBase).Background(theme.ElectricBlue)
	case "blocked", "failed", "canceled", "deleted":
		style = style.Foreground(theme.CosmicBright).Background(lipgloss.Color("95"))
	default:
		style = style.Foreground(theme.CosmicText).Background(lipgloss.Color("238"))
	}
	return renderBadge(value, style)
}

func kindBadge(kind string) string {
	style := theme.ProjectStyle
	if strings.EqualFold(kind, "task") {
		style = theme.TodoStyle
	}
	return style.Render(strings.ToUpper(kind))
}

func compactPreview(value string, fallback string, width int) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		value = fallback
	}
	return truncate(value, width)
}

func shortDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("Jan 02")
}

func wrapPlainText(value string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	result := make([]string, 0)
	for _, rawLine := range strings.Split(value, "\n") {
		if strings.TrimSpace(rawLine) == "" {
			result = append(result, "")
			continue
		}
		words := strings.Fields(rawLine)
		line := ""
		for _, word := range words {
			if lipgloss.Width(word) > width {
				if line != "" {
					result = append(result, line)
					line = ""
				}
				for lipgloss.Width(word) > width {
					part, rest := splitTextAtWidth(word, width)
					result = append(result, part)
					word = rest
				}
				if word != "" {
					line = word
				}
				continue
			}
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if lipgloss.Width(candidate) > width {
				result = append(result, line)
				line = word
			} else {
				line = candidate
			}
		}
		if line != "" {
			result = append(result, line)
		}
	}
	if len(result) == 0 {
		return []string{""}
	}
	return result
}

func splitTextAtWidth(value string, width int) (string, string) {
	if width <= 0 {
		return "", value
	}
	runes := []rune(value)
	cut := 0
	for cut < len(runes) && lipgloss.Width(string(runes[:cut+1])) <= width {
		cut++
	}
	if cut == 0 {
		cut = 1
	}
	return string(runes[:cut]), string(runes[cut:])
}

func clipStyled(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return termansi.Truncate(value, width, "")
}
