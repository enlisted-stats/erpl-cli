package cli

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

func infoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info [number]",
		Short: "Show what battle a replay is",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, r, err := open(args)
			if err != nil {
				return err
			}
			m := r.Meta()

			perTeam := map[int]int{}
			for _, p := range m.Players {
				perTeam[p.Team]++
			}
			finished := ui.Good.Render("yes")
			if !m.Finished {
				finished = ui.Bad.Render("no")
			}

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprintln(w, header(rp.Index, rp.Path))
			fmt.Fprintln(w, ui.Fields(
				[2]string{"Mission", m.Mission},
				[2]string{"Mode", fmt.Sprintf("%s · %s · %s", m.MissionType, m.Mode, m.Difficulty)},
				[2]string{"Campaign", m.Campaign},
				[2]string{"Started", ui.Date(m.StartTime)},
				[2]string{"Length", ui.Clock(m.Duration)},
				[2]string{"Finished", finished},
				[2]string{"Players", fmt.Sprintf("%d: %s vs %s", len(m.Players),
					ui.Team(1).Render(strconv.Itoa(perTeam[1])), ui.Team(2).Render(strconv.Itoa(perTeam[2])))},
				[2]string{"Game", fmt.Sprintf("%s, protocol %d", m.ExeVersion, m.NetProtocol)},
				[2]string{"Session", strconv.FormatUint(m.SessionID, 10)},
			))
			return nil
		},
	}
}

// header is the first line of a command's output about one replay.
func header(index int, path string) string {
	return ui.Title.Render(fmt.Sprintf("Replay #%d", index)) + "  " + ui.Muted.Render(filepath.Base(path)) + "\n"
}
