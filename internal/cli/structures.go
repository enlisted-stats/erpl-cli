package cli

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/enlisted-stats/erpl/battle"
	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

func structuresCommand() *cobra.Command {
	var kind string
	var all bool
	c := &cobra.Command{
		Use:   "structures [number]",
		Short: "Show what engineers built: rally points, guns, fortifications",
		Long: "Show what engineers built: rally points, guns, fortifications.\n\n" +
			"By default structures are counted by kind; --all lists every structure.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, b, err := read(args)
			if err != nil {
				return err
			}
			gd, err := game()
			if err != nil {
				return err
			}
			name := func(s *battle.Structure) string { return named(gd.Structures[s.Kind].Name, s.Kind) }
			var ss []*battle.Structure
			for _, s := range b.Structures {
				if strings.Contains(s.Kind, kind) || strings.Contains(strings.ToLower(name(s)), strings.ToLower(kind)) {
					ss = append(ss, s)
				}
			}
			// stood is how long s stood, until the end of the recording if it was not removed.
			stood := func(s *battle.Structure) time.Duration {
				if s.Removed == 0 {
					return m.LastRecordTime - s.Built
				}
				return s.Removed - s.Built
			}

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprint(w, header(rp.Index, rp.Path))
			if all {
				fmt.Fprintln(w, structuresTable(w, ss, name, stood))
			} else {
				fmt.Fprintln(w, kindsTable(w, ss, name, func(n int) string { return teamName(gd, b, n) }, stood))
			}
			fmt.Fprintln(w, ui.Muted.Render(fmt.Sprintf("%d structures", len(ss))))
			return nil
		},
	}
	c.Flags().StringVarP(&kind, "kind", "k", "", "only structures whose name or kind contains this text, e.g. rally")
	c.Flags().BoolVarP(&all, "all", "a", false, "list every structure instead of counting them by kind")
	return c
}

func structuresTable(w ui.Out, ss []*battle.Structure, name func(*battle.Structure) string, stood func(*battle.Structure) time.Duration) string {
	rows := make([][]string, len(ss))
	for i, s := range ss {
		removed := ui.Good.Render("stood")
		if s.Removed != 0 {
			removed = ui.Clock(s.Removed)
		}
		rows[i] = []string{
			ui.Clock(s.Built), name(s), ui.Team(s.Team).Render(playerName(s.Player)),
			fmt.Sprintf("%.0f, %.0f", s.Pos[0], s.Pos[2]), removed, ui.Clock(stood(s)),
		}
	}
	return w.Table([]ui.Column{
		{Title: "Built", Right: true}, {Title: "Structure", Cut: true}, {Title: "Player", Cut: true},
		{Title: "x, z", Right: true}, {Title: "Removed", Right: true}, {Title: "Stood", Right: true},
	}, rows, nil)
}

// kindsTable counts the structures of each name by team, most built first: kinds of
// different armies share a name, e.g. "Rally point". team names the teams.
func kindsTable(w ui.Out, ss []*battle.Structure, name func(*battle.Structure) string, team func(int) string, stood func(*battle.Structure) time.Duration) string {
	type kind struct {
		name    string
		teams   [3]int
		removed int
		stood   time.Duration
	}
	byName := map[string]*kind{}
	var kinds []*kind
	for _, s := range ss {
		n := name(s)
		k := byName[n]
		if k == nil {
			k = &kind{name: n}
			byName[n] = k
			kinds = append(kinds, k)
		}
		if s.Team == 1 || s.Team == 2 {
			k.teams[s.Team]++
		} else {
			k.teams[0]++
		}
		if s.Removed != 0 {
			k.removed++
		}
		k.stood += stood(s)
	}
	total := func(k *kind) int { return k.teams[0] + k.teams[1] + k.teams[2] }
	slices.SortStableFunc(kinds, func(a, b *kind) int { return cmp.Compare(total(b), total(a)) })

	rows := make([][]string, len(kinds))
	for i, k := range kinds {
		rows[i] = []string{
			k.name, ui.Team(1).Render(strconv.Itoa(k.teams[1])), ui.Team(2).Render(strconv.Itoa(k.teams[2])),
			strconv.Itoa(k.removed), ui.Clock(k.stood / time.Duration(total(k))),
		}
	}
	return w.Table([]ui.Column{
		{Title: "Structure", Cut: true}, {Title: team(1), Right: true}, {Title: team(2), Right: true},
		{Title: "Removed", Right: true}, {Title: "Stood on average", Right: true},
	}, rows, nil)
}
