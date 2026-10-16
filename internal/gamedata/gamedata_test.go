package gamedata

import (
	"testing"

	"github.com/enlisted-stats/erpl"
)

// TestLoad checks that the built-in data is of the game version erpl reads.
func TestLoad(t *testing.T) {
	d, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if d.Game != erpl.GameVersion {
		t.Errorf("game data of %s, erpl reads %s: regenerate the data with enlisted-gamedata", d.Game, erpl.GameVersion)
	}
	for key, m := range d.Maps {
		if _, err := d.Image(m); err != nil {
			t.Errorf("map %s: %v", key, err)
		}
	}
}
