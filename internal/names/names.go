package names

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Names are the in-game names of what a replay refers to by id, in one language:
// written by erpl-mapgen from the game's translations, read by erpl.
type Names struct {
	Missions   map[string]string `json:"missions"`   // by mission, e.g. "volokolamsk_city_bomb"
	Modes      map[string]string `json:"modes"`      // by mission type, e.g. "destruction"
	Campaigns  map[string]string `json:"campaigns"`  // e.g. "moscow"
	Armies     map[string]string `json:"armies"`     // by army of a player, e.g. "ger"
	Zones      map[string]string `json:"zones"`      // by zone caption without "capzone/", e.g. "canteen"
	Vehicles   map[string]string `json:"vehicles"`   // by lowercase vehicle model, e.g. "germ_pzkpfw_iii_ausf_b"
	Structures map[string]string `json:"structures"` // by structure kind, e.g. "sandbags_wall_2"
}

// File is the name of the file with the names in a language, e.g. "names.ru.json".
func File(lang string) string { return "names." + lang + ".json" }

// Load reads the names in a language from the folder dir. Missing names are not an
// error: the result is then empty and ids are shown as they are.
func Load(dir, lang string) (Names, error) {
	var n Names
	data, err := os.ReadFile(filepath.Join(dir, File(lang)))
	if errors.Is(err, fs.ErrNotExist) {
		return n, nil
	}
	if err != nil {
		return n, err
	}
	err = json.Unmarshal(data, &n)
	return n, err
}

// Save writes the names in a language into the folder dir.
func (n Names) Save(dir, lang string) error {
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, File(lang)), data, 0o644)
}
