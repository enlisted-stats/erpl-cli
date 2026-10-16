package report

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/enlisted-stats/erpl"
	"github.com/enlisted-stats/erpl/battle"

	"github.com/enlisted-stats/erpl-cli/internal/gamedata"
)

// data is what the page gets as JSON. Times are seconds since the start of the
// recording, positions are world (x, z) in meters. An index into a list is -1 when
// there is nothing to point to.
type data struct {
	Meta       meta            `json:"meta"`
	Map        *mapImage       `json:"map"` // nil when the map is not known
	Areas      []gamedata.Area `json:"areas"`
	Teams      []team          `json:"teams"`
	Players    []player        `json:"players"`
	Weapons    []weapon        `json:"weapons"`
	Soldiers   []soldier       `json:"soldiers"`
	Vehicles   []vehicle       `json:"vehicles"`
	Structures []structure     `json:"structures"`
	Zones      []zone          `json:"zones"`
	Bases      []base          `json:"bases"`
	Spawns     []spawn         `json:"spawns"`
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
	End          float64 `json:"end"`    // last moment of the recording
	Winner       int     `json:"winner"` // team number; zero when no team won
	Version      string  `json:"version"`
	Replay       string  `json:"replay"` // file name
}

// mapImage is the part of the minimap around the battle.
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

// weapon is what killed: only the weapons of this battle are listed.
type weapon struct {
	Name string `json:"name"` // in-game name, or the game's id when it has none
	Kind string `json:"kind"` // see gamedata.Weapon; empty when not known
}

type soldier struct {
	Player  int     `json:"player"` // index in players
	Team    int     `json:"team"`
	Spawned float64 `json:"spawned"`
	Died    float64 `json:"died,omitempty"`
	Killer  int     `json:"killer"` // index in players
	// KillerSoldier is the index of the killing soldier in soldiers; its track gives
	// the place the kill was made from.
	KillerSoldier int  `json:"killerSoldier"`
	Weapon        int  `json:"weapon"` // index in weapons
	Headshot      bool `json:"headshot,omitempty"`
	// Downs are the knockdowns: time, index of the soldier who knocked down, time of
	// the revival or zero.
	Downs [][3]float64 `json:"downs,omitempty"`
	Track track        `json:"track"`
}

type vehicle struct {
	Model string `json:"model"`
	Name  string `json:"name,omitempty"` // in-game name; empty when not known
	// Kind is "tank", "apc", "plane", "bike", "ship" or "gun" for a gun built by an
	// engineer; "car" for other ground vehicles.
	Kind      string  `json:"kind"`
	Team      int     `json:"team"`
	Player    int     `json:"player"`
	Structure int     `json:"structure"` // index in structures of a gun built by an engineer
	Spawned   float64 `json:"spawned"`
	Destroyed float64 `json:"destroyed,omitempty"`
	// KillerSoldier and Weapon tell who knocked the vehicle out and with what.
	KillerSoldier int   `json:"killerSoldier"`
	Weapon        int   `json:"weapon"`
	Crew          []int `json:"crew"` // indexes in soldiers of everyone who sat in it
	Track         track `json:"track"`
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

var kinds = map[erpl.PhysKind]string{erpl.PhysVehicle: "car", erpl.PhysPlane: "plane", erpl.PhysShip: "ship"}

func build(lang, file string, m erpl.Meta, b *battle.Battle, gd *gamedata.Data, img *mapImage) data {
	mission := gd.Missions[m.Mission]
	d := data{
		Meta: meta{
			Lang: lang, Mission: m.Mission, Mode: m.MissionType, Campaign: m.Campaign,
			MissionName: mission.Name.In(lang), ModeName: gd.Modes[m.MissionType].Name.In(lang), CampaignName: gd.Campaigns[m.Campaign].Name.In(lang),
			Duration: sec(m.Duration), End: sec(m.LastRecordTime), Winner: b.Winner,
			Version: m.ExeVersion, Replay: file,
		},
		Map:     img,
		Areas:   mission.Areas,
		Weapons: []weapon{},
	}
	if !m.StartTime.IsZero() {
		d.Meta.Started = m.StartTime.Format(time.RFC3339)
	}

	players := indexes(b.Players)
	armies := map[int]string{}
	for _, p := range b.Players {
		if armies[p.Team] == "" {
			armies[p.Team] = p.Army
		}
	}
	for _, t := range slices.Sorted(maps.Keys(armies)) {
		if t > 0 {
			d.Teams = append(d.Teams, team{Number: t, Army: armies[t], ArmyName: gd.Armies[armies[t]].Name.In(lang)})
		}
	}

	platoons := platoonLetters(b.Players)
	for _, p := range b.Players {
		d.Players = append(d.Players, player{
			Name: p.Name, Team: p.Team, Bot: p.Bot, Platoon: platoons[p],
			Left: sec(p.Left), Stats: p.Stats,
		})
	}

	weapons := map[string]int{}
	weaponOf := func(key string) int {
		if key == "" {
			return -1
		}
		if i, ok := weapons[key]; ok {
			return i
		}
		w := gd.Weapons[key]
		weapons[key] = len(d.Weapons)
		d.Weapons = append(d.Weapons, weapon{Name: cmp.Or(w.Name.In(lang), key), Kind: w.Kind})
		return weapons[key]
	}

	soldiers := indexes(b.Soldiers)
	for _, s := range b.Soldiers {
		x := soldier{Player: players.of(s.Player), Spawned: sec(s.Spawned), Killer: -1, KillerSoldier: -1, Weapon: -1, Track: trackOf(s.Track)}
		if s.Player != nil {
			x.Team = s.Player.Team
			d.Players[x.Player].Soldiers++
		}
		if dt := s.Death; dt != nil {
			x.Died = sec(dt.Time)
			x.Weapon, x.Headshot = weaponOf(dt.Weapon), dt.Headshot
			if k := dt.Killer; k != nil {
				x.Killer, x.KillerSoldier = players.of(k.Player), soldiers.of(k)
			}
		}
		for _, dn := range s.Downs {
			x.Downs = append(x.Downs, [3]float64{sec(dn.Time), float64(soldiers.of(dn.By)), sec(dn.Revived)})
		}
		d.Soldiers = append(d.Soldiers, x)
	}

	structures := indexes(b.Structures)
	for _, s := range b.Structures {
		d.Structures = append(d.Structures, structure{
			Kind: s.Kind, Name: gd.Structures[s.Kind].Name.In(lang), Rally: isRally(s), Team: s.Team, Player: players.of(s.Player),
			Pos: ground(s.Pos), Built: sec(s.Built), Removed: sec(s.Removed),
		})
	}

	for _, v := range b.Vehicles {
		model, _, _ := strings.Cut(v.Template, "+")
		x := vehicle{
			Model: model, Name: gd.Vehicles[model].Name.In(lang), Kind: cmp.Or(gd.Vehicles[model].Kind, kinds[v.Kind]),
			Team: v.Team, Player: players.of(v.Player), Structure: -1, Spawned: sec(v.Spawned),
			KillerSoldier: -1, Weapon: -1, Crew: []int{}, Track: trackOf(v.Track),
		}
		if s := v.Structure; s != nil {
			x.Model, x.Name, x.Kind = s.Kind, gd.Structures[s.Kind].Name.In(lang), "gun"
			x.Team, x.Player, x.Structure = s.Team, players.of(s.Player), structures.of(s)
		}
		if dt := v.Death; dt != nil {
			x.Destroyed, x.KillerSoldier, x.Weapon = sec(dt.Time), soldiers.of(dt.Killer), weaponOf(dt.Weapon)
		}
		for _, c := range v.Crew {
			if i := soldiers.of(c.Soldier); i >= 0 && !slices.Contains(x.Crew, i) {
				x.Crew = append(x.Crew, i)
			}
		}
		d.Vehicles = append(d.Vehicles, x)
	}

	for _, z := range b.Zones {
		name := strings.TrimPrefix(z.Caption, "capzone/")
		x := zone{Title: z.Title, Name: name, Label: gd.Zones[name].Name.In(lang), Group: z.Group, Pos: ground(z.Pos),
			Radius: round(float64(z.Radius)), Area: z.Area, Active: activeSpans(z, m.LastRecordTime)}
		for _, o := range z.Owner {
			x.Owners = append(x.Owners, [2]float64{sec(o.Time), float64(o.Value)})
		}
		d.Zones = append(d.Zones, x)
	}

	bs, points := bases(b, m.LastRecordTime)
	d.Bases = bs
	d.Spawns = spawns(b, players, points, d.Vehicles)
	return d
}

// index finds the position of an object in its list.
type index[T comparable] map[T]int

func indexes[T comparable](list []T) index[T] {
	ix := index[T]{}
	for i, x := range list {
		ix[x] = i
	}
	return ix
}

// of is the position of x, or -1 when x is not in the list.
func (ix index[T]) of(x T) int {
	if i, ok := ix[x]; ok {
		return i
	}
	return -1
}

func isRally(s *battle.Structure) bool { return strings.HasSuffix(s.Kind, "_rallypoint") }

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
