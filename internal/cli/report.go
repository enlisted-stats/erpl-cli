package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enlisted-stats/erpl-cli/internal/gamedata"
	"github.com/enlisted-stats/erpl-cli/internal/home"
	"github.com/enlisted-stats/erpl-cli/internal/report"
	"github.com/enlisted-stats/erpl-cli/internal/ui"
)

func reportCommand() *cobra.Command {
	var noOpen bool
	lang := "en"
	c := &cobra.Command{
		Use:   "report [number]",
		Short: "Make an HTML page about a battle with an interactive map, and open it",
		Long: "Make an HTML page about a battle with an interactive map, and open it.\n\n" +
			"The page goes to the reports folder next to erpl. It is a single file that opens without the internet, " +
			"so it can be sent to others.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !slices.Contains(report.Languages, lang) {
				return fmt.Errorf("no language %q: languages are %s", lang, strings.Join(report.Languages, ", "))
			}
			rp, m, b, err := read(args)
			if err != nil {
				return err
			}
			d, err := home.Find()
			if err != nil {
				return err
			}
			gd, err := gamedata.Load()
			if err != nil {
				return err
			}

			if err := os.MkdirAll(d.Reports(), 0o755); err != nil {
				return err
			}
			name := strings.TrimSuffix(filepath.Base(rp.Path), filepath.Ext(rp.Path))
			if !m.StartTime.IsZero() {
				name = m.StartTime.Local().Format("2006-01-02_1504") + "_" + m.Mission
			}
			path := filepath.Join(d.Reports(), name+".html")
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			if err := report.Write(f, lang, filepath.Base(rp.Path), m, b, gd); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}

			w := ui.NewOut(cmd.OutOrStdout())
			fmt.Fprint(w, header(rp.Index, rp.Path))
			fmt.Fprintln(w, "Report: "+ui.Accent.Render(path))
			if gd.Maps[gd.Missions[m.Mission].Map].Image == "" {
				fmt.Fprintln(w, ui.Muted.Render("The game has no map image for "+m.Mission+": the page draws the battle on a grid."))
			}
			if !noOpen {
				return openFile(path)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&noOpen, "no-open", false, "only write the page, do not open it")
	c.Flags().StringVarP(&lang, "lang", "l", lang, "language of the page: "+strings.Join(report.Languages, ", "))
	return c
}

// openFile opens a file in the program the system uses for it, a browser for a page.
func openFile(path string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		c = exec.Command("open", path)
	default:
		c = exec.Command("xdg-open", path)
	}
	return c.Start()
}
