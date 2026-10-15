package cli

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/enlisted-stats/erpl/battle"
	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

func zonesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "zones [number]",
		Short: "Show the zones and who held them when",
		Long: "Show the zones and who held them when.\n\n" +
			"The replay only has changes, so who held a zone and whether it was active before the first change is not known.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, _, b, err := read(args)
			if err != nil {
				return err
			}
			zs := slices.Clone(b.Zones)
			slices.SortStableFunc(zs, func(a, b *battle.Zone) int {
				return cmp.Or(cmp.Compare(a.Created, b.Created), cmp.Compare(a.Title, b.Title))
			})

			rows := make([][]string, len(zs))
			for i, z := range zs {
				rows[i] = []string{
					cmp.Or(z.Title, "-"), strings.TrimPrefix(z.Caption, "capzone/"), z.Group,
					shape(z), active(z), owners(z.Owner),
				}
			}

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprint(w, header(rp.Index, rp.Path))
			fmt.Fprintln(w, w.Table([]ui.Column{
				{Title: "Zone"}, {Title: "Name", Cut: true}, {Title: "Group", Cut: true}, {Title: "Shape"},
				{Title: "Active", Cut: true}, {Title: "Owners", Cut: true},
			}, rows, nil))
			return nil
		},
	}
}

func shape(z *battle.Zone) string {
	if z.Radius > 0 {
		return fmt.Sprintf("circle %.0f m", z.Radius)
	}
	return fmt.Sprintf("%d corners", len(z.Area))
}

// active lists the spans when the zone was active. The value at the creation of the
// zone is its initial state; a span that started before the first change starts with "?".
func active(z *battle.Zone) string {
	if len(z.Active) == 0 {
		return "-"
	}
	var spans []string
	start, open := "?", false
	for i, c := range z.Active {
		switch {
		case c.Value && !open:
			start, open = ui.Clock(c.Time), true
		case !c.Value && (open || i == 0 && c.Time != z.Created):
			spans = append(spans, start+"–"+ui.Clock(c.Time))
			open = false
		}
	}
	if open {
		spans = append(spans, start+"–")
	}
	if len(spans) == 0 {
		return ui.Muted.Render("never")
	}
	return strings.Join(spans, ", ")
}

// owners lists the owner changes as "time team", the team in its color.
func owners(cs []battle.Change[int]) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		owner := ui.Muted.Render("none")
		if c.Value > 0 {
			owner = ui.Team(c.Value).Render(fmt.Sprintf("team %d", c.Value))
		}
		parts[i] = ui.Clock(c.Time) + " " + owner
	}
	return strings.Join(parts, " → ")
}
