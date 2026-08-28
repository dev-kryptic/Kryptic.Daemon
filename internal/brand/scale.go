package brand

import (
	"bytes"
	"image"
	"image/png"
)

// ScaledLogoPNG returns LogoPNG resampled to size x size. The embedded asset
// is 1024x1024; nearest-neighbor is exact for the power-of-two sizes the
// About panel (128) and the Linux launcher icon (256) use.
func ScaledLogoPNG(size int) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(LogoPNG))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			out.Set(x, y, src.At(b.Min.X+x*b.Dx()/size, b.Min.Y+y*b.Dy()/size))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
