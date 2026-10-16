package cli

import (
	"cmp"
	"fmt"
	"sync"

	"github.com/enlisted-stats/erpl/battle"

	"github.com/enlisted-stats/erpl-cli/internal/gamedata"
)

// game is the game data built into erpl, read on first use.
var game = sync.OnceValues(gamedata.Load)

// named is the English name, or id when the game has no name.
func named(n gamedata.Name, id string) string { return cmp.Or(n.In("en"), id) }

// teamArmy is the army of the players of team n, e.g. "ussr"; empty when the team has none.
func teamArmy(b *battle.Battle, n int) string {
	for _, p := range b.Players {
		if p.Team == n && p.Army != "" {
			return p.Army
		}
	}
	return ""
}

// teamName is the name of the army of team n, e.g. "USSR", or "team n" when the army
// is not known.
func teamName(gd *gamedata.Data, b *battle.Battle, n int) string {
	if army := teamArmy(b, n); army != "" {
		return named(gd.Armies[army].Name, army)
	}
	return fmt.Sprintf("team %d", n)
}
