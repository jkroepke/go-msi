// Package artwork draws the bitmaps of the msi-builder setup wizard and encodes
// them as Windows bitmaps for the Binary table. The built-in pictures are
// generated from a Palette, so a package never ships third-party pictures
// and a theme is a different palette. Decode reads the pictures a package
// brings itself.
package artwork

import (
	"encoding/binary"
	"image"
	"image/color"
	"math"
)

// Palette is the colour scheme of a wizard.
type Palette struct {
	// Dark and Light are the two ends of the side panel gradient.
	Dark  color.RGBA
	Light color.RGBA
	// Accent draws the stripe on the banner and the shapes on the panel.
	Accent color.RGBA
	// Paper is the banner background behind the dialog title.
	Paper color.RGBA
}

// Default is the standard msi-builder palette: deep blue to teal with an amber
// accent.
func Default() Palette {
	return Palette{
		Dark:   color.RGBA{R: 0x12, G: 0x2B, B: 0x4A, A: 0xFF},
		Light:  color.RGBA{R: 0x1E, G: 0x8C, B: 0x9E, A: 0xFF},
		Accent: color.RGBA{R: 0xF2, G: 0xA9, B: 0x3B, A: 0xFF},
		Paper:  color.RGBA{R: 0xF4, G: 0xF6, B: 0xF8, A: 0xFF},
	}
}

// Pixel sizes. The Bitmap control stretches the picture to its rectangle,
// so the sizes only set the resolution: both fill the controls that show
// them (370×44 dialog units for the banner, 370×234 for the dialog
// background) at 96 dpi, the sizes other installer toolkits use too. The
// side panel is the left 123 units of the dialog background; the page text
// starts at 135 units.
const (
	BannerWidth  = 493
	BannerHeight = 58
	DialogWidth  = 493
	DialogHeight = 312
	SideWidth    = 164
)

// Banner draws the strip shown at the top of the inner wizard pages: paper
// with a thin accent line at the bottom and a small mark at the right.
func Banner(palette Palette) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, BannerWidth, BannerHeight))

	for y := range BannerHeight {
		for x := range BannerWidth {
			img.SetRGBA(x, y, palette.Paper)
		}
	}

	// Accent line along the bottom edge.
	for y := BannerHeight - 3; y < BannerHeight; y++ {
		for x := range BannerWidth {
			img.SetRGBA(x, y, palette.Accent)
		}
	}

	// Three circles of the gradient colours as the mark, right aligned.
	mark(img, palette, BannerWidth-40, BannerHeight/2-5, 17)

	return img
}

// Dialog draws the background of the welcome and finish pages: on the
// left a panel with a diagonal gradient from Dark to Light and translucent
// circles, on the right paper behind the page text.
func Dialog(palette Palette) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, DialogWidth, DialogHeight))

	for y := range DialogHeight {
		for x := range DialogWidth {
			img.SetRGBA(x, y, palette.Paper)
		}
	}

	panel, ok := img.SubImage(image.Rect(0, 0, SideWidth, DialogHeight)).(*image.RGBA)
	if !ok {
		panic("artwork: SubImage of an RGBA is an RGBA")
	}

	for y := range DialogHeight {
		for x := range SideWidth {
			t := (float64(x)/SideWidth + float64(y)/DialogHeight) / 2
			panel.SetRGBA(x, y, mix(palette.Dark, palette.Light, t))
		}
	}

	circle(panel, 127, 260, 87, palette.Light, 0.35)
	circle(panel, 30, 80, 60, palette.Dark, 0.35)
	circle(panel, 143, 47, 37, palette.Accent, 0.9)

	return img
}

// mark draws three overlapping circles around (cx, cy).
func mark(img *image.RGBA, p Palette, cx, cy, radius int) {
	circle(img, cx-radius/2, cy, radius, p.Dark, 0.9)
	circle(img, cx+radius/2, cy, radius, p.Light, 0.7)
	circle(img, cx, cy+radius/2, radius, p.Accent, 0.7)
}

// circle blends a filled circle with the given opacity into img, with one
// pixel of edge smoothing.
func circle(img *image.RGBA, cx, cy, radius int, colour color.RGBA, opacity float64) {
	bounds := img.Bounds()

	for row := max(cy-radius-1, bounds.Min.Y); row <= min(cy+radius+1, bounds.Max.Y-1); row++ {
		for col := max(cx-radius-1, bounds.Min.X); col <= min(cx+radius+1, bounds.Max.X-1); col++ {
			dist := math.Hypot(float64(col-cx), float64(row-cy))

			cover := float64(radius) + 0.5 - dist
			if cover <= 0 {
				continue
			}

			img.SetRGBA(col, row, mix(img.RGBAAt(col, row), colour, opacity*min(cover, 1)))
		}
	}
}

func mix(from, to color.RGBA, t float64) color.RGBA {
	lerp := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
	}

	return color.RGBA{R: lerp(from.R, to.R), G: lerp(from.G, to.G), B: lerp(from.B, to.B), A: 0xFF}
}

const (
	fileHeaderSize = 14
	infoHeaderSize = 40
	bitsPerPixel   = 24
)

// EncodeBMP writes img as an uncompressed 24-bit bottom-up Windows bitmap
// (BITMAPINFOHEADER), the form every Windows Installer version displays.
func EncodeBMP(img image.Image) []byte {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	stride := (width*3 + 3) &^ 3
	size := fileHeaderSize + infoHeaderSize + stride*height

	out := make([]byte, size)
	endian := binary.LittleEndian

	copy(out, "BM")
	endian.PutUint32(out[2:], uint32(size))                   //nolint:gosec // bitmap sizes are far below 4 GiB
	endian.PutUint32(out[10:], fileHeaderSize+infoHeaderSize) // pixel data offset
	endian.PutUint32(out[14:], infoHeaderSize)                // biSize
	endian.PutUint32(out[18:], uint32(width))                 //nolint:gosec // bounded by the image size
	endian.PutUint32(out[22:], uint32(height))                //nolint:gosec // bounded by the image size
	endian.PutUint16(out[26:], 1)                             // biPlanes
	endian.PutUint16(out[28:], bitsPerPixel)                  // biBitCount
	endian.PutUint32(out[34:], uint32(stride*height))         //nolint:gosec // bounded by the image size
	endian.PutUint32(out[38:], 2835)                          // 72 dpi
	endian.PutUint32(out[42:], 2835)

	pixels := out[fileHeaderSize+infoHeaderSize:]

	for y := range height {
		row := pixels[(height-1-y)*stride:]

		for x := range width {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			row[x*3] = uint8(b >> 8)   //nolint:gosec // RGBA returns 16-bit channels
			row[x*3+1] = uint8(g >> 8) //nolint:gosec // RGBA returns 16-bit channels
			row[x*3+2] = uint8(r >> 8) //nolint:gosec // RGBA returns 16-bit channels
		}
	}

	return out
}
