package report

import (
	"encoding/json"
	"math"
	"time"

	"github.com/enlisted-stats/erpl/battle"
)

// trackStep is the time between the track points the page gets.
const trackStep = time.Second

// track is a track thinned to a point per trackStep, with the time in tenths of a
// second and the position in whole meters. In JSON each point is the difference to
// the point before: dt, dx, dz, dt, dx, dz, ... The first point is the difference to
// zero, so it holds the values themselves.
type track [][3]int

func trackOf(t battle.Track) track {
	var out track
	next := time.Duration(math.MinInt64)
	for _, p := range t {
		if p.Time < next {
			continue
		}
		next = p.Time + trackStep
		out = append(out, [3]int{int(math.Round(p.Time.Seconds() * 10)), int(math.Round(float64(p.Pos[0]))), int(math.Round(float64(p.Pos[2])))})
	}
	return out
}

func (t track) MarshalJSON() ([]byte, error) {
	flat := make([]int, 0, 3*len(t))
	var prev [3]int
	for _, p := range t {
		flat = append(flat, p[0]-prev[0], p[1]-prev[1], p[2]-prev[2])
		prev = p
	}
	return json.Marshal(flat)
}
