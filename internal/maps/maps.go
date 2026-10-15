package maps

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// File is the name of the index in the data folder.
const File = "maps.json"

// Index describes the maps of the game: written by erpl-mapgen, read by erpl.
type Index struct {
	Maps     map[string]Map     `json:"maps"`     // by texture name, e.g. "manchuria_det_minimap"
	Missions map[string]Mission `json:"missions"` // by mission name, as in erpl.Meta.Mission
}

// Map is a minimap image and the part of the world it shows.
type Map struct {
	Image string `json:"image,omitempty"` // file in the maps folder
	// LeftTop and RightBottom are the world (x, z) at the corners of the image.
	LeftTop     [2]float32 `json:"leftTop"`
	RightBottom [2]float32 `json:"rightBottom"`
	NorthAngle  float32    `json:"northAngle"` // degrees
}

// Mission is what the game sets up for one mission.
type Mission struct {
	Map   string `json:"map"` // key in Index.Maps
	Areas []Area `json:"areas,omitempty"`
}

// Area is a battle area: leaving it kills soldiers or vehicles after a few seconds.
type Area struct {
	Air    bool         `json:"air,omitempty"`   // limits planes, not soldiers and ground vehicles
	Team   int          `json:"team"`            // 1 or 2, or -1 for both teams
	Group  string       `json:"group,omitempty"` // areas of a group are switched together, e.g. "sector_1_a"
	Active bool         `json:"active"`          // at the start of the battle
	Points [][2]float32 `json:"points"`          // outline (x, z)
}

// Load reads the index from the data folder dir.
func Load(dir string) (Index, error) {
	var ix Index
	data, err := os.ReadFile(filepath.Join(dir, File))
	if err != nil {
		return ix, err
	}
	err = json.Unmarshal(data, &ix)
	return ix, err
}

// Save writes the index into the data folder dir.
func (ix Index) Save(dir string) error {
	data, err := json.Marshal(ix)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, File), data, 0o644)
}
