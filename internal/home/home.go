package home

import (
	"os"
	"path/filepath"
)

// Dir is the program folder that holds the replays, reports and maps folders.
type Dir string

// Find returns the folder of the executable, or the working directory if only it
// has a replays folder, as under go run.
func Find() (Dir, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	d := Dir(filepath.Dir(exe))
	if !isDir(d.Replays()) {
		if wd, err := os.Getwd(); err == nil && isDir(Dir(wd).Replays()) {
			d = Dir(wd)
		}
	}
	return d, nil
}

func (d Dir) Replays() string { return filepath.Join(string(d), "replays") }

func (d Dir) Maps() string { return filepath.Join(string(d), "maps") }

// Data is the folder of the JSON files erpl-mapgen makes: the map index and the names.
func (d Dir) Data() string { return filepath.Join(string(d), "data") }

func (d Dir) Reports() string { return filepath.Join(string(d), "reports") }

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
