package artwork

import (
	"encoding/binary"
	"image"
	"image/color"
	"math"
)

// IconSize is the edge of the error icon in pixels.
const IconSize = 32

// ErrorIcon draws the icon of the error dialog: an accent disc with a white
// exclamation mark on a transparent background.
func ErrorIcon(palette Palette) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, IconSize, IconSize))
	white := color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	centre := float64(IconSize) / 2

	for y := range IconSize {
		for x := range IconSize {
			midX, midY := float64(x)+0.5, float64(y)+0.5

			disc := min(max(centre-0.5-math.Hypot(midX-centre, midY-centre), 0), 1)
			if disc == 0 {
				continue
			}

			colour := palette.Accent

			// The bar of the mark spans rows 7–19, the dot rows 21–25.
			bar := math.Abs(midX-centre) <= 2 && midY >= 7 && midY <= 20
			dot := math.Hypot(midX-centre, midY-23.5) <= 2.25

			if bar || dot {
				colour = white
			}

			img.SetNRGBA(x, y, color.NRGBA{R: colour.R, G: colour.G, B: colour.B, A: uint8(math.Round(disc * 0xFF))})
		}
	}

	return img
}

const (
	iconDirSize   = 6
	iconEntrySize = 16
	iconBits      = 32
)

// EncodeICO writes img as a Windows icon file with one 32-bit image: an
// ICONDIR, one ICONDIRENTRY and a BITMAPINFOHEADER DIB of double height,
// followed by the bottom-up BGRA pixels and the 1-bit AND mask. The image
// must be at most 256 pixels square.
func EncodeICO(img image.Image) []byte {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	maskStride := ((width + 31) / 32) * 4
	dibSize := infoHeaderSize + width*height*4 + maskStride*height
	out := make([]byte, iconDirSize+iconEntrySize+dibSize)
	endian := binary.LittleEndian

	endian.PutUint16(out[2:], 1) // type: icon
	endian.PutUint16(out[4:], 1) // one image

	entry := out[iconDirSize:]
	entry[0] = byte(width)         // 0 means 256
	entry[1] = byte(height)        // 0 means 256
	endian.PutUint16(entry[4:], 1) // planes
	endian.PutUint16(entry[6:], iconBits)
	endian.PutUint32(entry[8:], uint32(dibSize))            //nolint:gosec // icon sizes are tiny
	endian.PutUint32(entry[12:], iconDirSize+iconEntrySize) // image offset

	dib := out[iconDirSize+iconEntrySize:]
	endian.PutUint32(dib[0:], infoHeaderSize)
	endian.PutUint32(dib[4:], uint32(width))    //nolint:gosec // bounded by the image size
	endian.PutUint32(dib[8:], uint32(2*height)) //nolint:gosec // XOR image plus AND mask
	endian.PutUint16(dib[12:], 1)
	endian.PutUint16(dib[14:], iconBits)

	pixels := dib[infoHeaderSize:]
	mask := pixels[width*height*4:]

	for y := range height {
		row := pixels[(height-1-y)*width*4:]
		maskRow := mask[(height-1-y)*maskStride:]

		for x := range width {
			c := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA) //nolint:forcetypeassert // the model returns NRGBA
			row[x*4], row[x*4+1], row[x*4+2], row[x*4+3] = c.B, c.G, c.R, c.A

			if c.A == 0 {
				maskRow[x/8] |= 0x80 >> (x % 8)
			}
		}
	}

	return out
}
