// Package theme contains the shared terminal visual language used by the
// legacy and structured Rune clients. It intentionally owns presentation
// tokens only; client state and domain behavior remain in their packages.
package theme

import "github.com/charmbracelet/lipgloss"

var (
	ElectricBlue  lipgloss.Color = "#5b58e6"
	CosmicBase    lipgloss.Color = "#191821"
	CosmicSurface lipgloss.Color = "#211f2c"
	CosmicMuted   lipgloss.Color = "#77748f"
	CosmicText    lipgloss.Color = "#c8c8ff"
	CosmicBright  lipgloss.Color = "#f0efff"
	CosmicViolet  lipgloss.Color = "#b8b7ff"
	CosmicBlue    lipgloss.Color = "#7976ff"
	CosmicCyan    lipgloss.Color = ElectricBlue
	CosmicAmber   lipgloss.Color = "#e0c5ff"
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
	DimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#67657b"))
	TagStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#a7a6d8"))
	DoneStyle       = lipgloss.NewStyle().Foreground(CosmicGreen)
	OpenStyle       = lipgloss.NewStyle().Foreground(CosmicBlue)
	HeadingStyle    = lipgloss.NewStyle().Bold(true).Foreground(ElectricBlue)
	LabelStyle      = lipgloss.NewStyle().Bold(true).Foreground(CosmicViolet)
	CodeStyle       = lipgloss.NewStyle().Foreground(CosmicBright).Background(lipgloss.Color("#302e40"))
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
	lipgloss.Color("#a7a6d8"),
	lipgloss.Color("#7976ff"),
	lipgloss.Color("#b8b7ff"),
	lipgloss.Color("#e0c5ff"),
	lipgloss.Color("#9f9dd0"),
	ElectricBlue,
	lipgloss.Color("#a9d8c0"),
	lipgloss.Color("#77748f"),
}
