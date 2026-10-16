package report

import (
	"cmp"
	"embed"
	"encoding/json"
	"html/template"
	"io"

	"github.com/enlisted-stats/erpl"
	"github.com/enlisted-stats/erpl/battle"

	"github.com/enlisted-stats/erpl-cli/internal/gamedata"
)

//go:embed assets
var assets embed.FS

var page = template.Must(template.ParseFS(assets, "assets/report.html"))

// Languages are the languages of the page; assets/strings.js has a dictionary for each.
var Languages = []string{"en", "ru"}

// scripts are the page scripts in the order they run.
var scripts = []string{"icons.js", "glyphs.js", "strings.js", "app.js"}

// Write renders a self-contained page about the battle in the language lang, e.g.
// "en": the data, the map image, the styles and the script are all inside. gd gives
// the map and the in-game names. Without a map the page draws the battle on a grid,
// without a name it shows the id.
func Write(w io.Writer, lang, file string, m erpl.Meta, b *battle.Battle, gd *gamedata.Data) error {
	img, err := mapImageOf(gd, m.Mission, battleBox(b, gd.Missions[m.Mission].Areas))
	if err != nil {
		return err
	}
	js, err := json.Marshal(build(lang, file, m, b, gd, img))
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
	}{cmp.Or(gd.Missions[m.Mission].Name.In(lang), m.Mission), template.CSS(style), template.JS(script), template.JS(js)})
}
