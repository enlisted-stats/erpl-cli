package cli

import (
	"cmp"
	"fmt"
	"math"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/enlisted-stats/erpl"
	"github.com/enlisted-stats/erpl/battle"
	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/gamedata"
	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

var kindNames = map[erpl.PhysKind]string{
	erpl.PhysVehicle: "ground",
	erpl.PhysPlane:   "plane",
	erpl.PhysShip:    "ship",
}

func vehiclesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "vehicles [number]",
		Short: "Show tanks, planes and other vehicles, who called them in and how they did",
		Long: "Show tanks, planes and other vehicles, who called them in and how they did.\n\n" +
			"Guns built by engineers are listed too, with the kind gun.",
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

			destroyed := 0
			rows := make([][]string, len(b.Vehicles))
			for i, v := range b.Vehicles {
				end, fate, by, weapon := m.LastRecordTime, ui.Good.Render("survived"), "", ""
				if d := v.Death; d != nil {
					end, fate, by, weapon = d.Time, ui.Clock(d.Time), "-", weaponName(gd, d.Weapon)
					if d.Killer != nil {
						by = soldierName(d.Killer)
					}
					destroyed++
				}
				player, team := v.Player, v.Team
				if s := v.Structure; s != nil {
					player, team = s.Player, s.Team
				}
				rows[i] = []string{
					ui.Clock(v.Spawned), vehicleName(gd, v), vehicleKind(gd, v),
					ui.Team(team).Render(playerName(player)), fate, by, weapon, ui.Clock(max(end-v.Spawned, 0)),
					strconv.Itoa(crewSize(v)), fmt.Sprintf("%.1f", distance(v.Track)/1000),
				}
			}

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprint(w, header(rp.Index, rp.Path))
			fmt.Fprintln(w, w.Table([]ui.Column{
				{Title: "Spawned", Right: true}, {Title: "Vehicle", Cut: true}, {Title: "Kind"}, {Title: "Player", Cut: true},
				{Title: "Destroyed", Right: true}, {Title: "By", Cut: true}, {Title: "Weapon", Cut: true}, {Title: "Lived", Right: true}, {Title: "Crew", Right: true}, {Title: "km", Right: true},
			}, rows, func(i int) lipgloss.Style {
				if v := b.Vehicles[i]; v.Player == nil && v.Structure == nil {
					return ui.Muted
				}
				return lipgloss.NewStyle()
			}))
			fmt.Fprintln(w, ui.Muted.Render(fmt.Sprintf("%d vehicles · %d destroyed", len(b.Vehicles), destroyed)))
			return nil
		},
	}
}

// crewSize is the number of different soldiers who sat in the vehicle.
func crewSize(v *battle.Vehicle) int {
	seen := map[erpl.EntityID]bool{}
	for _, c := range v.Crew {
		seen[c.SoldierID] = true
	}
	return len(seen)
}

// distance is the length of the track on the ground plane, in meters.
func distance(t battle.Track) float64 {
	d := 0.0
	for i := 1; i < len(t); i++ {
		a, b := t[i-1].Pos, t[i].Pos
		d += math.Hypot(float64(b[0]-a[0]), float64(b[2]-a[2]))
	}
	return d
}

// vehicleKind is the kind of a vehicle the game counts in its statistics, e.g. "tank",
// "gun" for a gun built by an engineer, or the kind of physics for other vehicles.
func vehicleKind(gd *gamedata.Data, v *battle.Vehicle) string {
	if v.Structure != nil {
		return "gun"
	}
	return cmp.Or(gd.Vehicles[ui.Model(v.Template)].Kind, kindNames[v.Kind])
}
