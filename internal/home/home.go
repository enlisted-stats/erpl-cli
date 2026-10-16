package home

import (
	"os"
	"path/filepath"
)

// Dir is the program folder that holds the replays and reports folders.
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

func (d Dir) Reports() string { return filepath.Join(string(d), "reports") }

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
