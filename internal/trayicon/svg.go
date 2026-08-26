// Package trayicon rasterizes the Falcon SVGs for the Linux and Windows
// system trays. Those trays only accept PNG or ICO pixmaps, so this is the
// conversion step; the source assets stay SVG.
package trayicon

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"regexp"
	"strconv"
	"strings"
)

const superSample = 4

var classFillRe = regexp.MustCompile(`\.([A-Za-z0-9_-]+)\s*\{\s*fill:\s*([^;}]+)`)

// RasterPNG draws svg into a size×size PNG (transparent background, filled
// paths). size is the output edge in pixels.
func RasterPNG(svg []byte, size int) ([]byte, error) {
	if size < 1 {
		return nil, fmt.Errorf("tray icon size %d", size)
	}
	vb, shapes, err := parseSVG(svg)
	if err != nil {
		return nil, err
	}
	big := size * superSample
	img := image.NewRGBA(image.Rect(0, 0, big, big))
	sx := float64(big) / vb.w
	sy := float64(big) / vb.h
	for _, shape := range shapes {
		var contours [][]point
		for _, c := range shape.contours {
			scaled := make([]point, len(c))
			for i, p := range c {
				scaled[i] = point{p.x * sx, p.y * sy}
			}
			contours = append(contours, scaled)
		}
		fillEvenOdd(img, contours, shape.fill)
	}
	out := downsample(img, size, superSample)
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type viewBox struct{ w, h float64 }

type shape struct {
	contours [][]point
	fill     color.RGBA
}

type svgXML struct {
	ViewBox string     `xml:"viewBox,attr"`
	Defs    svgDefs    `xml:"defs"`
	Groups  []svgGroup `xml:"g"`
	Paths   []svgPath  `xml:"path"`
}

type svgDefs struct {
	Style string `xml:"style"`
}

type svgGroup struct {
	Transform string     `xml:"transform,attr"`
	Style     string     `xml:"style,attr"`
	Groups    []svgGroup `xml:"g"`
	Paths     []svgPath  `xml:"path"`
}

type svgPath struct {
	D         string `xml:"d,attr"`
	Class     string `xml:"class,attr"`
	Fill      string `xml:"fill,attr"`
	Style     string `xml:"style,attr"`
	Transform string `xml:"transform,attr"`
}

func parseSVG(raw []byte) (viewBox, []shape, error) {
	var doc svgXML
	if err := xml.NewDecoder(bytes.NewReader(raw)).Decode(&doc); err != nil && err != io.EOF {
		return viewBox{}, nil, err
	}
	vb, err := parseViewBox(doc.ViewBox)
	if err != nil {
		return viewBox{}, nil, err
	}
	classes := classFills(doc.Defs.Style)
	var shapes []shape
	var walk func(svgGroup, point)
	walk = func(g svgGroup, origin point) {
		origin = origin.add(parseTranslate(g.Transform))
		for _, p := range g.Paths {
			sh, err := pathShape(p, origin.add(parseTranslate(p.Transform)), classes)
			if err != nil {
				continue
			}
			shapes = append(shapes, sh)
		}
		for _, child := range g.Groups {
			walk(child, origin)
		}
	}
	for _, g := range doc.Groups {
		walk(g, point{})
	}
	for _, p := range doc.Paths {
		sh, err := pathShape(p, parseTranslate(p.Transform), classes)
		if err != nil {
			return vb, nil, err
		}
		shapes = append(shapes, sh)
	}
	if len(shapes) == 0 {
		return vb, nil, fmt.Errorf("svg has no paths")
	}
	return vb, shapes, nil
}

func pathShape(p svgPath, origin point, classes map[string]color.RGBA) (shape, error) {
	contours, err := parsePath(p.D)
	if err != nil {
		return shape{}, err
	}
	if origin.x != 0 || origin.y != 0 {
		for i := range contours {
			for j := range contours[i] {
				contours[i][j] = contours[i][j].add(origin)
			}
		}
	}
	return shape{contours: contours, fill: pathFill(p, classes)}, nil
}

func parseViewBox(s string) (viewBox, error) {
	fields := strings.Fields(strings.ReplaceAll(s, ",", " "))
	if len(fields) != 4 {
		return viewBox{}, fmt.Errorf("viewBox %q", s)
	}
	w, err1 := strconv.ParseFloat(fields[2], 64)
	h, err2 := strconv.ParseFloat(fields[3], 64)
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return viewBox{}, fmt.Errorf("viewBox %q", s)
	}
	return viewBox{w: w, h: h}, nil
}

func parseTranslate(t string) point {
	t = strings.TrimSpace(t)
	if t == "" || !strings.HasPrefix(t, "translate(") {
		return point{}
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(t, "translate("), ")")
	inner = strings.ReplaceAll(inner, ",", " ")
	fields := strings.Fields(inner)
	if len(fields) == 0 {
		return point{}
	}
	x, _ := strconv.ParseFloat(fields[0], 64)
	var y float64
	if len(fields) > 1 {
		y, _ = strconv.ParseFloat(fields[1], 64)
	}
	return point{x, y}
}

func classFills(style string) map[string]color.RGBA {
	out := map[string]color.RGBA{}
	for _, m := range classFillRe.FindAllStringSubmatch(style, -1) {
		if c, ok := parseHexColor(m[2]); ok {
			out[m[1]] = c
		}
	}
	return out
}

func pathFill(p svgPath, classes map[string]color.RGBA) color.RGBA {
	if c, ok := parseHexColor(p.Fill); ok {
		return c
	}
	if c, ok := parseHexColor(styleFill(p.Style)); ok {
		return c
	}
	for _, name := range strings.Fields(p.Class) {
		if c, ok := classes[name]; ok {
			return c
		}
	}
	return color.RGBA{A: 255}
}

func styleFill(style string) string {
	for _, part := range strings.Split(style, ";") {
		k, v, ok := strings.Cut(part, ":")
		if ok && strings.TrimSpace(k) == "fill" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseHexColor(s string) (color.RGBA, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if s == "" || strings.EqualFold(s, "none") {
		return color.RGBA{}, false
	}
	switch len(s) {
	case 3:
		r := hexNibble(s[0]) * 17
		g := hexNibble(s[1]) * 17
		b := hexNibble(s[2]) * 17
		return color.RGBA{r, g, b, 255}, true
	case 6:
		r, ok1 := hexByte(s[0:2])
		g, ok2 := hexByte(s[2:4])
		b, ok3 := hexByte(s[4:6])
		if !ok1 || !ok2 || !ok3 {
			return color.RGBA{}, false
		}
		return color.RGBA{r, g, b, 255}, true
	default:
		return color.RGBA{}, false
	}
}

func hexNibble(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

func hexByte(s string) (uint8, bool) {
	n, err := strconv.ParseUint(s, 16, 8)
	return uint8(n), err == nil
}

func downsample(src *image.RGBA, size, factor int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	area := uint32(factor * factor)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a uint32
			for dy := 0; dy < factor; dy++ {
				for dx := 0; dx < factor; dx++ {
					p := src.RGBAAt(x*factor+dx, y*factor+dy)
					r += uint32(p.R)
					g += uint32(p.G)
					b += uint32(p.B)
					a += uint32(p.A)
				}
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r / area),
				G: uint8(g / area),
				B: uint8(b / area),
				A: uint8(a / area),
			})
		}
	}
	return dst
}

func fillEvenOdd(img *image.RGBA, contours [][]point, fill color.RGBA) {
	b := img.Bounds()
	minY, maxY := b.Max.Y, b.Min.Y
	var edges [][2]point
	for _, c := range contours {
		if len(c) < 3 {
			continue
		}
		for i := 0; i < len(c); i++ {
			p, q := c[i], c[(i+1)%len(c)]
			if p.y == q.y {
				continue
			}
			edges = append(edges, [2]point{p, q})
			if int(p.y) < minY {
				minY = int(p.y)
			}
			if int(q.y) < minY {
				minY = int(q.y)
			}
			if int(p.y) > maxY {
				maxY = int(p.y)
			}
			if int(q.y) > maxY {
				maxY = int(q.y)
			}
		}
	}
	if minY < b.Min.Y {
		minY = b.Min.Y
	}
	if maxY > b.Max.Y {
		maxY = b.Max.Y
	}
	xs := make([]float64, 0, 16)
	for y := minY; y < maxY; y++ {
		mid := float64(y) + 0.5
		xs = xs[:0]
		for _, e := range edges {
			p, q := e[0], e[1]
			if (p.y <= mid && q.y > mid) || (q.y <= mid && p.y > mid) {
				t := (mid - p.y) / (q.y - p.y)
				xs = append(xs, p.x+t*(q.x-p.x))
			}
		}
		if len(xs) < 2 {
			continue
		}
		sortFloats(xs)
		for i := 0; i+1 < len(xs); i += 2 {
			x0 := int(xs[i] + 0.5)
			x1 := int(xs[i+1] + 0.5)
			if x0 < b.Min.X {
				x0 = b.Min.X
			}
			if x1 > b.Max.X {
				x1 = b.Max.X
			}
			for x := x0; x < x1; x++ {
				img.SetRGBA(x, y, fill)
			}
		}
	}
}

func sortFloats(a []float64) {
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i
		for j > 0 && a[j-1] > v {
			a[j] = a[j-1]
			j--
		}
		a[j] = v
	}
}
