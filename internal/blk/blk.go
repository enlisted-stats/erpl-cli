package blk

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// ErrFormat means the data is not a binary BLK or is damaged.
var ErrFormat = errors.New("blk: bad data")

// Block is a block of parameters and nested blocks, in file order.
type Block struct {
	Name   string
	Params []Param
	Blocks []*Block
}

// Param is a named value. Values are string, int32, int64, float32, bool,
// [2]float32, [3]float32, [4]float32, [2]int32, [3]int32, [4]int32,
// Color and TMatrix.
type Param struct {
	Name  string
	Value any
}

// Color is an E3DCOLOR value as stored: b, g, r, a.
type Color [4]uint8

// TMatrix is a 4x3 matrix: three axes and the position.
type TMatrix [4][3]float32

// Get returns the value of the first parameter with the name.
func (b *Block) Get(name string) (any, bool) {
	for _, p := range b.Params {
		if p.Name == name {
			return p.Value, true
		}
	}
	return nil, false
}

// String returns the first string parameter with the name, or "".
func (b *Block) String(name string) string {
	v, _ := b.Get(name)
	s, _ := v.(string)
	return s
}

// Block returns the first nested block with the name, or nil.
func (b *Block) Block(name string) *Block {
	for _, c := range b.Blocks {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Labels of binary BLK files.
const (
	labelFull          = 1
	labelShared        = 3
	labelSharedZstd    = 4
	labelSharedZstdDic = 5
)

// sharedNamesFile is the vromfs file with the names shared by its BLK files.
const sharedNamesFile = "\xff?nm"

// Decoder reads the BLK files of one vromfs archive: they share its name map and zstd dictionary.
type Decoder struct {
	names []string
	zstd  *zstd.Decoder
}

// NewDecoder prepares a decoder for the files of a vromfs archive.
func NewDecoder(files map[string][]byte) (*Decoder, error) {
	nm, ok := files[sharedNamesFile]
	if !ok {
		return nil, fmt.Errorf("%w: no shared name map", ErrFormat)
	}
	if len(nm) < 40 {
		return nil, fmt.Errorf("%w: shared name map of %d bytes", ErrFormat, len(nm))
	}
	opts := []zstd.DOption{zstd.WithDecoderConcurrency(1)}
	if dict, ok := files[fmt.Sprintf("%x.dict", nm[8:40])]; ok {
		opts = append(opts, zstd.WithDecoderDicts(dict))
	}
	z, err := zstd.NewReader(nil, opts...)
	if err != nil {
		return nil, err
	}
	raw, err := z.DecodeAll(nm[40:], nil)
	if err != nil {
		return nil, fmt.Errorf("%w: shared name map: %w", ErrFormat, err)
	}
	r := &reader{data: raw}
	names := r.names()
	if r.err != nil {
		return nil, fmt.Errorf("%w: shared name map: %w", ErrFormat, r.err)
	}
	return &Decoder{names: names, zstd: z}, nil
}

// Decode reads a binary BLK file and returns its root block.
func (d *Decoder) Decode(data []byte) (*Block, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty", ErrFormat)
	}
	body, shared := data[1:], d.names
	var err error
	switch data[0] {
	case labelFull:
		shared = nil
	case labelShared:
	case labelSharedZstd, labelSharedZstdDic:
		body, err = d.zstd.DecodeAll(body, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFormat, err)
		}
	default:
		return nil, fmt.Errorf("%w: label %d", ErrFormat, data[0])
	}
	r := &reader{data: body}
	root := r.dump(shared)
	if r.err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFormat, r.err)
	}
	return root, nil
}

// reader reads a BLK dump; the first error stops reading and stays in err.
type reader struct {
	data []byte
	pos  int
	err  error
}

func (r *reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = fmt.Errorf(format, args...)
	}
}

func (r *reader) bytes(n int) []byte {
	if r.err != nil || n < 0 || r.pos+n > len(r.data) {
		r.fail("%d bytes past the end at %d", n, r.pos)
		return nil
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

// vlq reads an unsigned number of 7-bit groups, the low group first.
func (r *reader) vlq() int {
	v := 0
	for i := range 5 {
		b := r.bytes(1)
		if b == nil {
			return 0
		}
		v |= int(b[0]&0x7F) << (i * 7)
		if b[0]&0x80 == 0 {
			break
		}
	}
	return v
}

// names reads a name table: the count, then the size and the zero-separated names.
func (r *reader) names() []string {
	n := r.vlq()
	if n == 0 {
		return nil
	}
	raw := r.bytes(r.vlq())
	names := strings.Split(string(raw), "\x00")
	if len(names) < n {
		r.fail("%d names, want %d", len(names), n)
		return nil
	}
	return names[:n]
}

// Parameter types and the sizes of their values.
const (
	typeString = 1 + iota
	typeInt
	typeReal
	typePoint2
	typePoint3
	typePoint4
	typeIPoint2
	typeIPoint3
	typeBool
	typeColor
	typeMatrix
	typeInt64
	typeIPoint4
)

var typeSizes = [...]int{0, 8, 4, 4, 8, 12, 16, 8, 12, 1, 4, 48, 8, 16}

func (r *reader) dump(shared []string) *Block {
	names := append(shared[:len(shared):len(shared)], r.names()...)
	nblocks, nparams, csize := r.vlq(), r.vlq(), r.vlq()
	complexData := r.bytes(csize)
	rawParams := r.bytes(nparams * 8)
	if r.err != nil {
		return nil
	}
	name := func(id int) string {
		if id < 0 || id >= len(names) {
			r.fail("name %d of %d", id, len(names))
			return ""
		}
		return names[id]
	}

	params := make([]Param, nparams)
	for i := range params {
		w := binary.LittleEndian.Uint32(rawParams[i*8:])
		v := binary.LittleEndian.Uint32(rawParams[i*8+4:])
		t := int(w >> 24)
		if t == 0 || t >= len(typeSizes) {
			r.fail("parameter type %d", t)
			return nil
		}
		raw := rawParams[i*8+4 : i*8+8]
		if typeSizes[t] > 4 && t != typeString {
			if int(v)+typeSizes[t] > len(complexData) {
				r.fail("parameter value at %d past the end", v)
				return nil
			}
			raw = complexData[v : int(v)+typeSizes[t]]
		}
		params[i] = Param{Name: name(int(w & 0xFFFFFF)), Value: value(t, v, raw, complexData, name)}
	}

	type span struct{ first, count int }
	blocks := make([]*Block, nblocks)
	children := make([]span, nblocks)
	used := 0
	for i := range blocks {
		nameID, np, nb := r.vlq(), r.vlq(), r.vlq()
		first := 0
		if nb > 0 {
			first = r.vlq()
		}
		if used+np > len(params) || first+nb > nblocks {
			r.fail("block %d out of range", i)
			return nil
		}
		b := &Block{Params: params[used : used+np : used+np]}
		if nameID > 0 {
			b.Name = name(nameID - 1)
		}
		used += np
		blocks[i], children[i] = b, span{first, nb}
	}
	if r.err != nil || nblocks == 0 {
		r.fail("no root block")
		return nil
	}
	for i, b := range blocks {
		b.Blocks = blocks[children[i].first : children[i].first+children[i].count]
	}
	return blocks[0]
}

func value(t int, v uint32, raw, complexData []byte, name func(int) string) any {
	le := binary.LittleEndian
	f := func(i int) float32 { return math.Float32frombits(le.Uint32(raw[i*4:])) }
	n := func(i int) int32 { return int32(le.Uint32(raw[i*4:])) }
	switch t {
	case typeString:
		if v&0x80000000 != 0 {
			return name(int(v & 0x7FFFFFFF))
		}
		if int(v) >= len(complexData) {
			return ""
		}
		s := complexData[v:]
		if end := bytes.IndexByte(s, 0); end >= 0 {
			s = s[:end]
		}
		return string(s)
	case typeInt:
		return n(0)
	case typeReal:
		return f(0)
	case typePoint2:
		return [2]float32{f(0), f(1)}
	case typePoint3:
		return [3]float32{f(0), f(1), f(2)}
	case typePoint4:
		return [4]float32{f(0), f(1), f(2), f(3)}
	case typeIPoint2:
		return [2]int32{n(0), n(1)}
	case typeIPoint3:
		return [3]int32{n(0), n(1), n(2)}
	case typeIPoint4:
		return [4]int32{n(0), n(1), n(2), n(3)}
	case typeBool:
		return raw[0] != 0
	case typeColor:
		return Color{raw[0], raw[1], raw[2], raw[3]}
	case typeMatrix:
		var m TMatrix
		for i := range 12 {
			m[i/3][i%3] = f(i)
		}
		return m
	case typeInt64:
		return int64(le.Uint64(raw))
	}
	return nil
}
