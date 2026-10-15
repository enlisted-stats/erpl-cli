package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	stdmaps "maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/home"
	"github.com/enlisted-stats/erpl-cli/internal/maps"
	"github.com/enlisted-stats/erpl-cli/internal/report"
	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

// version is set by the release build, e.g. to "v1.0.0"; a build from source reports none.
var version string

func main() {
	if err := fang.Execute(context.Background(), command(), fang.WithVersion(version), fang.WithoutCompletions(), fang.WithoutManpage()); err != nil {
		os.Exit(1)
	}
}

func command() *cobra.Command {
	var gameDir string
	c := &cobra.Command{
		Use:   "erpl-mapgen --game folder",
		Short: "Extract the maps and names of Enlisted for erpl",
		Long: "Extract the maps and names of Enlisted for erpl.\n\n" +
			"Reads the installed game and writes two folders next to erpl. The maps folder gets the map images. The data folder " +
			"gets which map each mission is played on, where the map lies in the world, the battle areas, and the in-game names " +
			"of missions, objectives, vehicles and structures in the languages of the report. Run it again after a game update.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			g, err := openGame(gameDir)
			if err != nil {
				return err
			}
			ix := maps.Index{Maps: map[string]maps.Map{}, Missions: map[string]maps.Mission{}}
			skipped := 0
			for _, name := range g.missionNames() {
				mm, areas, err := g.mission(name)
				if err != nil {
					return err
				}
				if mm == "" {
					skipped++
					continue
				}
				m := g.minimaps[mm]
				ix.Maps[m.texture] = m.Map
				ix.Missions[name] = maps.Mission{Map: m.texture, Areas: areas}
			}

			d, err := home.Find()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(d.Maps(), 0o755); err != nil {
				return err
			}
			textures := slices.Sorted(stdmaps.Keys(ix.Maps))
			sources, err := textureSources(gameDir, textures)
			if err != nil {
				return err
			}
			var missing []string
			for _, t := range textures {
				src, ok := sources[t]
				if !ok {
					missing = append(missing, t)
					continue
				}
				m := ix.Maps[t]
				m.Image = t + ".jpg"
				if err := saveImage(src.pack, src.texture, filepath.Join(d.Maps(), m.Image)); err != nil {
					return err
				}
				ix.Maps[t] = m
			}
			if err := ix.Save(d.Data()); err != nil {
				return err
			}
			// the index lived next to the images before the data folder
			if err := os.Remove(filepath.Join(d.Maps(), maps.File)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}

			texts, err := translations(gameDir)
			if err != nil {
				return err
			}
			// names are needed only in the languages the report has
			missions := slices.Collect(stdmaps.Keys(ix.Missions))
			var langs []string
			for _, lang := range report.Languages {
				t, ok := texts[lang]
				if !ok {
					return fmt.Errorf("the game has no translations to %s", lang)
				}
				if err := g.gameNames(t, missions).Save(d.Data(), lang); err != nil {
					return err
				}
				langs = append(langs, lang)
			}
			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprintln(w, ui.Fields(
				[2]string{"Missions", fmt.Sprintf("%d %s", len(ix.Missions),
					ui.Muted.Render(fmt.Sprintf("(%d scenes without a minimap skipped)", skipped)))},
				[2]string{"Maps", mapsLine(len(ix.Maps), missing)},
				[2]string{"Languages", strings.Join(langs, ", ")},
				[2]string{"Written", d.Maps() + ", " + d.Data()},
			))
			return nil
		},
	}
	c.Flags().StringVarP(&gameDir, "game", "g", "", `folder of the installed game, e.g. F:\Games\Enlisted`)
	_ = c.MarkFlagRequired("game")
	return c
}

// mapsLine tells how many maps there are and which have no image found.
func mapsLine(n int, missing []string) string {
	if len(missing) == 0 {
		return fmt.Sprint(n)
	}
	return fmt.Sprintf("%d %s", n, ui.Bad.Render("no image for "+strings.Join(missing, ", ")))
}
