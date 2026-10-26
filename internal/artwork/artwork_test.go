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

func TestEncodeICO(t *testing.T) {
	t.Parallel()

	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	img.SetNRGBA(1, 1, color.NRGBA{B: 255, A: 128})

	out := artwork.EncodeICO(img)
	endian := binary.LittleEndian

	// ICONDIR, one entry, a 40-byte header, 2×2×4 pixel bytes and two
	// 4-byte rows of AND mask.
	if want := 6 + 16 + 40 + 16 + 8; len(out) != want {
		t.Fatalf("%d bytes, want %d", len(out), want)
	}

	if endian.Uint16(out[2:]) != 1 || endian.Uint16(out[4:]) != 1 || out[6] != 2 || out[7] != 2 ||
		out[8] != 0 || out[9] != 0 || endian.Uint16(out[10:]) != 1 ||
		endian.Uint16(out[12:]) != 32 || endian.Uint32(out[14:]) != 40+16+8 || endian.Uint32(out[18:]) != 22 {
		t.Errorf("directory %v", out[:22])
	}

	dib := out[22:]
	if endian.Uint32(dib) != 40 || endian.Uint32(dib[4:]) != 2 || endian.Uint32(dib[8:]) != 4 || endian.Uint16(dib[14:]) != 32 ||
		endian.Uint32(dib[16:]) != 0 || endian.Uint32(dib[20:]) != 0 {
		t.Errorf("header %v", dib[:40])
	}

	pixels := dib[40:]
	// Bottom-up BGRA: the first stored row is y=1.
	if !bytes.Equal(pixels[4:8], []byte{255, 0, 0, 128}) || !bytes.Equal(pixels[8:12], []byte{0, 0, 255, 255}) {
		t.Errorf("pixels %v", pixels[:16])
	}

	// The AND mask marks the transparent pixels: (0,1) and (1,0).
	if mask := pixels[16:]; mask[0] != 0x80 || mask[4] != 0x40 {
		t.Errorf("mask %v", mask)
	}
}

func TestErrorIcon(t *testing.T) {
	t.Parallel()

	img := artwork.ErrorIcon(artwork.Default())
	if img.Bounds().Dx() != artwork.IconSize || img.Bounds().Dy() != artwork.IconSize {
		t.Fatalf("size %v", img.Bounds())
	}

	if c := img.NRGBAAt(0, 0); c.A != 0 {
		t.Errorf("corner %v is not transparent", c)
	}

	if c := img.NRGBAAt(16, 12); c != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Errorf("mark %v is not white", c)
	}

	if c, accent := img.NRGBAAt(8, 16), artwork.Default().Accent; c.R != accent.R || c.G != accent.G || c.B != accent.B || c.A != 255 {
		t.Errorf("disc %v is not the accent colour", c)
	}
}
