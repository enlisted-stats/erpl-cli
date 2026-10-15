package report

import (
	"cmp"
	"embed"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/enlisted-stats/erpl"
	"github.com/enlisted-stats/erpl/battle"

	"github.com/enlisted-stats/erpl-cli/internal/maps"
	"github.com/enlisted-stats/erpl-cli/internal/names"
)

//go:embed assets
var assets embed.FS

var page = template.Must(template.ParseFS(assets, "assets/report.html"))

// Languages are the languages of the page; assets/strings.js has a dictionary for each.
var Languages = []string{"en", "ru"}

// scripts are the page scripts in the order they run.
var scripts = []string{"icons.js", "strings.js", "app.js"}

// Write renders a self-contained page about the battle in the language lang, e.g.
// "en": the data, the map image, the styles and the script are all inside. ix and
// mapsDir give the map; without it the page draws the battle on a grid. n gives the
// in-game names; without them the page shows ids.
func Write(w io.Writer, lang, file string, m erpl.Meta, b *battle.Battle, n names.Names, ix maps.Index, mapsDir string) error {
	mission := ix.Missions[m.Mission]
	img, err := mapImageOf(ix, mission, mapsDir)
	if err != nil {
		return err
	}
	js, err := json.Marshal(build(lang, file, m, b, n, mission, img))
	if err != nil {
		return err
	}
	style, err := assets.ReadFile("assets/style.css")
	if err != nil {
		return err
	}
	var script []byte
	for _, f := range scripts {
		s, err := assets.ReadFile("assets/" + f)
		if err != nil {
			return err
		}
		script = append(append(script, s...), '\n')
	}
	return page.Execute(w, struct {
		Title  string
		Style  template.CSS
		Script template.JS
		Data   template.JS
	}{cmp.Or(n.Missions[m.Mission], m.Mission), template.CSS(style), template.JS(script), template.JS(js)})
}

// mapImageOf reads the image of the mission's map, or returns nil if the map is not known.
func mapImageOf(ix maps.Index, mission maps.Mission, dir string) (*mapImage, error) {
	mp, ok := ix.Maps[mission.Map]
	if !ok || mp.Image == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(filepath.Join(dir, mp.Image))
	if err != nil {
		return nil, err
	}
	return &mapImage{
		Image:       "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(raw),
		LeftTop:     mp.LeftTop,
		RightBottom: mp.RightBottom,
	}, nil
}
