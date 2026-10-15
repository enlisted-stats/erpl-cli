package dxp

import (
	"encoding/binary"
	"image"
	"image/color"
)

// decodeDXT1 decodes BC1: 8-byte blocks of two RGB565 colors and 2-bit indices.
func decodeDXT1(data []byte, w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	blocks(w, h, 8, func(i, x, y int) {
		b := data[i:]
		pal := palette(b, true)
		idx := binary.LittleEndian.Uint32(b[4:])
		for p := range 16 {
			set(img, x+p%4, y+p/4, pal[idx>>(2*p)&3])
		}
	})
	return img
}

// decodeDXT5 decodes BC3: an 8-byte interpolated alpha block, then a BC1 color block.
func decodeDXT5(data []byte, w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	blocks(w, h, 16, func(i, x, y int) {
		b := data[i:]
		a0, a1 := b[0], b[1]
		alpha := [8]uint8{a0, a1}
		for k := 1; k < 7; k++ {
			if a0 > a1 {
				alpha[k+1] = uint8((int(a0)*(7-k) + int(a1)*k) / 7)
			} else if k < 5 {
				alpha[k+1] = uint8((int(a0)*(5-k) + int(a1)*k) / 5)
			} else {
				alpha[k+1] = []uint8{0, 255}[k-5]
			}
		}
		bits := uint64(b[2]) | uint64(b[3])<<8 | uint64(b[4])<<16 | uint64(b[5])<<24 | uint64(b[6])<<32 | uint64(b[7])<<40
		pal := palette(b[8:], false)
		idx := binary.LittleEndian.Uint32(b[12:])
		for p := range 16 {
			c := pal[idx>>(2*p)&3]
			c.A = alpha[bits>>(3*p)&7]
			set(img, x+p%4, y+p/4, c)
		}
	})
	return img
}

// blocks calls f for every 4x4 block with its offset in the data and its top-left pixel.
func blocks(w, h, size int, f func(i, x, y int)) {
	i := 0
	for y := 0; y < h; y += 4 {
		for x := 0; x < w; x += 4 {
			f(i, x, y)
			i += size
		}
	}
}

// palette returns the four colors of a BC1 color block. With transparency, a block
// whose first color is not greater than the second has black transparent as the fourth.
func palette(b []byte, transparency bool) [4]color.NRGBA {
	c0, c1 := binary.LittleEndian.Uint16(b), binary.LittleEndian.Uint16(b[2:])
	p0, p1 := rgb565(c0), rgb565(c1)
	mix := func(a, b uint8, wa, wb, d int) uint8 { return uint8((int(a)*wa + int(b)*wb) / d) }
	var p [4]color.NRGBA
	p[0], p[1] = p0, p1
	if c0 > c1 || !transparency {
		p[2] = color.NRGBA{mix(p0.R, p1.R, 2, 1, 3), mix(p0.G, p1.G, 2, 1, 3), mix(p0.B, p1.B, 2, 1, 3), 255}
		p[3] = color.NRGBA{mix(p0.R, p1.R, 1, 2, 3), mix(p0.G, p1.G, 1, 2, 3), mix(p0.B, p1.B, 1, 2, 3), 255}
	} else {
		p[2] = color.NRGBA{mix(p0.R, p1.R, 1, 1, 2), mix(p0.G, p1.G, 1, 1, 2), mix(p0.B, p1.B, 1, 1, 2), 255}
		p[3] = color.NRGBA{}
	}
	return p
}

func rgb565(c uint16) color.NRGBA {
	r, g, b := uint8(c>>11&31), uint8(c>>5&63), uint8(c&31)
	return color.NRGBA{r<<3 | r>>2, g<<2 | g>>4, b<<3 | b>>2, 255}
}

func set(img *image.NRGBA, x, y int, c color.NRGBA) {
	if x < img.Rect.Max.X && y < img.Rect.Max.Y {
		img.SetNRGBA(x, y, c)
	}
}
