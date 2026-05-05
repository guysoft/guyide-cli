package output

import "github.com/charmbracelet/lipgloss"

// Charmtone palette hex codes, sourced from
// ~/workspace/sniplets/CLI_STYLE.md (Crush theme).
const (
	HexGuac     = "#12C78F" // success
	HexSriracha = "#EB4268" // error
	HexZest     = "#E8FE96" // warning / dry-run
	HexMalibu   = "#00A4FF" // info
	HexCharple  = "#6B50FF" // primary / borders / headings
	HexDolly    = "#FF60FF" // alt emphasis
	HexBok      = "#68FFD6" // tertiary
	HexCumin    = "#BF976F" // file paths
	HexJulep    = "#00FFB2" // values
	HexCoral    = "#FF577D" // identifiers
	HexSquid    = "#858392" // muted / hints
	HexOyster   = "#605F6B" // subtle / repl echo
)

// theme bundles all named lipgloss styles. Its zero value is unsafe; use
// newTheme or newPlainTheme.
type theme struct {
	success  lipgloss.Style
	errStyle lipgloss.Style
	warning  lipgloss.Style
	info     lipgloss.Style
	primary  lipgloss.Style
	muted    lipgloss.Style
	subtle   lipgloss.Style
	path     lipgloss.Style
	value    lipgloss.Style
	id       lipgloss.Style
	header   lipgloss.Style
	step     lipgloss.Style
	dryRun   lipgloss.Style
	border   lipgloss.Border
	panel    lipgloss.Style
}

func newTheme() theme {
	border := lipgloss.RoundedBorder()
	return theme{
		success:  lipgloss.NewStyle().Foreground(lipgloss.Color(HexGuac)).Bold(true),
		errStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(HexSriracha)).Bold(true),
		warning:  lipgloss.NewStyle().Foreground(lipgloss.Color(HexZest)),
		info:     lipgloss.NewStyle().Foreground(lipgloss.Color(HexMalibu)),
		primary:  lipgloss.NewStyle().Foreground(lipgloss.Color(HexCharple)).Bold(true),
		muted:    lipgloss.NewStyle().Foreground(lipgloss.Color(HexSquid)),
		subtle:   lipgloss.NewStyle().Foreground(lipgloss.Color(HexOyster)),
		path:     lipgloss.NewStyle().Foreground(lipgloss.Color(HexCumin)),
		value:    lipgloss.NewStyle().Foreground(lipgloss.Color(HexJulep)),
		id:       lipgloss.NewStyle().Foreground(lipgloss.Color(HexCoral)),
		header:   lipgloss.NewStyle().Foreground(lipgloss.Color(HexCharple)).Bold(true),
		step:     lipgloss.NewStyle().Foreground(lipgloss.Color(HexMalibu)).Bold(true),
		dryRun:   lipgloss.NewStyle().Foreground(lipgloss.Color(HexZest)).Bold(true),
		border:   border,
		panel:    lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color(HexCharple)).Padding(0, 1),
	}
}

// newPlainTheme returns a theme with all colors stripped, suitable for
// NO_COLOR / CI / non-TTY outputs that still want some structure.
func newPlainTheme() theme {
	plain := lipgloss.NewStyle()
	border := lipgloss.NormalBorder()
	return theme{
		success:  plain,
		errStyle: plain,
		warning:  plain,
		info:     plain,
		primary:  plain,
		muted:    plain,
		subtle:   plain,
		path:     plain,
		value:    plain,
		id:       plain,
		header:   plain,
		step:     plain,
		dryRun:   plain,
		border:   border,
		panel:    lipgloss.NewStyle().Border(border).Padding(0, 1),
	}
}
