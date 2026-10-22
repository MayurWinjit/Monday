package theme

import "github.com/charmbracelet/lipgloss"

const (
	Rosewater = lipgloss.Color("#f5e0dc")
	Flamingo  = lipgloss.Color("#f2cdcd")
	Pink      = lipgloss.Color("#f5c2e7")
	Mauve     = lipgloss.Color("#cba6f7")
	Red       = lipgloss.Color("#f38ba8")
	Peach     = lipgloss.Color("#fab387")
	Yellow    = lipgloss.Color("#f9e2af")
	Green     = lipgloss.Color("#a6e3a1")
	Teal      = lipgloss.Color("#94e2d5")
	Sky       = lipgloss.Color("#89dceb")
	Blue      = lipgloss.Color("#89b4fa")
	Lavender  = lipgloss.Color("#b4befe")
	Text      = lipgloss.Color("#cdd6f4")
	Subtext0  = lipgloss.Color("#a6adc8")
	Overlay0  = lipgloss.Color("#6c7086")
	Surface0  = lipgloss.Color("#313244")
	Base      = lipgloss.Color("#1e1e2e")
	Mantle    = lipgloss.Color("#181825")
)

var StatusColor = map[string]lipgloss.Color{
	"waiting":     Overlay0,
	"running":     Blue,
	"needs_input": Yellow,
	"done":        Green,
	"failed":      Red,
}

const (
	MinWidth  = 60
	MinHeight = 15
)

var (
	Heading = lipgloss.NewStyle().
		Bold(true).
		Foreground(Mauve)

	Subheading = lipgloss.NewStyle().
			Foreground(Lavender)

	Body = lipgloss.NewStyle().
		Foreground(Text)

	Dim = lipgloss.NewStyle().
		Foreground(Subtext0)

	Faint = lipgloss.NewStyle().
		Foreground(Overlay0)

	ErrorText = lipgloss.NewStyle().
			Bold(true).
			Foreground(Red)

	HelpBarStyle = lipgloss.NewStyle().
			Foreground(Overlay0)

	BorderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Surface0)

	FocusedBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(Mauve)
)

func Centered(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func TooSmall(width, height int) string {
	msg := ErrorText.Render("Terminal too small") + "\n" + Dim.Render("Please resize to at least 60x15.")
	if width <= 0 || height <= 0 {
		return msg
	}
	return Centered(width, height, msg)
}
