// genicon draws the wslpp-gui icon: a rounded gradient tile with two
// opposite arrows, standing for ports forwarded between WSL and the LAN.
// It writes icon.png, icon.icns (for the macOS .app) and icon.ico using
// only the Go standard library, so CI needs no image tools.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
)

func main() {
	out := flag.String("out", "build/icons", "output directory")
	sysoDir := flag.String("syso-dir", "", "write rsrc_windows_<arch>.syso here (for the Go linker)")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}

	sizes := []int{16, 32, 64, 128, 256, 512}
	pngs := make(map[int][]byte, len(sizes))
	for _, s := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, drawIcon(s)); err != nil {
			panic(err)
		}
		pngs[s] = buf.Bytes()
	}

	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(*out, name), data, 0o644); err != nil {
			panic(err)
		}
	}
	write("icon.png", pngs[512])
	write("icon.icns", makeICNS(pngs))
	write("icon.ico", makeICO(pngs, sizes))

	if *sysoDir != "" {
		if err := os.MkdirAll(*sysoDir, 0o755); err != nil {
			panic(err)
		}
		writeSyso := func(name string, data []byte) {
			if err := os.WriteFile(filepath.Join(*sysoDir, name), data, 0o644); err != nil {
				panic(err)
			}
		}
		writeSyso("rsrc_windows_amd64.syso", makeSyso(pngs, sizes, "amd64"))
		writeSyso("rsrc_windows_arm64.syso", makeSyso(pngs, sizes, "arm64"))
	}
}

// drawIcon renders one square icon with 2x2 supersampled edges.
func drawIcon(n int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			bgCov, fgCov := 0.0, 0.0
			for sy := 0; sy < 2; sy++ {
				for sx := 0; sx < 2; sx++ {
					fx := (float64(x) + (float64(sx)+0.5)/2) / float64(n)
					fy := (float64(y) + (float64(sy)+0.5)/2) / float64(n)
					if !roundRect(fx, fy, 0.03, 0.03, 0.97, 0.97, 0.22) {
						continue
					}
					bgCov += 0.25
					if symbol(fx, fy) {
						fgCov += 0.25
					}
				}
			}
			if bgCov == 0 {
				continue
			}
			col := gradient((float64(x)+0.5)/float64(n), (float64(y)+0.5)/float64(n))
			if fgCov > 0 {
				t := fgCov / bgCov
				col.R = uint8(float64(col.R)*(1-t) + 255*t)
				col.G = uint8(float64(col.G)*(1-t) + 255*t)
				col.B = uint8(float64(col.B)*(1-t) + 255*t)
			}
			col.A = uint8(255 * bgCov)
			img.SetNRGBA(x, y, col)
		}
	}
	return img
}

func gradient(x, y float64) color.NRGBA {
	t := (x + y) / 2
	return color.NRGBA{
		R: uint8(lerp(0x14, 0x4A, t)),
		G: uint8(lerp(0x2E, 0x3C, t)),
		B: uint8(lerp(0x66, 0xB8, t)),
		A: 255,
	}
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func roundRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Min(math.Max(x, x0+r), x1-r)
	cy := math.Min(math.Max(y, y0+r), y1-r)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

func inRect(x, y, x0, y0, x1, y1 float64) bool {
	return x >= x0 && x <= x1 && y >= y0 && y <= y1
}

// symbol draws two horizontal arrows: the upper one points right (WSL ports
// out to the LAN), the lower one points left (traffic coming back).
func symbol(x, y float64) bool {
	upperShaft := inRect(x, y, 0.22, 0.36, 0.66, 0.44)
	upperHead := x >= 0.64 && x <= 0.80 && math.Abs(y-0.40) <= (0.80-x)/0.16*0.12
	lowerShaft := inRect(x, y, 0.34, 0.56, 0.78, 0.64)
	lowerHead := x >= 0.20 && x <= 0.36 && math.Abs(y-0.60) <= (x-0.20)/0.16*0.12
	return upperShaft || upperHead || lowerShaft || lowerHead
}

// makeICNS wraps the PNGs in the icon types macOS expects.
func makeICNS(pngs map[int][]byte) []byte {
	entries := []struct {
		typ  string
		size int
	}{
		{"icp4", 16}, {"icp5", 32}, {"icp6", 64},
		{"ic07", 128}, {"ic08", 256}, {"ic09", 512},
	}
	var body bytes.Buffer
	for _, e := range entries {
		data := pngs[e.size]
		body.WriteString(e.typ)
		_ = binary.Write(&body, binary.BigEndian, uint32(8+len(data)))
		body.Write(data)
	}
	var out bytes.Buffer
	out.WriteString("icns")
	_ = binary.Write(&out, binary.BigEndian, uint32(8+body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// makeICO builds a PNG-compressed Windows icon. 256 is encoded as width 0.
func makeICO(pngs map[int][]byte, sizes []int) []byte {
	type entry struct {
		size int
		data []byte
	}
	var imgs []entry
	for _, s := range sizes {
		if s <= 256 {
			imgs = append(imgs, entry{s, pngs[s]})
		}
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(&out, binary.LittleEndian, uint16(1)) // type: icon
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(imgs)))
	offset := 6 + 16*len(imgs)
	for _, im := range imgs {
		w := byte(im.size)
		if im.size == 256 {
			w = 0
		}
		out.WriteByte(w)                                        // width
		out.WriteByte(w)                                        // height
		out.WriteByte(0)                                        // colors
		out.WriteByte(0)                                        // reserved
		_ = binary.Write(&out, binary.LittleEndian, uint16(1))  // planes
		_ = binary.Write(&out, binary.LittleEndian, uint16(32)) // bits per pixel
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(im.data)))
		_ = binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(im.data)
	}
	for _, im := range imgs {
		out.Write(im.data)
	}
	return out.Bytes()
}

// makeSyso builds a COFF object holding Windows icon resources (RT_ICON and
// RT_GROUP_ICON). Placed in the main package as rsrc_windows_<arch>.syso, the
// Go linker embeds it into the exe automatically — no rc/windres tool and no
// cgo. This is the same layout mygo's own builder uses.
func makeSyso(pngs map[int][]byte, sizes []int, arch string) []byte {
	machine, relocation, ok := map[string]struct{ m, r uint16 }{
		"amd64": {0x8664, 0x0003}, // IMAGE_REL_AMD64_ADDR32NB
		"arm64": {0xAA64, 0x0002}, // IMAGE_REL_ARM64_ADDR32NB
	}[arch]
	if !ok {
		panic("unsupported arch: " + arch)
	}

	const rtIcon, rtGroupIcon = 3, 14
	var iconSizes []int
	for _, s := range sizes {
		if s <= 256 {
			iconSizes = append(iconSizes, s)
		}
	}
	res := map[uint16]map[uint16][]byte{rtIcon: {}, rtGroupIcon: {}}

	var group bytes.Buffer
	_ = binary.Write(&group, binary.LittleEndian, [3]uint16{0, 1, uint16(len(iconSizes))})
	for i, s := range iconSizes {
		id := uint16(i + 1)
		img := pngs[s]
		res[rtIcon][id] = img
		dim := byte(s)
		if s >= 256 {
			dim = 0
		}
		group.Write([]byte{dim, dim, 0, 0})
		_ = binary.Write(&group, binary.LittleEndian, [2]uint16{1, 32})
		_ = binary.Write(&group, binary.LittleEndian, uint32(len(img)))
		_ = binary.Write(&group, binary.LittleEndian, id)
	}
	res[rtGroupIcon] = map[uint16][]byte{1: group.Bytes()}
	return buildCOFF(res, machine, relocation)
}

// buildCOFF lays out the three-level resource directory (type, id, language),
// the data entries and the raw resource bytes, plus the relocations the
// linker needs to turn data-entry RVAs into real addresses.
func buildCOFF(res map[uint16]map[uint16][]byte, machine, relocation uint16) []byte {
	types := make([]uint16, 0, len(res))
	for t := range res {
		types = append(types, t)
	}
	slices.Sort(types)

	dirSize := func(n int) int { return 16 + 8*n }
	size := dirSize(len(types))
	for _, t := range types {
		size += dirSize(len(res[t])) + len(res[t])*dirSize(1)
	}
	entriesAt := size
	count := 0
	for _, t := range types {
		count += len(res[t])
	}
	dataAt := entriesAt + 16*count

	raw := make([]byte, dataAt)
	le := binary.LittleEndian
	dir := func(at, n int) { le.PutUint16(raw[at+14:], uint16(n)) }
	var relocs []uint32
	var data bytes.Buffer
	dir(0, len(types))
	next := dirSize(len(types))
	entry := entriesAt
	for i, t := range types {
		le.PutUint32(raw[16+8*i:], uint32(t))
		le.PutUint32(raw[16+8*i+4:], uint32(next)|0x80000000)
		ids := make([]uint16, 0, len(res[t]))
		for id := range res[t] {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		idDir := next
		dir(idDir, len(ids))
		next += dirSize(len(ids))
		for j, id := range ids {
			le.PutUint32(raw[idDir+16+8*j:], uint32(id))
			le.PutUint32(raw[idDir+16+8*j+4:], uint32(next)|0x80000000)
			dir(next, 1)
			le.PutUint32(raw[next+16:], 1033) // language: en-US
			le.PutUint32(raw[next+20:], uint32(entry))
			next += dirSize(1)

			for data.Len()%8 != 0 {
				data.WriteByte(0)
			}
			le.PutUint32(raw[entry:], uint32(dataAt+data.Len())) // OffsetToData
			le.PutUint32(raw[entry+4:], uint32(len(res[t][id])))
			relocs = append(relocs, uint32(entry))
			entry += 16
			data.Write(res[t][id])
		}
	}
	raw = append(raw, data.Bytes()...)
	for len(raw)%4 != 0 {
		raw = append(raw, 0)
	}

	var out bytes.Buffer
	const headerSize, sectionHeaderSize = 20, 40
	rawAt := headerSize + sectionHeaderSize
	relocAt := rawAt + len(raw)
	symbolsAt := relocAt + 10*len(relocs)
	_ = binary.Write(&out, le, struct {
		Machine, Sections                   uint16
		Time, Symbols, SymbolCount          uint32
		OptionalHeaderSize, Characteristics uint16
	}{machine, 1, 0, uint32(symbolsAt), 1, 0, 0})
	_ = binary.Write(&out, le, struct {
		Name                                        [8]byte
		VirtualSize, VirtualAddress, RawSize, RawAt uint32
		RelocAt, LineNumbersAt                      uint32
		Relocs, LineNumbers                         uint16
		Characteristics                             uint32
	}{[8]byte{'.', 'r', 's', 'r', 'c'}, 0, 0, uint32(len(raw)), uint32(rawAt), uint32(relocAt), 0, uint16(len(relocs)), 0, 0x40000040})
	out.Write(raw)
	for _, r := range relocs {
		_ = binary.Write(&out, le, r)         // VirtualAddress
		_ = binary.Write(&out, le, uint32(0)) // symbol 0: the section
		_ = binary.Write(&out, le, relocation)
	}
	out.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	_ = binary.Write(&out, le, uint32(0)) // Value
	_ = binary.Write(&out, le, uint16(1)) // SectionNumber
	_ = binary.Write(&out, le, uint16(0)) // Type
	out.Write([]byte{3, 0})               // IMAGE_SYM_CLASS_STATIC
	_ = binary.Write(&out, le, uint32(4)) // empty string table
	return out.Bytes()
}
