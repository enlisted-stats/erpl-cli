package ui

import (
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
)

var (
	accent = lipgloss.Color("#E8B04B")
	muted  = lipgloss.Color("#7C7C7C")
	border = lipgloss.Color("#4A4A4A")
	green  = lipgloss.Color("#7BC67B")
	red    = lipgloss.Color("#E5675F")
	teams  = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C9C9C9")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#5FA8FF")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7A6B")),
	}
)

var (
	Title  = lipgloss.NewStyle().Bold(true).Foreground(accent)
	Accent = lipgloss.NewStyle().Foreground(accent)
	Muted  = lipgloss.NewStyle().Foreground(muted)
	Good   = lipgloss.NewStyle().Foreground(green)
	Bad    = lipgloss.NewStyle().Foreground(red)
)

// Team is the style of team 1 or 2, or a neutral one for other numbers.
func Team(n int) lipgloss.Style {
	if n < 0 || n >= len(teams) {
		n = 0
	}
	return teams[n]
}

// Out is where a command prints.
type Out struct {
	io.Writer
	Width int // of the terminal; zero if the output is not a terminal
}

// NewOut wraps w so that colors are reduced to what the terminal supports, and
// dropped when w is a file or a pipe.
func NewOut(w io.Writer) Out {
	o := Out{Writer: colorprofile.NewWriter(w, os.Environ())}
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		if width, _, err := term.GetSize(f.Fd()); err == nil {
			o.Width = width
		}
	}
	return o
}
