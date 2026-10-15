package vromfs

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

// ErrFormat means the data is not a vromfs archive or is damaged.
var ErrFormat = errors.New("vromfs: bad archive")

const (
	headerSize     = 16
	packedSizeMask = 0x3FFFFFFF
	flagZstd       = 0x40000000 // packed with zstd and obfuscated; zlib otherwise
)

// xorKey obfuscates the first and the last 16 bytes of a zstd-packed body.
var xorKey = [4]uint32{0xAA55AA55, 0xF00FF00F, 0xAA55AA55, 0x12481248}

// Open reads the archive at path and returns its files by name.
func Open(path string) (map[string][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	files, err := Read(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return files, nil
}

// Read unpacks an archive and returns its files by name. The files are slices of one buffer.
func Read(data []byte) (map[string][]byte, error) {
	if len(data) < headerSize {
		return nil, fmt.Errorf("%w: %d bytes", ErrFormat, len(data))
	}
	label := string(data[:4])
	size := binary.LittleEndian.Uint32(data[8:])
	packing := binary.LittleEndian.Uint32(data[12:])
	off := headerSize
	switch label {
	case "VRFs":
	case "VRFx":
		if len(data) < off+2 {
			return nil, fmt.Errorf("%w: no extended header", ErrFormat)
		}
		off += int(binary.LittleEndian.Uint16(data[off:]))
	default:
		return nil, fmt.Errorf("%w: label %q", ErrFormat, label)
	}

	packed := int(packing & packedSizeMask)
	if packed == 0 {
		packed = int(size)
	}
	if off+packed > len(data) {
		return nil, fmt.Errorf("%w: body of %d bytes past the end", ErrFormat, packed)
	}
	body := data[off : off+packed]
	var err error
	switch {
	case packing&packedSizeMask == 0:
	case packing&flagZstd != 0:
		body, err = unzstd(deobfuscate(body), int(size))
	default:
		body, err = unzlib(body, int(size))
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFormat, err)
	}
	if len(body) != int(size) {
		return nil, fmt.Errorf("%w: body is %d bytes, want %d", ErrFormat, len(body), size)
	}
	return files(body)
}

func deobfuscate(b []byte) []byte {
	b = bytes.Clone(b)
	xor := func(at int, key [4]uint32) {
		for i, k := range key {
			p := b[at+i*4:]
			binary.LittleEndian.PutUint32(p, binary.LittleEndian.Uint32(p)^k)
		}
	}
	if len(b) >= 16 {
		xor(0, xorKey)
	}
	if len(b) >= 32 {
		xor((len(b)/4-4)*4, [4]uint32{xorKey[3], xorKey[2], xorKey[1], xorKey[0]})
	}
	return b
}

func unzstd(b []byte, size int) ([]byte, error) {
	d, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(uint64(size)+1))
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.DecodeAll(b, make([]byte, 0, size))
}

func unzlib(b []byte, size int) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	out := bytes.NewBuffer(make([]byte, 0, size))
	_, err = io.Copy(out, r)
	return out.Bytes(), err
}

// files reads the name table and the data table of an unpacked body.
func files(body []byte) (map[string][]byte, error) {
	u32 := func(at int) (int, bool) {
		if at < 0 || at+4 > len(body) {
			return 0, false
		}
		return int(binary.LittleEndian.Uint32(body[at:])), true
	}
	namesOfs, ok1 := u32(0)
	count, ok2 := u32(4)
	dataOfs, ok3 := u32(16)
	if !ok1 || !ok2 || !ok3 || namesOfs+count*8 > len(body) || dataOfs+count*16 > len(body) {
		return nil, fmt.Errorf("%w: file tables", ErrFormat)
	}
	fs := make(map[string][]byte, count)
	for i := range count {
		nameOfs := int(binary.LittleEndian.Uint64(body[namesOfs+i*8:]))
		end := -1
		if nameOfs < len(body) {
			end = bytes.IndexByte(body[nameOfs:], 0)
		}
		ofs, _ := u32(dataOfs + i*16)
		size, _ := u32(dataOfs + i*16 + 4)
		if end < 0 || ofs+size > len(body) {
			return nil, fmt.Errorf("%w: file %d", ErrFormat, i)
		}
		fs[string(body[nameOfs:nameOfs+end])] = body[ofs : ofs+size]
	}
	return fs, nil
}
