package report

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"math"
	"slices"

	"github.com/enlisted-stats/erpl/battle"

	"github.com/enlisted-stats/erpl-cli/internal/gamedata"
)

// box is a part of the world: x grows to the right, z to the top.
type box struct{ l, r, b, t float64 }

// battleBox is where the battle went on: the battle areas of the ground, where most
// soldiers were and the zones, with a margin around. Soldiers stay in their vehicles
// and pilots fly far away, so the extremes of the soldiers are cut off.
func battleBox(b *battle.Battle, areas []gamedata.Area) box {
	var xs, zs []float64
	for _, s := range b.Soldiers {
		for _, p := range s.Track {
			xs = append(xs, float64(p.Pos[0]))
			zs = append(zs, float64(p.Pos[2]))
		}
	}
	if len(xs) == 0 {
		return box{}
	}
	slices.Sort(xs)
	slices.Sort(zs)
	q := func(a []float64, f float64) float64 { return a[int(f*float64(len(a)-1))] }
	bx := box{q(xs, 0.02), q(xs, 0.98), q(zs, 0.02), q(zs, 0.98)}
	add := func(x, y float64) { bx = box{min(bx.l, x), max(bx.r, x), min(bx.b, y), max(bx.t, y)} }
	for _, z := range b.Zones {
		add(float64(z.Pos[0]), float64(z.Pos[2]))
	}
	for _, a := range areas {
		if !a.Air {
			for _, p := range a.Points {
				add(float64(p[0]), float64(p[1]))
			}
		}
	}
	pad := max(100, 0.1*max(bx.r-bx.l, bx.t-bx.b))
	return box{bx.l - pad, bx.r + pad, bx.b - pad, bx.t + pad}
}

// mapImageOf cuts the part around the battle out of the mission's minimap, so the
// page stays small without losing detail. It is nil when the game has no minimap.
func mapImageOf(gd *gamedata.Data, mission string, around box) (*mapImage, error) {
	mp := gd.Maps[gd.Missions[mission].Map]
	raw, err := gd.Image(mp)
	if raw == nil || err != nil {
		return nil, err
	}
	l, t := float64(mp.LeftTop[0]), float64(mp.LeftTop[1])
	r, bt := float64(mp.RightBottom[0]), float64(mp.RightBottom[1])
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	size := img.Bounds().Size()
	px := func(x float64) int { return int(math.Round((x - l) / (r - l) * float64(size.X))) }
	py := func(z float64) int { return int(math.Round((z - t) / (bt - t) * float64(size.Y))) }
	cut := image.Rect(px(around.l), py(around.t), px(around.r), py(around.b)).Intersect(img.Bounds())
	if around == (box{}) || cut.Empty() {
		cut = img.Bounds()
	}
	if cut != img.Bounds() {
		var buf bytes.Buffer
		sub := img.(interface {
			SubImage(image.Rectangle) image.Image
		}).SubImage(cut)
		if err := jpeg.Encode(&buf, sub, &jpeg.Options{Quality: 88}); err != nil {
			return nil, err
		}
		raw = buf.Bytes()
	}
	wx := func(p int) float32 { return float32(l + float64(p)/float64(size.X)*(r-l)) }
	wz := func(p int) float32 { return float32(t + float64(p)/float64(size.Y)*(bt-t)) }
	return &mapImage{
		Image:       "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(raw),
		LeftTop:     [2]float32{wx(cut.Min.X), wz(cut.Min.Y)},
		RightBottom: [2]float32{wx(cut.Max.X), wz(cut.Max.Y)},
	}, nil
}
