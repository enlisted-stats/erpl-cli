// Package gamedata reads the game data built into erpl: what a replay refers to by
// id, with the names in the game languages.
package gamedata

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/enlisted-stats/erpl-cli/data"
)

// Data is the game data of one game version.
type Data struct {
	Game       string             // game version, e.g. "0.7.6.63"
	Maps       map[string]Map     // by minimap texture, e.g. "manchuria_det_minimap"
	Missions   map[string]Mission // by scene, as in erpl.Meta.Mission
	Vehicles   map[string]Vehicle // by template, e.g. "germ_pzkpfw_III_ausf_J"
	Weapons    map[string]Weapon  // by the weapon of a kill, as in battle.Death.Weapon
	Modes      map[string]Entry   // by mission type, e.g. "destruction"
	Campaigns  map[string]Entry   // e.g. "moscow"
	Armies     map[string]Entry   // by army of a player, e.g. "ger"
	Zones      map[string]Entry   // by zone caption without "capzone/", e.g. "canteen"
	Structures map[string]Entry   // by structure kind, as in battle.Structure.Kind
}

// Name is a name by language code, e.g. "ru". A language without a translation is missing.
type Name map[string]string

// In returns the name in the language lang, the English one when there is no
// translation, or "" when there is no name at all.
func (n Name) In(lang string) string {
	if s := n[lang]; s != "" {
		return s
	}
	return n["en"]
}

// Entry is something that has only a name.
type Entry struct {
	Name Name `json:"name"`
}

// Map is a minimap image and the part of the world it shows.
type Map struct {
	Image string `json:"image"` // file in data.FS; empty when the game has no image
	// LeftTop and RightBottom are the world (x, z) at the corners of the image.
	LeftTop     [2]float32 `json:"leftTop"`
	RightBottom [2]float32 `json:"rightBottom"`
	NorthAngle  float32    `json:"northAngle"` // degrees
}

// Mission is what the game sets up for one mission.
type Mission struct {
	Map   string `json:"map"` // key in Data.Maps
	Areas []Area `json:"areas"`
	Name  Name   `json:"name"`
}

// Area is a battle area: leaving it kills soldiers or vehicles after a few seconds.
type Area struct {
	Air    bool         `json:"air,omitempty"`   // limits planes, not soldiers and ground vehicles
	Team   int          `json:"team"`            // 1 or 2, or -1 for both teams
	Group  string       `json:"group,omitempty"` // areas of a group are switched together, e.g. "sector_1_a"
	Active bool         `json:"active"`          // at the start of the battle
	Points [][2]float32 `json:"points"`          // outline (x, z)
}

// Vehicle is a vehicle template.
type Vehicle struct {
	Kind string `json:"kind"` // "tank", "apc", "plane", "bike" or "ship"; empty for vehicles the game does not count
	Name Name   `json:"name"`
}

// Weapon is what a kill is credited to.
type Weapon struct {
	Kind string `json:"kind"` // e.g. "rifle", "grenade", "cannon", "artillery"; empty when not known
	Name Name   `json:"name"`
}

// Load reads the game data. All files must be of the same game version.
func Load() (*Data, error) {
	var d Data
	files := []struct {
		name string
		dst  any
	}{
		{"maps", &d.Maps}, {"missions", &d.Missions}, {"vehicles", &d.Vehicles}, {"weapons", &d.Weapons},
		{"modes", &d.Modes}, {"campaigns", &d.Campaigns}, {"armies", &d.Armies}, {"zones", &d.Zones}, {"structures", &d.Structures},
	}
	for _, f := range files {
		raw, err := fs.ReadFile(data.FS, f.name+".json")
		if err != nil {
			return nil, err
		}
		var file map[string]json.RawMessage
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, fmt.Errorf("game data %s: %w", f.name, err)
		}
		var game string
		if err := json.Unmarshal(file["game"], &game); err != nil {
			return nil, fmt.Errorf("game data %s: game version: %w", f.name, err)
		}
		if d.Game == "" {
			d.Game = game
		} else if game != d.Game {
			return nil, fmt.Errorf("game data %s is of game %s, the other files of %s", f.name, game, d.Game)
		}
		if err := json.Unmarshal(file[f.name], f.dst); err != nil {
			return nil, fmt.Errorf("game data %s: %w", f.name, err)
		}
	}
	return &d, nil
}

// Image returns the JPEG image of the map; nil when the game has no image.
func (d *Data) Image(m Map) ([]byte, error) {
	if m.Image == "" {
		return nil, nil
	}
	return fs.ReadFile(data.FS, m.Image)
}
