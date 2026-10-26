package artwork_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/internal/artwork"
)

var white = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} //nolint:gochecknoglobals // test constant

// bitmapFile describes a bitmap for bmp: a header of headerSize bytes, the
// masks for bit fields, a palette and rows of pixel data in file order.
type bitmapFile struct {
	masks         []uint32
	palette       []byte
	rows          [][]byte
	headerSize    int
	width, height int
	bitCount      int
	compression   uint32
}

// bmp assembles the bitmap file that file describes.
func bmp(file bitmapFile) []byte {
	endian := binary.LittleEndian
	hdr := make([]byte, file.headerSize)
	endian.PutUint32(hdr, uint32(file.headerSize)) //nolint:gosec // small test values
	endian.PutUint32(hdr[4:], uint32(file.width))  //nolint:gosec // small test values
	endian.PutUint32(hdr[8:], uint32(file.height)) //nolint:gosec // a signed field
	endian.PutUint16(hdr[12:], 1)
	endian.PutUint16(hdr[14:], uint16(file.bitCount)) //nolint:gosec // small test values
	endian.PutUint32(hdr[16:], file.compression)

	var extra []byte

	for i, m := range file.masks {
		if file.headerSize > 40 {
			endian.PutUint32(hdr[40+4*i:], m)
		} else {
			extra = endian.AppendUint32(extra, m)
		}
	}

	extra = append(extra, file.palette...)
	offset := 14 + len(hdr) + len(extra)

	var pixels []byte

	for _, row := range file.rows {
		row = append(row, make([]byte, (4-len(row)%4)%4)...)
		pixels = append(pixels, row...)
	}

	out := []byte("BM")
	out = endian.AppendUint32(out, uint32(offset+len(pixels))) //nolint:gosec // small test values
	out = endian.AppendUint32(out, 0)
	out = endian.AppendUint32(out, uint32(offset)) //nolint:gosec // small test values
	out = append(out, hdr...)
	out = append(out, extra...)

	return append(out, pixels...)
}

func TestDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	img := artwork.Banner(artwork.Default())

	got, err := artwork.Decode(artwork.EncodeBMP(img), white)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got.Pix, img.Pix) {
		t.Error("a decoded EncodeBMP picture differs from the original")
	}
}

func TestDecodeV5Bitfields(t *testing.T) {
	t.Parallel()

	// A 2×2 32-bit V5 bitmap with an alpha mask, as image editors save
	// them: bottom row first, the top right pixel half transparent black.
	masks := []uint32{0x00FF0000, 0x0000FF00, 0x000000FF, 0xFF000000}
	data := bmp(bitmapFile{headerSize: 124, width: 2, height: 2, bitCount: 32, compression: 3, masks: masks, rows: [][]byte{
		{0x00, 0x00, 0xFF, 0xFF, 0x00, 0xFF, 0x00, 0xFF},
		{0xFF, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x80},
	}})

	img, err := artwork.Decode(data, white)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct {
		x, y int
		c    color.RGBA
	}{
		{0, 1, color.RGBA{R: 0xFF, A: 0xFF}},
		{1, 1, color.RGBA{G: 0xFF, A: 0xFF}},
		{0, 0, color.RGBA{B: 0xFF, A: 0xFF}},
		{1, 0, color.RGBA{R: 0x7F, G: 0x7F, B: 0x7F, A: 0xFF}},
	} {
		if got := img.RGBAAt(want.x, want.y); got != want.c {
			t.Errorf("pixel %d,%d = %v, want %v", want.x, want.y, got, want.c)
		}
	}
}

func TestDecodeZeroAlphaIsOpaque(t *testing.T) {
	t.Parallel()

	masks := []uint32{0x00FF0000, 0x0000FF00, 0x000000FF, 0xFF000000}
	data := bmp(bitmapFile{headerSize: 124, width: 1, height: 1, bitCount: 32, compression: 3, masks: masks, rows: [][]byte{{0x10, 0x20, 0x30, 0x00}}})

	img, err := artwork.Decode(data, white)
	if err != nil {
		t.Fatal(err)
	}

	if got := img.RGBAAt(0, 0); got != (color.RGBA{R: 0x30, G: 0x20, B: 0x10, A: 0xFF}) {
		t.Errorf("pixel = %v", got)
	}
}

func TestDecodePaletteTopDown(t *testing.T) {
	t.Parallel()

	// 4-bit, 3×2, top-down (negative height), two palette entries.
	palette := []byte{0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0}
	h := -2
	data := bmp(bitmapFile{headerSize: 40, width: 3, height: h, bitCount: 4, palette: palette, rows: [][]byte{{0x10, 0x10}, {0x01, 0x00}}})
	binary.LittleEndian.PutUint32(data[14+32:], 2) // biClrUsed

	img, err := artwork.Decode(data, white)
	if err != nil {
		t.Fatal(err)
	}

	black := color.RGBA{A: 0xFF}
	for _, want := range []struct {
		x, y int
		c    color.RGBA
	}{{0, 0, white}, {1, 0, black}, {2, 0, white}, {0, 1, black}, {1, 1, white}, {2, 1, black}} {
		if got := img.RGBAAt(want.x, want.y); got != want.c {
			t.Errorf("pixel %d,%d = %v, want %v", want.x, want.y, got, want.c)
		}
	}
}

func TestDecode16Bit(t *testing.T) {
	t.Parallel()

	// BI_RGB 16-bit is 5-5-5: 0x7C00 is full red.
	data := bmp(bitmapFile{headerSize: 40, width: 1, height: 1, bitCount: 16, rows: [][]byte{{0x00, 0x7C}}})

	img, err := artwork.Decode(data, white)
	if err != nil {
		t.Fatal(err)
	}

	if got := img.RGBAAt(0, 0); got != (color.RGBA{R: 0xFF, A: 0xFF}) {
		t.Errorf("pixel = %v", got)
	}
}

func TestDecodePNG(t *testing.T) {
	t.Parallel()

	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 0xFF, A: 0xFF})
	src.SetNRGBA(1, 0, color.NRGBA{}) // transparent: the background shows

	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}

	paper := color.RGBA{R: 1, G: 2, B: 3, A: 0xFF}

	img, err := artwork.Decode(buf.Bytes(), paper)
	if err != nil {
		t.Fatal(err)
	}

	if img.RGBAAt(0, 0) != (color.RGBA{R: 0xFF, A: 0xFF}) || img.RGBAAt(1, 0) != paper {
		t.Errorf("pixels %v %v", img.RGBAAt(0, 0), img.RGBAAt(1, 0))
	}
}

func TestDecodeErrors(t *testing.T) {
	t.Parallel()

	good := bmp(bitmapFile{headerSize: 40, width: 2, height: 2, bitCount: 24, rows: [][]byte{make([]byte, 6), make([]byte, 6)}})
	huge := bmp(bitmapFile{headerSize: 40, width: artwork.MaxPixels + 1, height: 1, bitCount: 24})
	rle := bmp(bitmapFile{headerSize: 40, width: 2, height: 2, bitCount: 8, compression: 1})

	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"gif", []byte("GIF89a"), "not a BMP, PNG or JPEG"},
		{"truncated header", good[:20], "header is truncated"},
		{"truncated pixels", good[:len(good)-4], "pixel data is truncated"},
		{"too large", huge, "at most 8192"},
		{"rle", rle, "compression 1"},
		{"bad png", []byte("\x89PNG\r\n\x1a\nxxxx"), "png"},
	} {
		_, err := artwork.Decode(tc.data, white)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestDecodeHeaderVariants(t *testing.T) {
	t.Parallel()

	red := color.RGBA{R: 0xFF, A: 0xFF}
	black := color.RGBA{A: 0xFF}

	core := bmp(bitmapFile{headerSize: 40, width: 2, height: 1, bitCount: 1, palette: []byte{0, 0, 0, 0, 0, 0xFF}, rows: [][]byte{{0x40}}})
	// Rewrite the 40-byte header as a 12-byte BITMAPCOREHEADER with
	// 3-byte palette entries: size, 16-bit width and height, planes, bits.
	core = append(append(append(make([]byte, 0, len(core)), core[:14]...),
		12, 0, 0, 0, 2, 0, 1, 0, 1, 0, 1, 0), core[54:]...)
	binary.LittleEndian.PutUint32(core[10:], 14+12+6)

	for _, tc := range []struct {
		name string
		data []byte
		want []color.RGBA
	}{
		{"core header, 1 bit", core, []color.RGBA{black, red}},
		{"1 bit, 4-byte palette", bmp(bitmapFile{
			headerSize: 40, width: 2, height: 1, bitCount: 1, palette: []byte{0, 0, 0xFF, 0, 0, 0, 0, 0}, rows: [][]byte{{0x40}},
		}), []color.RGBA{red, black}},
		{"40-byte header with bit fields", bmp(bitmapFile{
			headerSize: 40, width: 1, height: 1, bitCount: 16, compression: 3, masks: []uint32{0xF800, 0x07E0, 0x001F}, rows: [][]byte{{0x00, 0xF8}},
		}), []color.RGBA{red}},
		{"alpha bit fields", bmp(bitmapFile{
			headerSize: 40, width: 1, height: 1, bitCount: 32, compression: 6,
			masks: []uint32{0x00FF0000, 0x0000FF00, 0x000000FF, 0xFF000000}, rows: [][]byte{{0, 0, 0xFF, 0xFF}},
		}), []color.RGBA{red}},
	} {
		img, err := artwork.Decode(tc.data, white)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)

			continue
		}

		for x, want := range tc.want {
			if got := img.RGBAAt(x, 0); got != want {
				t.Errorf("%s: pixel %d = %v, want %v", tc.name, x, got, want)
			}
		}
	}
}
