package artwork_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"github.com/jkroepke/go-msi/artwork"
)

func TestEncodeBMP(t *testing.T) {
	t.Parallel()

	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	img.SetRGBA(2, 1, color.RGBA{B: 255, A: 255})

	out := artwork.EncodeBMP(img)

	// 3 pixels × 3 bytes = 9, padded to 12 per row; two rows.
	if want := 14 + 40 + 24; len(out) != want || string(out[:2]) != "BM" {
		t.Fatalf("%d bytes, want %d", len(out), want)
	}

	if int(binary.LittleEndian.Uint32(out[2:])) != len(out) || binary.LittleEndian.Uint16(out[28:]) != 24 {
		t.Error("header fields are wrong")
	}

	pixels := out[54:]
	// Bottom-up: the first stored row is y=1, whose last pixel is blue.
	if !bytes.Equal(pixels[6:9], []byte{255, 0, 0}) {
		t.Errorf("row 1 pixel 2 = %v, want blue in BGR", pixels[6:9])
	}

	// The second stored row is y=0, whose first pixel is red.
	if !bytes.Equal(pixels[12:15], []byte{0, 0, 255}) {
		t.Errorf("row 0 pixel 0 = %v, want red in BGR", pixels[12:15])
	}
}

func TestPicturesAreDeterministic(t *testing.T) {
	t.Parallel()

	palette := artwork.Default()

	banner := artwork.EncodeBMP(artwork.Banner(palette))
	if !bytes.Equal(banner, artwork.EncodeBMP(artwork.Banner(palette))) {
		t.Error("banner differs between runs")
	}

	side := artwork.Side(palette)
	if !bytes.Equal(artwork.EncodeBMP(side), artwork.EncodeBMP(artwork.Side(palette))) {
		t.Error("side panel differs between runs")
	}

	if side.Bounds().Dx() != artwork.SideWidth || side.Bounds().Dy() != artwork.SideHeight {
		t.Errorf("side panel is %v", side.Bounds())
	}

	// The gradient runs from Dark at the top left to Light at the bottom right.
	if got := side.RGBAAt(0, 0); got != palette.Dark {
		t.Errorf("top left is %v, want %v", got, palette.Dark)
	}
}
