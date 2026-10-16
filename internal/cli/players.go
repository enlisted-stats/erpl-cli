package cli

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/enlisted-stats/erpl/battle"
	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/gamedata"
	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

// playerStats are the columns of the players table after the headshots, as titles
// and Player.Stats keys.
var playerStats = [][2]string{
	{"Deaths", "soldierDeaths"},
	{"Tanks", "tankKills"},
	{"Planes", "planeKills"},
	{"Assists", "assists"},
	{"Captures", "captures"},
	{"Built", "builtStructures"},
	{"Score", "score"},
}

func playersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "players [number]",
		Short: "Show the players of both teams and their results",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, _, b, err := read(args)
			if err != nil {
				return err
			}
			gd, err := game()
			if err != nil {
				return err
			}
			headshots := map[*battle.Player]int{}
			for _, s := range b.Soldiers {
				if d := s.Death; d != nil && d.Headshot && d.Killer != nil && d.Killer != s {
					headshots[d.Killer.Player]++
				}
			}

			teams := map[int][]*battle.Player{}
			for _, p := range b.Players {
				teams[p.Team] = append(teams[p.Team], p)
			}
			numbers := make([]int, 0, len(teams))
			for n := range teams {
				numbers = append(numbers, n)
			}
			slices.Sort(numbers)

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprint(w, header(rp.Index, rp.Path))
			for _, n := range numbers {
				fmt.Fprintln(w)
				fmt.Fprintln(w, teamTitle(gd, b, n, teams[n]))
				fmt.Fprintln(w, playersTable(w, teams[n], headshots))
			}
			return nil
		},
	}
}

func teamTitle(gd *gamedata.Data, b *battle.Battle, n int, ps []*battle.Player) string {
	title := "No team"
	if n != 0 {
		title = fmt.Sprintf("Team %d", n)
	}
	title = ui.Team(n).Bold(true).Render(title)
	if n != 0 && teamArmy(b, n) != "" {
		title += ui.Muted.Render(" · " + teamName(gd, b, n))
	}
	title += ui.Muted.Render(fmt.Sprintf(" · %d players", len(ps)))
	switch {
	case n == 0 || b.Winner == 0:
	case n == b.Winner:
		title += ui.Good.Render(" · won")
	default:
		title += ui.Bad.Render(" · lost")
	}
	return title
}

// playersTable lists the players of one team, best score first. headshots counts the
// headshot kills of each player.
func playersTable(w ui.Out, ps []*battle.Player, headshots map[*battle.Player]int) string {
	ps = slices.Clone(ps)
	slices.SortStableFunc(ps, func(a, b *battle.Player) int {
		return cmp.Compare(b.Stats["score"], a.Stats["score"])
	})
	platoons := platoonLetters(ps)

	cols := []ui.Column{{Title: "Player", Cut: true}, {Title: "Platoon"}, {Title: "Kills", Right: true}, {Title: "Headshots", Right: true}}
	for _, s := range playerStats {
		cols = append(cols, ui.Column{Title: s[0], Right: true})
	}
	cols = append(cols, ui.Column{Title: "Notes", Cut: true})

	rows := make([][]string, len(ps))
	for i, p := range ps {
		row := []string{p.Name, platoons[p.Group], number(p.Stats["kills"]), strconv.Itoa(headshots[p])}
		for _, s := range playerStats {
			row = append(row, number(p.Stats[s[1]]))
		}
		var notes []string
		if p.Bot {
			notes = append(notes, "bot")
		}
		if p.Left != 0 {
			notes = append(notes, "left at "+ui.Clock(p.Left))
		}
		rows[i] = append(row, strings.Join(notes, ", "))
	}

	return w.Table(cols, rows, func(i int) lipgloss.Style {
		switch {
		case ps[i].Bot:
			return ui.Muted
		case i == 0 && ps[i].Stats["score"] > 0:
			return ui.Accent
		}
		return lipgloss.NewStyle()
	})
}

// platoonLetters names the platoons of ps with two or more players A, B, ... in order of appearance.
func platoonLetters(ps []*battle.Player) map[int64]string {
	size := map[int64]int{}
	for _, p := range ps {
		size[p.Group]++
	}
	letters := map[int64]string{}
	for _, p := range ps {
		if _, ok := letters[p.Group]; ok || p.Group == 0 || size[p.Group] < 2 {
			continue
		}
		letters[p.Group] = string(rune('A' + len(letters)))
	}
	return letters
}

// number formats a counter with at most one decimal; captures count zone parts.
func number(v float64) string {
	v = math.Round(v*10) / 10
	if v == math.Trunc(v) {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}
