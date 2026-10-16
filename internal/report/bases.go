package report

import (
	"strings"
	"time"

	"github.com/enlisted-stats/erpl"
	"github.com/enlisted-stats/erpl/battle"
)

// base is a base of a team as the game offers it to choose: the respawn points of one
// kind with the same group. The points of every sector exist from the start of the
// battle and are switched on when their sector opens.
type base struct {
	Kind string     `json:"kind"` // "infantry", "vehicles" or "freeTank"
	Team int        `json:"team"`
	Pos  [2]float64 `json:"pos"` // the middle of its points
	// Active holds the spans when the base was open, in order of time.
	Active [][2]float64 `json:"active"`
}

// basePoint is a respawn point of a base, with the spans it was switched on.
type basePoint struct {
	r     *battle.Respawn
	base  int // index in the bases
	spans [][2]time.Duration
}

// baseKind tells by the template whether a respawn point lets infantry, vehicles or a
// free tank in; it is "" for other points, such as rally points.
func baseKind(template string) string {
	switch {
	case strings.Contains(template, "respTeam"), strings.HasPrefix(template, "respawnBaseWithGroupOnCapturePoint"):
		return "infantry"
	case strings.Contains(template, "resp_free_tank"):
		return "freeTank"
	case strings.Contains(template, "respVehicle"),
		strings.Contains(template, "respAPC"), strings.Contains(template, "respMotorcycle"), strings.Contains(template, "respAircraft"):
		return "vehicles"
	}
	return ""
}

// bases joins the respawn points that were ever switched on into bases: the points of a
// team of one kind, one group and one group of the mission.
func bases(b *battle.Battle, end time.Duration) ([]base, []basePoint) {
	out := []base{}
	var points []basePoint
	var members [][]erpl.Point3
	type key struct {
		kind, groupName string
		team, group     int
	}
	byKey := map[key]int{}
	for _, r := range b.Respawns {
		kind := baseKind(r.Template)
		if kind == "" || r.Structure != nil {
			continue
		}
		until := end
		if r.Removed != 0 {
			until = r.Removed
		}
		spans := switchedOn(r.Active, r.Created, until)
		if len(spans) == 0 {
			continue
		}
		team := r.Team
		if team == 0 {
			team = respawnTeam(r.Template)
		}
		k := key{kind, r.GroupName, team, r.Group}
		i, ok := byKey[k]
		if !ok {
			i = len(out)
			byKey[k] = i
			out = append(out, base{Kind: kind, Team: team})
			members = append(members, nil)
		}
		members[i] = append(members[i], r.Pos)
		for _, s := range spans {
			out[i].Active = addSpan(out[i].Active, [2]float64{sec(s[0]), sec(s[1])})
		}
		points = append(points, basePoint{r: r, base: i, spans: spans})
	}
	for i, ps := range members {
		var x, z float64
		for _, p := range ps {
			x += float64(p[0])
			z += float64(p[2])
		}
		out[i].Pos = [2]float64{round(x / float64(len(ps))), round(z / float64(len(ps)))}
	}
	return out, points
}

// switchedOn turns the changes of a switch into the spans it was on between from and
// to. Before the first change the switch is in the state the change leaves, unless the
// change came at from: then it holds the initial state. Without changes it is on all
// the time, as the templates set it.
func switchedOn(cs []battle.Change[bool], from, to time.Duration) [][2]time.Duration {
	if len(cs) == 0 {
		return [][2]time.Duration{{from, to}}
	}
	var spans [][2]time.Duration
	start, on := from, cs[0].Time != from && !cs[0].Value
	for _, c := range cs {
		switch {
		case c.Value && !on:
			start, on = c.Time, true
		case !c.Value && on:
			if c.Time > start {
				spans = append(spans, [2]time.Duration{start, c.Time})
			}
			on = false
		}
	}
	if on && to > start {
		spans = append(spans, [2]time.Duration{start, to})
	}
	return spans
}

// addSpan adds a span to spans in order of time, joining the ones that overlap or
// touch.
func addSpan(spans [][2]float64, s [2]float64) [][2]float64 {
	out := [][2]float64{}
	for _, x := range spans {
		switch {
		case x[1] < s[0]-1 || x[0] > s[1]+1:
			out = append(out, x)
		default:
			s = [2]float64{min(s[0], x[0]), max(s[1], x[1])}
		}
	}
	out = append(out, s)
	for i := len(out) - 1; i > 0 && out[i][0] < out[i-1][0]; i-- {
		out[i], out[i-1] = out[i-1], out[i]
	}
	return out
}

// openAt tells whether a point was switched on at t, give or take a couple of seconds.
func (p basePoint) openAt(t time.Duration) bool {
	for _, s := range p.spans {
		if s[0] <= t+2*time.Second && t <= s[1]+2*time.Second {
			return true
		}
	}
	return false
}
