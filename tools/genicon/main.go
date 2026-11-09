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
)

func main() {
	out := flag.String("out", "build/icons", "output directory")
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
