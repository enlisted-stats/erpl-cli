package ui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"
)

// Column is a table column.
type Column struct {
	Title string
	Right bool // for numbers
	Cut   bool // its cells may be cut short to fit the table into the terminal
}

// minCut is the width a cut column keeps at least.
const minCut = 6

// Table renders rows under cols. row, if not nil, gives the style of a whole row.
// A table wider than the terminal is narrowed by cutting the widest Cut column
// first; if that is not enough, the table stays wider than the terminal.
func (o Out) Table(cols []Column, rows [][]string, row func(i int) lipgloss.Style) string {
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = c.Title
	}
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(border)).
		Headers(headers...).
		StyleFunc(func(r, c int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			switch {
			case r == table.HeaderRow:
				s = s.Inherit(Title)
			case row != nil:
				s = s.Inherit(row(r))
			}
			if cols[c].Right {
				s = s.Align(lipgloss.Right)
			}
			return s
		})
	s := t.Rows(rows...).String()
	if over := lipgloss.Width(s) - o.Width; o.Width > 0 && over > 0 {
		s = t.ClearRows().Rows(cut(cols, rows, over)...).String()
	}
	return s
}

// cut shortens the cells of Cut columns so that the widest of them lose over cells in total.
func cut(cols []Column, rows [][]string, over int) [][]string {
	widths := make([]int, len(cols))
	for i, c := range cols {
		if !c.Cut {
			continue
		}
		widths[i] = lipgloss.Width(c.Title)
		for _, r := range rows {
			widths[i] = max(widths[i], lipgloss.Width(r[i]))
		}
	}
	for ; over > 0; over-- {
		i := slices.Index(widths, slices.Max(widths))
		if widths[i] <= max(minCut, lipgloss.Width(cols[i].Title)) {
			break
		}
		widths[i]--
	}
	cut := make([][]string, len(rows))
	for j, r := range rows {
		cut[j] = slices.Clone(r)
		for i, w := range widths {
			if cols[i].Cut {
				cut[j][i] = ansi.Truncate(r[i], w, "…")
			}
		}
	}
	return cut
}

// Fields renders name-value pairs one per line, with the names aligned.
func Fields(pairs ...[2]string) string {
	width := 0
	for _, p := range pairs {
		width = max(width, lipgloss.Width(p[0]))
	}
	name := Muted.Width(width + 2)
	lines := make([]string, len(pairs))
	for i, p := range pairs {
		lines[i] = name.Render(p[0]) + p[1]
	}
	return strings.Join(lines, "\n")
}
