package report

import (
	"math"
	"strings"
	"time"

	"github.com/enlisted-stats/erpl"
	"github.com/enlisted-stats/erpl/battle"
)

// spawn is a squad entering the battle.
type spawn struct {
	Time   float64    `json:"t"`
	Pos    [2]float64 `json:"pos"`
	Team   int        `json:"team"`
	Player int        `json:"player"` // index in players
	// From is where the squad came from: "base" (Ref in bases; a squad that called in a
	// vehicle came from the base of vehicles), "rally" (Ref in structures), "apc" for an
	// APC of another squad, "vehicle" for its own vehicle out of a base, "air" for a plane
	// with paratroopers (Ref in vehicles), or "" when the place is not known.
	From string `json:"from"`
	Ref  int    `json:"ref"`
}

// spawnReach is how far from a place a squad may appear to have come from it.
const spawnReach = 40

// spawns finds where each squad came from: the nearest place a squad can enter the
// battle from at the moment its first soldier appears.
func spawns(b *battle.Battle, players index[*battle.Player], points []basePoint, vs []vehicle) []spawn {
	out := []spawn{}
	for _, sq := range b.Squads {
		if len(sq.Soldiers) == 0 || len(sq.Soldiers[0].Track) == 0 {
			continue
		}
		first := sq.Soldiers[0].Track[0]
		t := first.Time
		alive := func(from, to time.Duration) bool { return from <= t+2*time.Second && (to == 0 || to >= t) }
		s := spawn{Time: sec(t), Pos: ground(first.Pos), Player: players.of(sq.Player), Ref: -1}
		if sq.Player != nil {
			s.Team = sq.Player.Team
		}
		best := float64(spawnReach)
		near := func(p erpl.Point3, from string, ref int) {
			if d := math.Hypot(float64(p[0]-first.Pos[0]), float64(p[2]-first.Pos[2])); d < best {
				best, s.From, s.Ref = d, from, ref
			}
		}
		for _, p := range points {
			if p.openAt(t) {
				near(p.r.Pos, "base", p.base)
			}
		}
		for i, st := range b.Structures {
			if isRally(st) && alive(st.Built, st.Removed) {
				near(st.Pos, "rally", i)
			}
		}
		for i, v := range b.Vehicles {
			end := time.Duration(0)
			if v.Death != nil {
				end = v.Death.Time
			}
			if !alive(v.Spawned, end) {
				continue
			}
			p, ok := posAt(v.Track, t+2*time.Second)
			if !ok {
				continue
			}
			from := "vehicle"
			switch {
			case strings.Contains(v.Template, "paratroopers"):
				from = "air"
			case vs[i].Kind == "apc" && v.Squad != sq:
				from = "apc"
			}
			near(p, from, i)
		}
		if s.From == "vehicle" {
			if i := vehicleBase(b.Vehicles[s.Ref], points); i >= 0 {
				s.From, s.Ref = "base", i
			}
		}
		out = append(out, s)
	}
	return out
}

// vehicleBase finds the base of vehicles a vehicle was called in at: the nearest open
// point of vehicles where its track starts. It is -1 for a vehicle that did not come
// from a base, such as a plane appearing in the air.
func vehicleBase(v *battle.Vehicle, points []basePoint) int {
	if len(v.Track) == 0 {
		return -1
	}
	start := v.Track[0].Pos
	best, found := float64(spawnReach), -1
	for _, p := range points {
		if k := baseKind(p.r.Template); k != "vehicles" && k != "freeTank" || !p.openAt(v.Spawned) {
			continue
		}
		if d := math.Hypot(float64(p.r.Pos[0]-start[0]), float64(p.r.Pos[2]-start[2])); d < best {
			best, found = d, p.base
		}
	}
	return found
}

// posAt is the last position of a track at or before t.
func posAt(tr battle.Track, t time.Duration) (erpl.Point3, bool) {
	var p erpl.Point3
	ok := false
	for _, x := range tr {
		if x.Time > t {
			break
		}
		p, ok = x.Pos, true
	}
	return p, ok
}
