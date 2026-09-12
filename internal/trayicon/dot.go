package trayicon

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

// WithStatusDot paints a filled status circle in the bottom-right of png,
// with a contrasting ring so it stays readable on light and dark falcons.
func WithStatusDot(src []byte, r, g, b uint8) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)

	w := bounds.Dx()
	diameter := w / 4
	if diameter < 6 {
		diameter = 6
	}
	pad := w / 16
	if pad < 1 {
		pad = 1
	}
	cx := bounds.Max.X - pad - diameter/2
	cy := bounds.Max.Y - pad - diameter/2
	fillCircle(dst, cx, cy, diameter/2+int(math.Max(1, float64(diameter)/10)), color.RGBA{255, 255, 255, 255})
	fillCircle(dst, cx, cy, diameter/2, color.RGBA{r, g, b, 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fillCircle(img *image.RGBA, cx, cy, radius int, fill color.RGBA) {
	if radius < 1 {
		return
	}
	r2 := radius * radius
	b := img.Bounds()
	for y := cy - radius; y <= cy+radius; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := cx - radius; x <= cx+radius; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r2 {
				img.SetRGBA(x, y, fill)
			}
		}
	}
}
