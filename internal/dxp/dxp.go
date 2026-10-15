package dxp

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// ErrFormat means the data is not a texture pack or is damaged.
var ErrFormat = errors.New("dxp: bad texture pack")

// Header is the header of a DDSx texture.
type Header struct {
	Format     string // e.g. "DXT1"
	Flags      uint32
	W, H       int
	Levels     int // mip levels
	DxtShift   int // log2 of the bytes per 4x4 block; zero for formats without blocks
	MemSize    int // unpacked size of all levels
	PackedSize int
}

const (
	flagRevMipOrder = 0x00040000 // the smallest level comes first
	flagNeedBase    = 0x00020000 // only the high-quality levels; the rest is in another texture
	flagComprMask   = 0xE0000000
	flagZstd        = 0x20000000
	flagZlib        = 0x80000000
)

// Texture is a texture in a pack.
type Texture struct {
	Name string // without the parameters after "*", e.g. "manchuria_det_minimap"
	Header
	offset int64
}

// Pack is an open texture pack (.dxp.bin).
type Pack struct {
	f        *os.File
	Textures []Texture
}

const (
	headerSize    = 16
	ddsxSize      = 32
	maxIndexBytes = 64 << 20
)

// Open opens a pack and reads its texture list; the textures are read when decoded.
func Open(path string) (*Pack, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	ts, err := readIndex(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Pack{f: f, Textures: ts}, nil
}

func (p *Pack) Close() error { return p.f.Close() }

func readIndex(r io.ReaderAt) ([]Texture, error) {
	var h [headerSize]byte
	if _, err := r.ReadAt(h[:], 0); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFormat, err)
	}
	le := binary.LittleEndian
	version := le.Uint32(h[4:])
	if string(h[:4]) != "DxP2" || version != 2 && version != 3 {
		return nil, fmt.Errorf("%w: label %q, version %d", ErrFormat, h[:4], version)
	}
	size := le.Uint32(h[12:])
	if size > maxIndexBytes {
		return nil, fmt.Errorf("%w: index of %d bytes", ErrFormat, size)
	}
	d := make([]byte, size)
	if _, err := r.ReadAt(d, headerSize); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFormat, err)
	}

	// The index starts with three tables, each an offset into the index and a count:
	// names, DDSx headers and records of where the data is.
	table := func(i, itemSize int) (int, int, error) {
		ofs, n := int(le.Uint32(d[i*16:])), int(le.Uint32(d[i*16+4:]))
		if len(d) < 48 || ofs < 0 || n < 0 || ofs+n*itemSize > len(d) {
			return 0, 0, fmt.Errorf("%w: table %d", ErrFormat, i)
		}
		return ofs, n, nil
	}
	recSize := 24
	if version == 3 {
		recSize = 12
	}
	namesOfs, n, err := table(0, 8)
	if err != nil {
		return nil, err
	}
	hdrOfs, nh, err := table(1, ddsxSize)
	if err != nil {
		return nil, err
	}
	recOfs, nr, err := table(2, recSize)
	if err != nil {
		return nil, err
	}
	if nh != n || nr != n {
		return nil, fmt.Errorf("%w: %d names, %d headers, %d records", ErrFormat, n, nh, nr)
	}

	ts := make([]Texture, n)
	for i := range ts {
		nameOfs := int(le.Uint64(d[namesOfs+i*8:]))
		end := -1
		if nameOfs < len(d) {
			end = bytes.IndexByte(d[nameOfs:], 0)
		}
		if end < 0 {
			return nil, fmt.Errorf("%w: name %d", ErrFormat, i)
		}
		name, _, _ := strings.Cut(string(d[nameOfs:nameOfs+end]), "*")

		hd := d[hdrOfs+i*ddsxSize:]
		if string(hd[:4]) != "DDSx" {
			return nil, fmt.Errorf("%w: texture %s has no DDSx header", ErrFormat, name)
		}
		rec := d[recOfs+i*recSize:]
		if version == 2 {
			rec = rec[8:] // a 64-bit texture id comes first
		}
		ts[i] = Texture{
			Name: name,
			Header: Header{
				Format:     strings.TrimRight(string(hd[4:8]), "\x00"),
				Flags:      le.Uint32(hd[8:]),
				W:          int(le.Uint16(hd[12:])),
				H:          int(le.Uint16(hd[14:])),
				Levels:     int(hd[16]),
				DxtShift:   int(hd[23] & 0x0F),
				MemSize:    int(le.Uint32(hd[24:])),
				PackedSize: int(le.Uint32(hd[28:])),
			},
			offset: int64(le.Uint32(rec[4:])),
		}
	}
	return ts, nil
}

// Find returns the texture with the name, or nil.
func (p *Pack) Find(name string) *Texture {
	for i := range p.Textures {
		if p.Textures[i].Name == name {
			return &p.Textures[i]
		}
	}
	return nil
}

// Decode reads the full-size level of a texture.
func (p *Pack) Decode(t *Texture) (image.Image, error) {
	if t.Flags&flagNeedBase != 0 {
		return nil, fmt.Errorf("dxp: %s holds only high-quality levels", t.Name)
	}
	packed := make([]byte, t.PackedSize)
	if _, err := p.f.ReadAt(packed, t.offset); err != nil {
		return nil, fmt.Errorf("dxp: %s: %w", t.Name, err)
	}
	var data []byte
	var err error
	switch t.Flags & flagComprMask {
	case 0:
		data = packed
	case flagZstd:
		data, err = unzstd(packed, t.MemSize)
	case flagZlib:
		data, err = unzlib(packed, t.MemSize)
	default:
		err = fmt.Errorf("compression %#x is not supported", t.Flags&flagComprMask)
	}
	if err != nil {
		return nil, fmt.Errorf("dxp: %s: %w", t.Name, err)
	}

	top := max(t.W/4, 1) * max(t.H/4, 1) << t.DxtShift
	if t.DxtShift == 0 || len(data) < top {
		return nil, fmt.Errorf("dxp: %s: %d bytes for a %dx%d %s level", t.Name, len(data), t.W, t.H, t.Format)
	}
	if t.Flags&flagRevMipOrder != 0 {
		data = data[len(data)-top:]
	} else {
		data = data[:top]
	}
	switch t.Format {
	case "DXT1":
		return decodeDXT1(data, t.W, t.H), nil
	case "DXT5":
		return decodeDXT5(data, t.W, t.H), nil
	}
	return nil, fmt.Errorf("dxp: %s: format %s is not supported", t.Name, t.Format)
}

// unzstd unpacks the first zstd frame; the data may be followed by padding.
func unzstd(b []byte, size int) ([]byte, error) {
	d, err := zstd.NewReader(bytes.NewReader(b), zstd.WithDecoderConcurrency(1))
	if err != nil {
		return nil, err
	}
	defer d.Close()
	out := make([]byte, size)
	_, err = io.ReadFull(d, out)
	return out, err
}

func unzlib(b []byte, size int) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	out := make([]byte, size)
	_, err = io.ReadFull(r, out)
	return out, err
}
