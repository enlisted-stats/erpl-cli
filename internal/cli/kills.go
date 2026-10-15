package cli

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/enlisted-stats/erpl/battle"
	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

func killsCommand() *cobra.Command {
	var player string
	c := &cobra.Command{
		Use:   "kills [number]",
		Short: "Show who killed whom, in order of time",
		Long: "Show who killed whom, in order of time.\n\n" +
			"A name is the player who owns the soldier's squad: most soldiers are the bots of a squad.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, _, b, err := read(args)
			if err != nil {
				return err
			}
			player = strings.ToLower(player)
			seats := seatsOf(b)

			var deaths []*battle.Soldier
			for _, s := range b.Soldiers {
				if s.Death == nil {
					continue
				}
				if player != "" && !hasName(s.Player, player) && (s.Death.Killer == nil || !hasName(s.Death.Killer.Player, player)) {
					continue
				}
				deaths = append(deaths, s)
			}
			slices.SortStableFunc(deaths, func(a, b *battle.Soldier) int { return cmp.Compare(a.Death.Time, b.Death.Time) })

			rows := make([][]string, len(deaths))
			for i, s := range deaths {
				d := s.Death
				killer, notes := "-", []string(nil)
				switch k := d.Killer; {
				case k == s:
					notes = append(notes, "suicide")
				case k == nil:
					notes = append(notes, "not by a soldier")
				default:
					killer = soldierName(k)
					if v := seats.vehicle(k, d.Time); v != nil {
						notes = append(notes, "from "+ui.Model(v.Template))
					}
					if playerTeam(k.Player) == playerTeam(s.Player) {
						notes = append(notes, "team kill")
					}
				}
				rows[i] = []string{ui.Clock(d.Time), killer, soldierName(s), ui.Muted.Render(strings.Join(notes, ", "))}
			}

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprint(w, header(rp.Index, rp.Path))
			fmt.Fprintln(w, w.Table([]ui.Column{
				{Title: "Time", Right: true}, {Title: "Killer", Cut: true}, {Title: "Killed", Cut: true}, {Title: "Notes", Cut: true},
			}, rows, nil))
			fmt.Fprintln(w, ui.Muted.Render(fmt.Sprintf("%d deaths", len(deaths))))
			return nil
		},
	}
	c.Flags().StringVar(&player, "player", "", "only deaths where the killer or the killed player's name contains this text")
	return c
}

// soldierName is the name of the soldier's player in the team color.
func soldierName(s *battle.Soldier) string {
	return ui.Team(playerTeam(s.Player)).Render(playerName(s.Player))
}

// hasName tells whether the name of p contains the lowercase text.
func hasName(p *battle.Player, text string) bool {
	return p != nil && strings.Contains(strings.ToLower(p.Name), text)
}

// seats finds the vehicle a soldier sat in at a given time.
type seats map[*battle.Soldier][]seat

type seat struct {
	vehicle  *battle.Vehicle
	from, to time.Duration
}

func seatsOf(b *battle.Battle) seats {
	ss := seats{}
	for _, v := range b.Vehicles {
		for _, c := range v.Crew {
			if c.Soldier != nil {
				ss[c.Soldier] = append(ss[c.Soldier], seat{v, c.From, c.To})
			}
		}
	}
	return ss
}

func (ss seats) vehicle(s *battle.Soldier, t time.Duration) *battle.Vehicle {
	for _, st := range ss[s] {
		if st.from <= t && (st.to == 0 || t <= st.to) {
			return st.vehicle
		}
	}
	return nil
}
