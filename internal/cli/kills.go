package cli

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/enlisted-stats/erpl/battle"
	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/gamedata"
	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

func killsCommand() *cobra.Command {
	var player, weapon, kind string
	var vehicles bool
	c := &cobra.Command{
		Use:   "kills [number]",
		Short: "Show who killed whom and with what, in order of time",
		Long: "Show who killed whom and with what, in order of time: soldiers and vehicles.\n\n" +
			"A name is the player who owns the soldier's squad: most soldiers are the bots of a squad.",
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
			player, weapon = strings.ToLower(player), strings.ToLower(weapon)
			seats := seatsOf(b)

			kills := slices.DeleteFunc(killsOf(b), func(k kill) bool {
				var killer *battle.Player
				if k.Killer != nil {
					killer = k.Killer.Player
				}
				w := gd.Weapons[k.Weapon]
				return vehicles && k.vehicle == nil ||
					player != "" && !hasName(k.player, player) && !hasName(killer, player) ||
					weapon != "" && !strings.Contains(strings.ToLower(k.Weapon), weapon) && !strings.Contains(strings.ToLower(w.Name.In("en")), weapon) ||
					kind != "" && w.Kind != kind
			})
			slices.SortStableFunc(kills, func(a, b kill) int { return cmp.Compare(a.Time, b.Time) })

			rows := make([][]string, len(kills))
			for i, k := range kills {
				killed := ""
				if k.soldier != nil {
					killed = soldierName(k.soldier)
				} else {
					killed = ui.Team(k.team).Render(vehicleName(gd, k.vehicle) + " (" + playerName(k.player) + ")")
				}
				killer, notes := "-", []string(nil)
				switch kr := k.Killer; {
				case kr == nil:
					notes = append(notes, "not by a soldier")
				case kr == k.soldier:
					notes = append(notes, "suicide")
				default:
					killer = soldierName(kr)
					if v := seats.vehicle(kr, k.Time); v != nil && v != k.vehicle {
						notes = append(notes, "from "+vehicleName(gd, v))
					}
					if playerTeam(kr.Player) == k.team {
						notes = append(notes, "team kill")
					}
				}
				if k.Headshot {
					notes = append(notes, "headshot")
				}
				rows[i] = []string{ui.Clock(k.Time), killer, killed, weaponName(gd, k.Weapon), ui.Muted.Render(strings.Join(notes, ", "))}
			}

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprint(w, header(rp.Index, rp.Path))
			fmt.Fprintln(w, w.Table([]ui.Column{
				{Title: "Time", Right: true}, {Title: "Killer", Cut: true}, {Title: "Killed", Cut: true},
				{Title: "Weapon", Cut: true}, {Title: "Notes", Cut: true},
			}, rows, nil))
			soldiers := 0
			for _, k := range kills {
				if k.soldier != nil {
					soldiers++
				}
			}
			fmt.Fprintln(w, ui.Muted.Render(fmt.Sprintf("%d soldiers · %d vehicles", soldiers, len(kills)-soldiers)))
			return nil
		},
	}
	c.Flags().StringVar(&player, "player", "", "only kills where the killer or the killed player's name contains this text")
	c.Flags().StringVar(&weapon, "weapon", "", "only kills by a weapon whose name contains this text, e.g. ppsh")
	c.Flags().StringVar(&kind, "kind", "", "only kills by a kind of weapon, e.g. rifle, grenade, cannon, artillery")
	c.Flags().BoolVar(&vehicles, "vehicles", false, "only destroyed vehicles")
	return c
}

// kill is the death of a soldier or of a vehicle.
type kill struct {
	*battle.Death
	soldier *battle.Soldier // nil for a vehicle
	vehicle *battle.Vehicle // nil for a soldier
	player  *battle.Player  // whose soldier or vehicle it was
	team    int
}

// killsOf returns the deaths of soldiers and vehicles. A gun built by an engineer
// belongs to the player who built it.
func killsOf(b *battle.Battle) []kill {
	var kills []kill
	for _, s := range b.Soldiers {
		if s.Death != nil {
			kills = append(kills, kill{Death: s.Death, soldier: s, player: s.Player, team: playerTeam(s.Player)})
		}
	}
	for _, v := range b.Vehicles {
		if v.Death == nil {
			continue
		}
		k := kill{Death: v.Death, vehicle: v, player: v.Player, team: v.Team}
		if s := v.Structure; s != nil {
			k.player, k.team = s.Player, s.Team
		}
		kills = append(kills, k)
	}
	return kills
}

// soldierName is the name of the soldier's player in the team color.
func soldierName(s *battle.Soldier) string {
	return ui.Team(playerTeam(s.Player)).Render(playerName(s.Player))
}

// vehicleName is the in-game name of a vehicle, or its model when the game has no name.
// A gun built by an engineer is named as a structure.
func vehicleName(gd *gamedata.Data, v *battle.Vehicle) string {
	if s := v.Structure; s != nil {
		return named(gd.Structures[s.Kind].Name, s.Kind)
	}
	model := ui.Model(v.Template)
	return named(gd.Vehicles[model].Name, model)
}

// weaponName is the in-game name of the weapon of a kill; "-" when the game names none.
func weaponName(gd *gamedata.Data, weapon string) string {
	if weapon == "" {
		return "-"
	}
	return named(gd.Weapons[weapon].Name, weapon)
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
