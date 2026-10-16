package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/enlisted-stats/erpl"
	"github.com/enlisted-stats/erpl/battle"
	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/home"
	"github.com/enlisted-stats/erpl-cli/internal/replays"
)

// New returns the root command of erpl.
func New() *cobra.Command {
	root := &cobra.Command{
		Use:   "erpl",
		Short: "Look into Enlisted replays",
		Long: "Look into Enlisted replays.\n\n" +
			"Download replays from https://enlisted.net/replays into the replays folder next to erpl. " +
			"Commands take the replay number shown by erpl list; without a number they use the latest replay.",
	}
	root.AddCommand(listCommand(), infoCommand(), playersCommand(), killsCommand(),
		vehiclesCommand(), weaponsCommand(), structuresCommand(), zonesCommand(), reportCommand())
	return root
}

// scan lists the replays folder, creating it if it does not exist yet.
func scan() ([]replays.Replay, error) {
	d, err := home.Find()
	if err != nil {
		return nil, err
	}
	rs, err := replays.Scan(d.Replays())
	if errors.Is(err, fs.ErrNotExist) {
		err = os.MkdirAll(d.Replays(), 0o755)
	}
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, fmt.Errorf("%w in %s: download them from https://enlisted.net/replays", replays.ErrEmpty, d.Replays())
	}
	return rs, nil
}

// open opens the replay chosen by the optional number argument.
func open(args []string) (replays.Replay, *erpl.Reader, error) {
	rs, err := scan()
	if err != nil {
		return replays.Replay{}, nil, err
	}
	arg := ""
	if len(args) > 0 {
		arg = args[0]
	}
	rp, err := replays.Pick(rs, arg)
	if err != nil {
		return replays.Replay{}, nil, err
	}
	r, err := erpl.Open(rp.Path)
	if err != nil {
		return replays.Replay{}, nil, fmt.Errorf("replay #%d: %w", rp.Index, err)
	}
	return rp, r, nil
}

// read decodes the battle of the replay chosen by the optional number argument.
func read(args []string) (replays.Replay, erpl.Meta, *battle.Battle, error) {
	rp, r, err := open(args)
	if err != nil {
		return replays.Replay{}, erpl.Meta{}, nil, err
	}
	if v := r.Meta().ExeVersion; v != erpl.GameVersion {
		return replays.Replay{}, erpl.Meta{}, nil, fmt.Errorf("replay #%d is of game %s, erpl reads replays of %s", rp.Index, v, erpl.GameVersion)
	}
	b, err := battle.Read(r)
	if err != nil {
		return replays.Replay{}, erpl.Meta{}, nil, fmt.Errorf("replay #%d: %w", rp.Index, err)
	}
	return rp, r.Meta(), b, nil
}

// playerName is the name of p, or "AI" for vehicles and soldiers without a player.
func playerName(p *battle.Player) string {
	if p == nil {
		return "AI"
	}
	return p.Name
}

// playerTeam is the team of p, or zero without a player.
func playerTeam(p *battle.Player) int {
	if p == nil {
		return 0
	}
	return p.Team
}
