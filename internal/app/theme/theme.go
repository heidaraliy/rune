// Package theme contains the shared terminal visual language used by the
// legacy and structured Rune clients. It intentionally owns presentation
// tokens only; client state and domain behavior remain in their packages.
package theme

import "github.com/charmbracelet/lipgloss"

var (
	ElectricBlue  lipgloss.Color = "39"
	CosmicBase    lipgloss.Color = "#090812"
	CosmicSurface lipgloss.Color = "#151122"
	CosmicMuted   lipgloss.Color = "#52436f"
	CosmicText    lipgloss.Color = "#e8e3f4"
	CosmicBright  lipgloss.Color = "#f5f0ff"
	CosmicViolet  lipgloss.Color = "#c8a7ff"
	CosmicBlue    lipgloss.Color = "#9bbcff"
	CosmicCyan    lipgloss.Color = ElectricBlue
	CosmicAmber   lipgloss.Color = "#f4d889"
	CosmicGreen   lipgloss.Color = "#8fe6a7"
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
	DimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	TagStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	DoneStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	OpenStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	HeadingStyle    = lipgloss.NewStyle().Bold(true).Foreground(ElectricBlue)
	LabelStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	CodeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("221")).Background(lipgloss.Color("236"))
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
	lipgloss.Color("252"),
	lipgloss.Color("111"),
	lipgloss.Color("151"),
	lipgloss.Color("222"),
	lipgloss.Color("218"),
	lipgloss.Color("183"),
	ElectricBlue,
	lipgloss.Color("214"),
	lipgloss.Color("245"),
}
