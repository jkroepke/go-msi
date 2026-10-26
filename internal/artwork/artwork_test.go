package artwork_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"github.com/jkroepke/go-msi/internal/artwork"
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

	dialog := artwork.Dialog(palette)
	if !bytes.Equal(artwork.EncodeBMP(dialog), artwork.EncodeBMP(artwork.Dialog(palette))) {
		t.Error("dialog background differs between runs")
	}

	if dialog.Bounds().Dx() != artwork.DialogWidth || dialog.Bounds().Dy() != artwork.DialogHeight {
		t.Errorf("dialog background is %v", dialog.Bounds())
	}

	// The panel gradient runs from Dark at the top left; right of the panel is paper.
	if got := dialog.RGBAAt(0, 0); got != palette.Dark {
		t.Errorf("top left is %v, want %v", got, palette.Dark)
	}

	if got := dialog.RGBAAt(artwork.SideWidth, 0); got != palette.Paper {
		t.Errorf("right of the panel is %v, want %v", got, palette.Paper)
	}
}
