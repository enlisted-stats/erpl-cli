package replays

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ErrEmpty means the folder has no replays.
var ErrEmpty = errors.New("no replays")

// Replay is a replay file in the replays folder.
type Replay struct {
	Index   int // from 1 for the oldest file
	Path    string
	Created time.Time
}

// Scan lists the replays of dir, oldest first.
func Scan(dir string) ([]Replay, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var rs []Replay
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".erpl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		rs = append(rs, Replay{Path: filepath.Join(dir, e.Name()), Created: created(info)})
	}
	slices.SortFunc(rs, func(a, b Replay) int {
		if c := a.Created.Compare(b.Created); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	for i := range rs {
		rs[i].Index = i + 1
	}
	return rs, nil
}

// Pick returns the replay with the index given in arg, or the latest replay if arg is empty.
func Pick(rs []Replay, arg string) (Replay, error) {
	if len(rs) == 0 {
		return Replay{}, ErrEmpty
	}
	if arg == "" {
		return rs[len(rs)-1], nil
	}
	i, err := strconv.Atoi(arg)
	if err != nil || i < 1 || i > len(rs) {
		return Replay{}, fmt.Errorf("no replay #%s: numbers go from 1 to %d", arg, len(rs))
	}
	return rs[i-1], nil
}
