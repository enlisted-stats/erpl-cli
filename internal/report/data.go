package report

import (
	stdmaps "maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/enlisted-stats/erpl"
	"github.com/enlisted-stats/erpl/battle"

	"github.com/enlisted-stats/erpl-cli/internal/maps"
	"github.com/enlisted-stats/erpl-cli/internal/names"
)

// data is what the page gets as JSON. Times are seconds since the start of the
// recording, positions are world (x, z) in meters.
type data struct {
	Meta       meta        `json:"meta"`
	Map        *mapImage   `json:"map"` // nil when the map is not known
	Areas      []maps.Area `json:"areas"`
	Teams      []team      `json:"teams"`
	Players    []player    `json:"players"`
	Soldiers   []soldier   `json:"soldiers"`
	Vehicles   []vehicle   `json:"vehicles"`
	Structures []structure `json:"structures"`
	Zones      []zone      `json:"zones"`
	Respawns   []respawn   `json:"respawns"`
}

type meta struct {
	Lang     string `json:"lang"` // language of the page, e.g. "en"
	Mission  string `json:"mission"`
	Mode     string `json:"mode"`
	Campaign string `json:"campaign"`
	// MissionName, ModeName and CampaignName are the in-game names; empty when not known.
	MissionName  string  `json:"missionName,omitempty"`
	ModeName     string  `json:"modeName,omitempty"`
	CampaignName string  `json:"campaignName,omitempty"`
	Started      string  `json:"started"` // RFC 3339
	Duration     float64 `json:"duration"`
	End          float64 `json:"end"` // last moment of the recording
	Version      string  `json:"version"`
	Replay       string  `json:"replay"` // file name
}

type mapImage struct {
	Image       string     `json:"image"` // data URL
	LeftTop     [2]float32 `json:"leftTop"`
	RightBottom [2]float32 `json:"rightBottom"`
}

type team struct {
	Number   int    `json:"number"`
	Army     string `json:"army"`
	ArmyName string `json:"armyName,omitempty"` // in-game name; empty when not known
}

type player struct {
	Name     string             `json:"name"`
	Team     int                `json:"team"`
	Bot      bool               `json:"bot,omitempty"`
	Platoon  string             `json:"platoon,omitempty"`
	Left     float64            `json:"left,omitempty"`
	Stats    map[string]float64 `json:"stats"`
	Soldiers int                `json:"soldiers"`
}

type soldier struct {
	Player  int     `json:"player"` // index in players, or -1
	Team    int     `json:"team"`
	Spawned float64 `json:"spawned"`
	Died    float64 `json:"died,omitempty"`
	Killer  int     `json:"killer"` // index in players, or -1
	// KillerSoldier is the index of the killing soldier in soldiers, or -1; its track
	// gives the place the kill was made from.
	KillerSoldier int       `json:"killerSoldier"`
	Track         []float64 `json:"track"` // t, x, z, t, x, z, ...
}

type vehicle struct {
	Model     string    `json:"model"`
	Name      string    `json:"name,omitempty"` // in-game name; empty when not known
	Kind      string    `json:"kind"`
	Team      int       `json:"team"`
	Player    int       `json:"player"`
	Spawned   float64   `json:"spawned"`
	Destroyed float64   `json:"destroyed,omitempty"`
	Track     []float64 `json:"track"`
}

type structure struct {
	Kind    string     `json:"kind"`
	Name    string     `json:"name,omitempty"` // in-game name; empty when not known
	Rally   bool       `json:"rally,omitempty"`
	Team    int        `json:"team"`
	Player  int        `json:"player"`
	Pos     [2]float64 `json:"pos"`
	Built   float64    `json:"built"`
	Removed float64    `json:"removed,omitempty"`
}

type zone struct {
	Title  string        `json:"title"`
	Name   string        `json:"name"`
	Label  string        `json:"label,omitempty"` // in-game name; empty when not known
	Group  string        `json:"group,omitempty"`
	Pos    [2]float64    `json:"pos"`
	Radius float64       `json:"radius,omitempty"`
	Area   []erpl.Point2 `json:"area,omitempty"`
	Owners [][2]float64  `json:"owners"` // time, team or -1
	// Active holds the spans when the zone was active; nil when the replay has no
	// activity changes, as in modes where all zones are always active.
	Active [][2]float64 `json:"active"`
}

type respawn struct {
	Kind    string     `json:"kind"`
	Team    int        `json:"team"`
	Pos     [2]float64 `json:"pos"`
	Created float64    `json:"created"`
	Removed float64    `json:"removed,omitempty"`
}

// trackStep is the time between the track points the page gets.
const trackStep = time.Second

var kinds = map[erpl.PhysKind]string{erpl.PhysVehicle: "ground", erpl.PhysPlane: "plane", erpl.PhysShip: "ship"}

func build(lang, file string, m erpl.Meta, b *battle.Battle, n names.Names, mission maps.Mission, img *mapImage) data {
	d := data{
		Meta: meta{
			Lang: lang, Mission: m.Mission, Mode: m.MissionType, Campaign: m.Campaign,
			MissionName: n.Missions[m.Mission], ModeName: n.Modes[m.MissionType], CampaignName: n.Campaigns[m.Campaign],
			Duration: sec(m.Duration), End: sec(m.LastRecordTime),
			Version: m.ExeVersion, Replay: file,
		},
		Map:   img,
		Areas: mission.Areas,
	}
	if !m.StartTime.IsZero() {
		d.Meta.Started = m.StartTime.Format(time.RFC3339)
	}

	players := map[*battle.Player]int{}
	for i, p := range b.Players {
		players[p] = i
	}
	index := func(p *battle.Player) int {
		if i, ok := players[p]; ok {
			return i
		}
		return -1
	}
	armies := map[int]string{}
	for _, p := range b.Players {
		if armies[p.Team] == "" {
			armies[p.Team] = p.Army
		}
	}
	for _, t := range slices.Sorted(stdmaps.Keys(armies)) {
		if t > 0 {
			d.Teams = append(d.Teams, team{Number: t, Army: armies[t], ArmyName: n.Armies[armies[t]]})
		}
	}

	platoons := platoonLetters(b.Players)
	for _, p := range b.Players {
		d.Players = append(d.Players, player{
			Name: p.Name, Team: p.Team, Bot: p.Bot, Platoon: platoons[p],
			Left: sec(p.Left), Stats: p.Stats,
		})
	}
	soldiers := map[*battle.Soldier]int{}
	for i, s := range b.Soldiers {
		soldiers[s] = i
	}
	for _, s := range b.Soldiers {
		x := soldier{Player: index(s.Player), Spawned: sec(s.Spawned), Killer: -1, KillerSoldier: -1, Track: track(s.Track)}
		if s.Player != nil {
			x.Team = s.Player.Team
			d.Players[x.Player].Soldiers++
		}
		if s.Death != nil {
			x.Died = sec(s.Death.Time)
			if k := s.Death.Killer; k != nil {
				x.Killer, x.KillerSoldier = index(k.Player), soldiers[k]
			}
		}
		d.Soldiers = append(d.Soldiers, x)
	}
	for _, v := range b.Vehicles {
		model, _, _ := strings.Cut(v.Template, "+")
		d.Vehicles = append(d.Vehicles, vehicle{
			Model: model, Name: n.Vehicles[strings.ToLower(model)], Kind: kinds[v.Kind], Team: v.Team, Player: index(v.Player),
			Spawned: sec(v.Spawned), Destroyed: sec(v.Destroyed), Track: track(v.Track),
		})
	}
	for _, s := range b.Structures {
		d.Structures = append(d.Structures, structure{
			Kind: s.Kind, Name: n.Structures[s.Kind], Rally: strings.HasSuffix(s.Kind, "_rallypoint"), Team: s.Team, Player: index(s.Player),
			Pos: ground(s.Pos), Built: sec(s.Built), Removed: sec(s.Removed),
		})
	}
	for _, z := range b.Zones {
		name := strings.TrimPrefix(z.Caption, "capzone/")
		x := zone{Title: z.Title, Name: name, Label: n.Zones[name], Group: z.Group, Pos: ground(z.Pos),
			Radius: round(float64(z.Radius)), Area: z.Area, Active: activeSpans(z, m.LastRecordTime)}
		for _, o := range z.Owner {
			x.Owners = append(x.Owners, [2]float64{sec(o.Time), float64(o.Value)})
		}
		d.Zones = append(d.Zones, x)
	}
	for _, r := range b.Respawns {
		if r.Structure != nil {
			continue // shown as its rally point
		}
		kind, _, _ := strings.Cut(r.Template, "+")
		t := r.Team
		if t == 0 {
			t = respawnTeam(r.Template)
		}
		d.Respawns = append(d.Respawns, respawn{Kind: kind, Team: t, Pos: ground(r.Pos), Created: sec(r.Created), Removed: sec(r.Removed)})
	}
	return d
}

// track thins a track to a point per trackStep.
func track(t battle.Track) []float64 {
	var out []float64
	next := time.Duration(math.MinInt64)
	for _, p := range t {
		if p.Time < next {
			continue
		}
		next = p.Time + trackStep
		out = append(out, sec(p.Time), round(float64(p.Pos[0])), round(float64(p.Pos[2])))
	}
	return out
}

// activeSpans turns the activity changes of a zone into spans. The value at the
// creation of the zone is its initial state; a span open at the end lasts until end,
// and one that started before the first change starts at the creation.
func activeSpans(z *battle.Zone, end time.Duration) [][2]float64 {
	if len(z.Active) == 0 {
		return nil
	}
	spans := [][2]float64{}
	from, open := z.Created, z.Active[0].Time != z.Created && !z.Active[0].Value
	for _, c := range z.Active {
		switch {
		case c.Value && !open:
			from, open = c.Time, true
		case !c.Value && open:
			spans = append(spans, [2]float64{sec(from), sec(c.Time)})
			open = false
		}
	}
	if open {
		spans = append(spans, [2]float64{sec(from), sec(end)})
	}
	return spans
}

// respawnTeam reads the team from templates like "respawnChooser+respTeam1".
func respawnTeam(template string) int {
	switch {
	case strings.Contains(template, "respTeam1"):
		return 1
	case strings.Contains(template, "respTeam2"):
		return 2
	}
	return 0
}

// platoonLetters names the platoons with two or more players of a team A, B, ...
func platoonLetters(ps []*battle.Player) map[*battle.Player]string {
	type key struct {
		team  int
		group int64
	}
	size := map[key]int{}
	for _, p := range ps {
		size[key{p.Team, p.Group}]++
	}
	letters, perTeam := map[key]string{}, map[int]int{}
	out := map[*battle.Player]string{}
	for _, p := range ps {
		k := key{p.Team, p.Group}
		if p.Group == 0 || size[k] < 2 {
			continue
		}
		if letters[k] == "" {
			letters[k] = string(rune('A' + perTeam[p.Team]))
			perTeam[p.Team]++
		}
		out[p] = letters[k]
	}
	return out
}

func ground(p erpl.Point3) [2]float64 { return [2]float64{round(float64(p[0])), round(float64(p[2]))} }

// sec converts a time to seconds with one decimal; zero stays zero.
func sec(d time.Duration) float64 { return round(d.Seconds()) }

func round(v float64) float64 { return math.Round(v*10) / 10 }
