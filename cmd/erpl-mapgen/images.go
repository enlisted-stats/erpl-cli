package main

import (
	"errors"
	"fmt"
	"image/jpeg"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enlisted-stats/erpl-cli/internal/dxp"
)

// packDirs hold the texture packs, relative to the game folder. The high-quality
// packs have bigger versions of some textures, named with the hqSuffix.
var packDirs = []string{"content", "content.hq"}

const hqSuffix = "$hq"

const imageQuality = 88

// source is where a texture is taken from: a pack and the name of the texture in it.
type source struct {
	pack, texture string
	width         int
	modified      time.Time
}

// textureSources finds each texture among the packs of the game. The biggest version
// is taken, and of equal ones the most recently modified pack: an update may bring a
// new version of a texture in another pack and leave the old one.
func textureSources(gameDir string, names []string) (map[string]source, error) {
	want := map[string]string{} // texture in a pack → texture asked for
	for _, n := range names {
		want[n], want[n+hqSuffix] = n, n
	}
	var paths []string
	for _, dir := range packDirs {
		err := filepath.WalkDir(filepath.Join(gameDir, dir), func(p string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) && p == filepath.Join(gameDir, dir) {
				return fs.SkipDir
			}
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".dxp.bin") {
				paths = append(paths, p)
			}
			return err
		})
		if err != nil {
			return nil, err
		}
	}

	found := map[string]source{}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		pk, err := dxp.Open(p)
		if err != nil {
			return nil, err
		}
		for _, t := range pk.Textures {
			n, ok := want[t.Name]
			if !ok || t.W == 0 {
				continue
			}
			c := source{p, t.Name, t.W, info.ModTime()}
			if old, ok := found[n]; !ok || c.width > old.width || c.width == old.width && c.modified.After(old.modified) {
				found[n] = c
			}
		}
		pk.Close()
	}
	return found, nil
}

// saveImage decodes a texture of a pack and writes it as a JPEG file.
func saveImage(pack, texture, path string) error {
	pk, err := dxp.Open(pack)
	if err != nil {
		return err
	}
	defer pk.Close()
	t := pk.Find(texture)
	if t == nil {
		return fmt.Errorf("%s: no texture %s", pack, texture)
	}
	img, err := pk.Decode(t)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: imageQuality}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
