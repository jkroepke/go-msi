package artwork

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math/bits"
)

// MaxPixels bounds the side of a picture that Decode accepts. Wizard
// pictures are shown at a few hundred pixels; the bound keeps a wrong file
// from allocating gigabytes.
const MaxPixels = 8192

// maxArea bounds the pixel count, so that decoding needs at most a few
// hundred megabytes.
const maxArea = 4096 * 4096

// Decode reads a picture for the wizard: a Windows bitmap (BMP), PNG or
// JPEG. Transparent pixels are blended onto background, since the Bitmap
// control draws no alpha channel. The result is fully opaque and ready for
// EncodeBMP.
func Decode(data []byte, background color.RGBA) (*image.RGBA, error) {
	var (
		img image.Image
		err error
	)

	switch {
	case bytes.HasPrefix(data, []byte("BM")):
		img, err = decodeBMP(data)
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		img, err = decodeChecked("png", data, png.DecodeConfig, png.Decode)
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		img, err = decodeChecked("jpeg", data, jpeg.DecodeConfig, jpeg.Decode)
	default:
		return nil, errors.New("not a BMP, PNG or JPEG picture")
	}

	if err != nil {
		return nil, err
	}

	return flatten(img, background), nil
}

func decodeChecked(format string, data []byte, config func(io.Reader) (image.Config, error), decode func(io.Reader) (image.Image, error)) (image.Image, error) {
	cfg, err := config(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", format, err)
	}

	if err := checkSize(cfg.Width, cfg.Height); err != nil {
		return nil, fmt.Errorf("%s: %w", format, err)
	}

	img, err := decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", format, err)
	}

	return img, nil
}

func checkSize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("picture is %d×%d pixels", width, height)
	}

	if width > MaxPixels || height > MaxPixels || width*height > maxArea {
		return fmt.Errorf("picture is %d×%d pixels, at most %d per side and %d in all", width, height, MaxPixels, maxArea)
	}

	return nil
}

// flatten blends img onto background and returns an opaque RGBA copy.
func flatten(img image.Image, background color.RGBA) *image.RGBA {
	bounds := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))

	for row := range bounds.Dy() {
		for col := range bounds.Dx() {
			// RGBA returns alpha-premultiplied 16-bit channels.
			r, g, b, a := img.At(bounds.Min.X+col, bounds.Min.Y+row).RGBA()
			blend := func(c uint32, bg uint8) uint8 {
				return uint8((c + uint32(bg)*257*(0xFFFF-a)/0xFFFF) >> 8) //nolint:gosec // at most 0xFFFF before the shift
			}
			out.SetRGBA(col, row, color.RGBA{R: blend(r, background.R), G: blend(g, background.G), B: blend(b, background.B), A: 0xFF})
		}
	}

	return out
}

// Compression values of the bitmap header.
const (
	biRGB            = 0
	biBitfields      = 3
	biAlphaBitfields = 6

	coreHeaderSize = 12
)

// bmpHeader is the part of a bitmap header that Decode uses.
type bmpHeader struct {
	width, height int
	bitCount      int
	colorsUsed    int
	paletteAt     int
	entrySize     int
	masks         [4]uint32 // red, green, blue, alpha
	compression   uint32
	topDown       bool
}

// decodeBMP reads an uncompressed bitmap with any header version
// (BITMAPCOREHEADER, BITMAPINFOHEADER, V4 or V5): 1, 4 and 8 bits with a
// palette, 16 and 32 bits with or without bit fields, and 24 bits. RLE and
// embedded PNG or JPEG bitmaps are refused.
func decodeBMP(data []byte) (image.Image, error) {
	hdr, err := parseBMPHeader(data)
	if err != nil {
		return nil, err
	}

	endian := binary.LittleEndian
	offset := int(endian.Uint32(data[10:]))
	stride := (hdr.width*hdr.bitCount + 31) / 32 * 4

	if offset < fileHeaderSize || offset > len(data) || (len(data)-offset)/stride < hdr.height {
		return nil, errors.New("bitmap: pixel data is truncated")
	}

	palette, err := hdr.palette(data, offset)
	if err != nil {
		return nil, err
	}

	img := image.NewNRGBA(image.Rect(0, 0, hdr.width, hdr.height))
	hasAlpha := hdr.masks[3] != 0
	anyAlpha := hdr.readPixels(img, data[offset:], stride, palette)

	// A bitmap with an alpha mask whose alpha bytes are all zero was written
	// by a program that does not use the channel; it is opaque.
	if hasAlpha && !anyAlpha {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 0xFF
		}
	}

	return img, nil
}

func parseBMPHeader(data []byte) (*bmpHeader, error) {
	endian := binary.LittleEndian

	if len(data) < fileHeaderSize+coreHeaderSize {
		return nil, errors.New("bitmap: header is truncated")
	}

	size := int(endian.Uint32(data[14:]))
	if size < coreHeaderSize || fileHeaderSize+size > len(data) {
		return nil, errors.New("bitmap: header is truncated")
	}

	hdr := &bmpHeader{paletteAt: fileHeaderSize + size, entrySize: 4}

	if size == coreHeaderSize {
		hdr.width = int(endian.Uint16(data[18:]))
		hdr.height = int(endian.Uint16(data[20:]))
		hdr.bitCount = int(endian.Uint16(data[24:]))
		hdr.entrySize = 3
	} else {
		if size < infoHeaderSize {
			return nil, fmt.Errorf("bitmap: unknown header size %d", size)
		}

		hdr.width = int(int32(endian.Uint32(data[18:]))) //nolint:gosec // a signed field
		height := int(int32(endian.Uint32(data[22:])))   //nolint:gosec // a signed field
		hdr.topDown = height < 0
		hdr.height = max(height, -height)
		hdr.bitCount = int(endian.Uint16(data[28:]))
		hdr.compression = endian.Uint32(data[30:])
		hdr.colorsUsed = int(endian.Uint32(data[46:]))
	}

	if err := checkSize(hdr.width, hdr.height); err != nil {
		return nil, fmt.Errorf("bitmap: %w", err)
	}

	if err := hdr.readMasks(data, size); err != nil {
		return nil, err
	}

	return hdr, nil
}

// readPixels decodes the rows of pixel data into img and reports whether
// any pixel has a non-zero alpha value.
func (h *bmpHeader) readPixels(img *image.NRGBA, pixels []byte, stride int, palette []color.NRGBA) bool {
	hasAlpha := h.masks[3] != 0
	anyAlpha := false

	for row := range h.height {
		dstRow := h.height - 1 - row
		if h.topDown {
			dstRow = row
		}

		line := pixels[row*stride : (row+1)*stride]

		for col := range h.width {
			pixel := h.pixel(line, col, palette)
			if !hasAlpha {
				pixel.A = 0xFF
			} else if pixel.A != 0 {
				anyAlpha = true
			}

			img.SetNRGBA(col, dstRow, pixel)
		}
	}

	return anyAlpha
}

// readMasks sets the channel masks from the compression and bit count.
func (h *bmpHeader) readMasks(data []byte, size int) error {
	switch h.compression {
	case biRGB:
		return h.rgbMasks()
	case biBitfields, biAlphaBitfields:
		return h.bitfieldMasks(data, size)
	default:
		return fmt.Errorf("bitmap: compression %d is not supported; save the bitmap uncompressed", h.compression)
	}
}

// rgbMasks sets the fixed masks of an uncompressed bitmap.
func (h *bmpHeader) rgbMasks() error {
	switch h.bitCount {
	case 1, 4, 8, 24:
	case 16:
		h.masks = [4]uint32{0x7C00, 0x03E0, 0x001F, 0}
	case 32:
		// The fourth byte of a BI_RGB pixel is reserved, not alpha.
		h.masks = [4]uint32{0xFF0000, 0x00FF00, 0x0000FF, 0}
	default:
		return fmt.Errorf("bitmap: %d bits per pixel is not supported", h.bitCount)
	}

	return nil
}

// bitfieldMasks reads the masks of a bit-field bitmap. A 40-byte header
// stores them after itself; later versions hold them inside the header at
// the same offset.
func (h *bmpHeader) bitfieldMasks(data []byte, size int) error {
	if h.bitCount != 16 && h.bitCount != 32 {
		return fmt.Errorf("bitmap: bit fields with %d bits per pixel", h.bitCount)
	}

	count := 3
	if h.compression == biAlphaBitfields || size >= 56 {
		count = 4
	}

	maskAt := fileHeaderSize + infoHeaderSize
	if maskAt+4*count > len(data) {
		return errors.New("bitmap: bit fields are truncated")
	}

	for i := range count {
		h.masks[i] = binary.LittleEndian.Uint32(data[maskAt+4*i:])
	}

	if size == infoHeaderSize {
		h.paletteAt += 4 * count
	}

	return nil
}

func (h *bmpHeader) palette(data []byte, offset int) ([]color.NRGBA, error) {
	if h.bitCount > 8 {
		return nil, nil
	}

	count := 1 << h.bitCount
	if h.colorsUsed > 0 && h.colorsUsed < count {
		count = h.colorsUsed
	}

	end := h.paletteAt + count*h.entrySize
	if end > offset {
		return nil, errors.New("bitmap: palette overlaps the pixel data")
	}

	palette := make([]color.NRGBA, count)
	for i := range palette {
		entry := data[h.paletteAt+i*h.entrySize:]
		palette[i] = color.NRGBA{R: entry[2], G: entry[1], B: entry[0], A: 0xFF}
	}

	return palette, nil
}

// pixel returns pixel col of one row of pixel data.
func (h *bmpHeader) pixel(line []byte, col int, palette []color.NRGBA) color.NRGBA {
	switch h.bitCount {
	case 1, 4, 8:
		bit := col * h.bitCount
		index := int(line[bit/8]>>(8-h.bitCount-bit%8)) & (1<<h.bitCount - 1)

		if index >= len(palette) {
			return color.NRGBA{A: 0xFF}
		}

		return palette[index]
	case 24:
		p := line[col*3:]

		return color.NRGBA{R: p[2], G: p[1], B: p[0], A: 0xFF}
	}

	var word uint32
	if h.bitCount == 16 {
		word = uint32(binary.LittleEndian.Uint16(line[col*2:]))
	} else {
		word = binary.LittleEndian.Uint32(line[col*4:])
	}

	return color.NRGBA{R: channel(word, h.masks[0]), G: channel(word, h.masks[1]), B: channel(word, h.masks[2]), A: channel(word, h.masks[3])}
}

// channel extracts the bits of mask from word and scales them to 8 bits.
func channel(word, mask uint32) uint8 {
	if mask == 0 {
		return 0
	}

	shift := bits.TrailingZeros32(mask)
	width := bits.OnesCount32(mask)
	value := uint64((word & mask) >> shift)
	top := uint64(1)<<width - 1

	return uint8(min((value*0xFF+top/2)/top, 0xFF))
}
