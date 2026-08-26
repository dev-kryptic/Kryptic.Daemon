package trayicon

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRasterFalconSVGs(t *testing.T) {
	white := readWorkspaceSVG(t, "Falcon.svg")
	black := readWorkspaceSVG(t, "Falcon-black.svg")

	whitePNG, err := RasterPNG(white, 128)
	if err != nil {
		t.Fatal(err)
	}
	blackPNG, err := RasterPNG(black, 128)
	if err != nil {
		t.Fatal(err)
	}

	wOpaque, wWhite, wBlack := pixelStats(t, whitePNG)
	bOpaque, bWhite, bBlack := pixelStats(t, blackPNG)
	if wOpaque < 2000 || bOpaque < 2000 {
		t.Fatalf("falcon too empty: white opaque=%d black opaque=%d", wOpaque, bOpaque)
	}
	if wWhite < wOpaque/2 {
		t.Fatalf("white falcon is not white: opaque=%d white=%d black=%d", wOpaque, wWhite, wBlack)
	}
	if bBlack < bOpaque/2 {
		t.Fatalf("black falcon is not black: opaque=%d white=%d black=%d", bOpaque, bWhite, bBlack)
	}
	delta := wOpaque - bOpaque
	if delta < 0 {
		delta = -delta
	}
	if delta > wOpaque/10 {
		t.Fatalf("silhouettes differ too much: white=%d black=%d", wOpaque, bOpaque)
	}
	if wOpaque < 5000 {
		t.Fatalf("falcon looks clipped: opaque=%d", wOpaque)
	}
}

func TestPNGToICO(t *testing.T) {
	png, err := RasterPNG(readWorkspaceSVG(t, "Falcon.svg"), 128)
	if err != nil {
		t.Fatal(err)
	}
	ico := PNGToICO(png, 128)
	if len(ico) != 22+len(png) {
		t.Fatalf("ico len %d", len(ico))
	}
	if !bytes.Equal(ico[22:30], []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("ico should contain a PNG")
	}
}

func TestParsePathSquare(t *testing.T) {
	contours, err := parsePath("M0,0 L10,0 L10,10 L0,10 Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(contours) != 1 || len(contours[0]) < 4 {
		t.Fatalf("%v", contours)
	}
}

func readWorkspaceSVG(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "cmd", "kryptic-tray", "assets", name),
		filepath.Join("..", "..", "..", name),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			return b
		}
	}
	t.Fatalf("could not find %s", name)
	return nil
}

func pixelStats(t *testing.T, raw []byte) (opaque, nearWhite, nearBlack int) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)
			if a8 < 16 {
				continue
			}
			opaque++
			if r8 > 200 && g8 > 200 && b8 > 200 {
				nearWhite++
			}
			if r8 < 40 && g8 < 40 && b8 < 40 {
				nearBlack++
			}
		}
	}
	return
}
