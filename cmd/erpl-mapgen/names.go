package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/enlisted-stats/erpl-cli/internal/names"
	"github.com/enlisted-stats/erpl-cli/internal/vromfs"
)

// langArchive holds the translations, relative to the game folder.
var langArchive = filepath.Join("content", "enlisted", "enlisted-lang.vromfs.bin")

const langFile = "lang/common/common.csv"

// languages are the codes of the language columns of the translation table.
var languages = map[string]string{
	"<English>": "en", "<German>": "de", "<French>": "fr", "<Spanish>": "es", "<Japanese>": "ja",
	"<Korean>": "ko", "<Chinese>": "zh", "<HChinese>": "zh-Hant", "<Russian>": "ru", "<Polish>": "pl",
	"<Vietnamese>": "vi",
}

// armyKeys are the translation keys of the armies of players: a replay names an army
// by a short code, the translations by the country.
var armyKeys = map[string]string{
	"ussr": "country/ussr", "ger": "country/germany", "usa": "country/usa",
	"jap": "country/japan", "uk": "country/britain", "ita": "country/italy",
}

// missionType is the placeholder for the game mode in the names of missions.
var missionType = regexp.MustCompile(`\s*\(\{mission_type\}\)`)

// variantSuffixes end the names of mission variants.
var variantSuffixes = []string{"_solo", "_axis", "_allies", "_benchmark"}

func cutSuffix(s string, suffixes []string) (string, bool) {
	for _, x := range suffixes {
		if base, ok := strings.CutSuffix(s, x); ok {
			return base, true
		}
	}
	return s, false
}

// translations reads the translation table of the game: for each language code, the
// texts by translation key. An empty text falls back to English.
func translations(gameDir string) (map[string]map[string]string, error) {
	files, err := vromfs.Open(filepath.Join(gameDir, langArchive))
	if err != nil {
		return nil, err
	}
	data, ok := files[langFile]
	if !ok {
		return nil, fmt.Errorf("%s: no %s", langArchive, langFile)
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", langFile, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: empty", langFile)
	}
	columns := map[int]string{}
	english := -1
	for i, h := range rows[0] {
		if code, ok := languages[h]; ok {
			columns[i] = code
			if code == "en" {
				english = i
			}
		}
	}
	texts := map[string]map[string]string{}
	for _, code := range columns {
		texts[code] = map[string]string{}
	}
	for _, row := range rows[1:] {
		if len(row) == 0 || row[0] == "" {
			continue
		}
		for i, code := range columns {
			t := ""
			if i < len(row) {
				t = row[i]
			}
			if t == "" && english >= 0 && english < len(row) {
				t = row[english]
			}
			if t != "" {
				texts[code][row[0]] = t
			}
		}
	}
	return texts, nil
}

// gameNames picks out of the translations of one language the names a report needs.
func (g *game) gameNames(texts map[string]string, missions []string) names.Names {
	n := names.Names{
		Missions: map[string]string{}, Modes: map[string]string{}, Campaigns: map[string]string{},
		Armies: map[string]string{}, Zones: map[string]string{}, Vehicles: map[string]string{},
		Structures: map[string]string{},
	}
	for _, m := range missions {
		// variants of a mission for one side, for solo play or for benchmarks have no
		// names of their own: the name is of the mission they are made from
		key := m
		for {
			if t, ok := texts[key]; ok {
				n.Missions[m] = missionType.ReplaceAllString(t, "")
				break
			}
			base, cut := cutSuffix(key, variantSuffixes)
			if !cut {
				break
			}
			key = base
		}
	}
	for army, key := range armyKeys {
		if t, ok := texts[key]; ok {
			n.Armies[army] = t
		}
	}
	for key, t := range texts {
		switch {
		case strings.HasPrefix(key, "missionType/"):
			n.Modes[strings.TrimPrefix(key, "missionType/")] = t
		case strings.HasPrefix(key, "capzone/"):
			n.Zones[strings.TrimPrefix(key, "capzone/")] = t
		case strings.HasPrefix(key, "items/") && !strings.Contains(key[len("items/"):], "/"):
			n.Vehicles[strings.ToLower(strings.TrimPrefix(key, "items/"))] = t
		case strings.HasSuffix(key, "/full") && !strings.Contains(strings.TrimSuffix(key, "/full"), "/"):
			n.Campaigns[strings.TrimSuffix(key, "/full")] = t
		}
	}
	for name := range g.templates {
		if key := g.buildingKey(name); key != "" {
			if t, ok := texts[key]; ok {
				n.Structures[name] = t
			}
		}
	}
	return n
}
