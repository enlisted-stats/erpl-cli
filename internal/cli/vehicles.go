package cli

import (
	"fmt"
	"math"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/enlisted-stats/erpl"
	"github.com/enlisted-stats/erpl/battle"
	"github.com/spf13/cobra"

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
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, m, b, err := read(args)
			if err != nil {
				return err
			}

			destroyed := 0
			rows := make([][]string, len(b.Vehicles))
			for i, v := range b.Vehicles {
				end, fate := v.Destroyed, ui.Clock(v.Destroyed)
				if v.Destroyed == 0 {
					end, fate = m.LastRecordTime, ui.Good.Render("survived")
				} else {
					destroyed++
				}
				rows[i] = []string{
					ui.Clock(v.Spawned), ui.Model(v.Template), kindNames[v.Kind],
					ui.Team(v.Team).Render(playerName(v.Player)), fate, ui.Clock(max(end-v.Spawned, 0)),
					strconv.Itoa(crewSize(v)), fmt.Sprintf("%.1f", distance(v.Track)/1000),
				}
			}

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprint(w, header(rp.Index, rp.Path))
			fmt.Fprintln(w, w.Table([]ui.Column{
				{Title: "Spawned", Right: true}, {Title: "Vehicle", Cut: true}, {Title: "Kind"}, {Title: "Player", Cut: true},
				{Title: "Destroyed", Right: true}, {Title: "Lived", Right: true}, {Title: "Crew", Right: true}, {Title: "km", Right: true},
			}, rows, func(i int) lipgloss.Style {
				if b.Vehicles[i].Player == nil {
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
