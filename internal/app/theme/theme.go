// Package theme contains the shared terminal visual language used by the
// legacy and structured Rune clients. It intentionally owns presentation
// tokens only; client state and domain behavior remain in their packages.
package theme

import "github.com/charmbracelet/lipgloss"

var (
	ElectricBlue  lipgloss.Color = "#5c54e8"
	CosmicBase    lipgloss.Color = "#101322"
	CosmicSurface lipgloss.Color = "#15182a"
	CosmicMuted   lipgloss.Color = "#6f728f"
	CosmicText    lipgloss.Color = "#c4c4ff"
	CosmicBright  lipgloss.Color = "#f4f2ff"
	CosmicViolet  lipgloss.Color = "#7770ff"
	CosmicBlue    lipgloss.Color = "#7770ff"
	CosmicCyan    lipgloss.Color = ElectricBlue
	CosmicAmber   lipgloss.Color = "#c7b9ff"
	CosmicGreen   lipgloss.Color = "#a9d8c0"
)

var (
	TopStyle        = lipgloss.NewStyle().Foreground(CosmicText).Background(CosmicBase)
	LogoStyle       = lipgloss.NewStyle().Bold(true).Foreground(CosmicViolet).Background(CosmicBase)
	TopLabelStyle   = lipgloss.NewStyle().Bold(true).Foreground(CosmicBlue).Background(CosmicBase)
	ProjectStyle    = lipgloss.NewStyle().Bold(true).Foreground(CosmicViolet).Background(CosmicBase)
	TodoStyle       = lipgloss.NewStyle().Bold(true).Foreground(CosmicAmber).Background(CosmicBase)
	DoneCountStyle  = lipgloss.NewStyle().Bold(true).Foreground(CosmicGreen).Background(CosmicBase)
	TopMetaStyle    = lipgloss.NewStyle().Foreground(CosmicMuted).Background(CosmicBase)
	SelectedStyle   = lipgloss.NewStyle().Foreground(CosmicBase).Background(CosmicViolet).Bold(true)
	DimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#686b89"))
	TagStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#a4a4dc"))
	DoneStyle       = lipgloss.NewStyle().Foreground(CosmicGreen)
	OpenStyle       = lipgloss.NewStyle().Foreground(CosmicBlue)
	HeadingStyle    = lipgloss.NewStyle().Bold(true).Foreground(ElectricBlue)
	LabelStyle      = lipgloss.NewStyle().Bold(true).Foreground(CosmicViolet)
	CodeStyle       = lipgloss.NewStyle().Foreground(CosmicBright).Background(lipgloss.Color("#202541"))
	SurfaceStyle    = lipgloss.NewStyle().Foreground(CosmicText).Background(CosmicSurface)
	FooterBarStyle  = SurfaceStyle
	FooterKeyStyle  = lipgloss.NewStyle().Bold(true).Foreground(CosmicCyan).Background(CosmicSurface)
	FooterTextStyle = lipgloss.NewStyle().Foreground(CosmicBright).Background(CosmicSurface)
	FooterSepStyle  = lipgloss.NewStyle().Foreground(CosmicMuted).Background(CosmicSurface)
	StatusStyle     = lipgloss.NewStyle().Bold(true).Foreground(CosmicBase).Background(CosmicViolet)
	SectionBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(CosmicViolet)
	TopBoxStyle     = SectionBoxStyle.Background(CosmicBase)
	FooterBoxStyle  = SectionBoxStyle.Background(CosmicSurface)
	PanelBoxStyle   = SectionBoxStyle.Background(CosmicSurface)
)

var DepthColors = []lipgloss.Color{
	lipgloss.Color("#c8c8ff"),
	lipgloss.Color("#a4a4dc"),
	lipgloss.Color("#7770ff"),
	lipgloss.Color("#aba9ff"),
	lipgloss.Color("#c7b9ff"),
	lipgloss.Color("#9b9bc9"),
	ElectricBlue,
	lipgloss.Color("#a9d8c0"),
	lipgloss.Color("#77748f"),
}
