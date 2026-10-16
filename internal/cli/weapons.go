package cli

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

func weaponsCommand() *cobra.Command {
	var byKind bool
	c := &cobra.Command{
		Use:   "weapons [number]",
		Short: "Show what the teams killed with",
		Long: "Show what the teams killed with: kills of enemy soldiers and vehicles by weapon, most kills first.\n\n" +
			"Team kills, suicides and deaths not caused by a soldier are not counted.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, _, b, err := read(args)
			if err != nil {
				return err
			}
			gd, err := game()
			if err != nil {
				return err
			}

			type row struct {
				name, kind string
				teams      [3]int // kills by team 1 and 2
				vehicles   int
				headshots  int
			}
			rows := map[string]*row{}
			var order []*row
			total := 0
			for _, k := range killsOf(b) {
				if k.Killer == nil || k.Killer.Player == nil {
					continue
				}
				team := k.Killer.Player.Team
				if team != 1 && team != 2 || team == k.team {
					continue
				}
				w := gd.Weapons[k.Weapon]
				key, name := k.Weapon, weaponName(gd, k.Weapon)
				if byKind {
					key, name = w.Kind, cmp.Or(w.Kind, "-")
				}
				r := rows[key]
				if r == nil {
					r = &row{name: name, kind: w.Kind}
					rows[key] = r
					order = append(order, r)
				}
				r.teams[team]++
				if k.vehicle != nil {
					r.vehicles++
				}
				if k.Headshot {
					r.headshots++
				}
				total++
			}
			sum := func(r *row) int { return r.teams[1] + r.teams[2] }
			slices.SortStableFunc(order, func(a, b *row) int { return cmp.Compare(sum(b), sum(a)) })

			cols := []ui.Column{{Title: "Weapon", Cut: true}, {Title: "Kind"}}
			if byKind {
				cols = []ui.Column{{Title: "Kind"}}
			}
			cols = append(cols,
				ui.Column{Title: teamName(gd, b, 1), Right: true}, ui.Column{Title: teamName(gd, b, 2), Right: true},
				ui.Column{Title: "Vehicles", Right: true}, ui.Column{Title: "Headshots", Right: true})
			table := make([][]string, len(order))
			for i, r := range order {
				cells := []string{r.name, r.kind}
				if byKind {
					cells = cells[:1]
				}
				table[i] = append(cells,
					ui.Team(1).Render(strconv.Itoa(r.teams[1])), ui.Team(2).Render(strconv.Itoa(r.teams[2])),
					strconv.Itoa(r.vehicles), strconv.Itoa(r.headshots))
			}

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprint(w, header(rp.Index, rp.Path))
			fmt.Fprintln(w, w.Table(cols, table, nil))
			what := "weapons"
			if byKind {
				what = "kinds"
			}
			fmt.Fprintln(w, ui.Muted.Render(fmt.Sprintf("%d kills · %d %s", total, len(order), what)))
			return nil
		},
	}
	c.Flags().BoolVarP(&byKind, "by-kind", "k", false, "count kills by kind of weapon instead of by weapon")
	return c
}
