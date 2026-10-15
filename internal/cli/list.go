package cli

import (
	"fmt"
	"path/filepath"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/enlisted-stats/erpl"
	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

const pageSize = 20

func listCommand() *cobra.Command {
	page := 1
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the replays, newest first",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rs, err := scan()
			if err != nil {
				return err
			}
			pages := (len(rs) + pageSize - 1) / pageSize
			if page < 1 || page > pages {
				return fmt.Errorf("no page %d: pages go from 1 to %d", page, pages)
			}
			last := len(rs) - (page-1)*pageSize
			first := max(last-pageSize, 0)

			var rows [][]string
			failed := map[int]bool{}
			for i := last - 1; i >= first; i-- {
				rp := rs[i]
				num := strconv.Itoa(rp.Index)
				r, err := erpl.Open(rp.Path)
				if err != nil {
					failed[len(rows)] = true
					rows = append(rows, []string{num, "-", filepath.Base(rp.Path), err.Error(), "", "", ""})
					continue
				}
				m := r.Meta()
				rows = append(rows, []string{
					num, ui.Date(m.StartTime), m.Mission, m.MissionType, m.Campaign,
					ui.Clock(m.Duration), strconv.Itoa(len(m.Players)),
				})
			}

			cols := []ui.Column{
				{Title: "#", Right: true}, {Title: "Date"}, {Title: "Mission", Cut: true}, {Title: "Mode"},
				{Title: "Campaign", Cut: true}, {Title: "Length", Right: true}, {Title: "Players", Right: true},
			}
			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprintln(w, w.Table(cols, rows, func(i int) lipgloss.Style {
				if failed[i] {
					return ui.Bad
				}
				return lipgloss.NewStyle()
			}))
			footer := fmt.Sprintf("page %d of %d · %d replays", page, pages, len(rs))
			if page < pages {
				footer += fmt.Sprintf(" · older: erpl list -p %d", page+1)
			}
			fmt.Fprintln(w, ui.Muted.Render(footer))
			return nil
		},
	}
	c.Flags().IntVarP(&page, "page", "p", 1, "page number, from 1 for the newest replays")
	return c
}
