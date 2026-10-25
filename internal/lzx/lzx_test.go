package lzx //nolint:testpackage // the tests reach the unexported encoder parts

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
)

func decodeAll(t *testing.T, frames [][]byte, window, size int) []byte {
	t.Helper()

	dec, err := NewDecoder(window)
	if err != nil {
		t.Fatal(err)
	}

	var out []byte

	for i, frame := range frames {
		n := min(FrameSize, size-i*FrameSize)

		got, err := dec.Decode(frame, n)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}

		out = append(out, got...)
	}

	return out
}

// x86ish builds data with E8 calls, repeated records and text, the mix an
// executable has.
func x86ish(size int, seed int64) []byte {
	rng := rand.New(rand.NewSource(seed)) //nolint:gosec // reproducible test data
	out := make([]byte, 0, size)

	words := []string{"windows", "installer", "cabinet", "exporter", "alloy", "metrics", "\x00\x00\x00\x00"}

	for len(out) < size {
		switch rng.Intn(5) {
		case 0:
			out = append(out, 0xE8)
			out = binary.LittleEndian.AppendUint32(out, uint32(rng.Intn(1<<20)-1<<19)) //nolint:gosec // test data
		case 1:
			out = append(out, words[rng.Intn(len(words))]...)
		case 2:
			if len(out) > 300 {
				from := len(out) - 1 - rng.Intn(min(len(out)-260, 1<<18))
				n := 3 + rng.Intn(250)
				out = append(out, out[from:from+n]...)
			}
		case 3:
			// 16-byte records: aligned-offset friendly distances.
			for range 4 {
				out = binary.LittleEndian.AppendUint32(out, uint32(rng.Intn(16))*16) //nolint:gosec // test data
			}
		default:
			out = append(out, byte(rng.Intn(256))) //nolint:gosec // below 256
		}
	}

	return out[:size]
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	random := make([]byte, 200000)
	rand.New(rand.NewSource(1)).Read(random) //nolint:gosec // reproducible test data

	inputs := map[string][]byte{
		"one":         {42},
		"short":       []byte("abc"),
		"ten":         []byte("0123456789"),
		"eleven-e8":   {0xE8, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		"frame":       bytes.Repeat([]byte("ab"), FrameSize/2),
		"frame+1":     bytes.Repeat([]byte("x"), FrameSize+1),
		"zeros":       make([]byte, 5*FrameSize+17),
		"random":      random,
		"x86":         x86ish(700000, 2),
		"text":        bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. 0123456789\r\n"), 9000),
		"e8-tail":     append(bytes.Repeat([]byte{0xE8, 0x10, 0, 0, 0}, 7000), 0xE8, 1, 2, 3, 4),
		"block-split": x86ish(blockSpan+FrameSize+5, 3),
	}

	for name, data := range inputs {
		for _, opts := range []Options{{}, {Window: 15, Level: 1}, {Window: 16, Block: blockVerb}, {Window: 18, Block: blockAligned}, {Window: 21, Level: 4, Block: blockAligned}} {
			t.Run(fmt.Sprintf("%s/w%d-l%d-b%d", name, opts.Window, opts.Level, opts.Block), func(t *testing.T) {
				t.Parallel()

				frames, err := Compress(data, opts)
				if err != nil {
					t.Fatal(err)
				}

				if want := (len(data) + FrameSize - 1) / FrameSize; len(frames) != want {
					t.Fatalf("%d frames, want %d", len(frames), want)
				}

				for i, f := range frames {
					if len(f)%2 != 0 || len(f) > FrameSize+6144 {
						t.Fatalf("frame %d is %d bytes", i, len(f))
					}
				}

				window := opts.Window
				if window == 0 {
					window = MaxWindow
				}

				if got := decodeAll(t, frames, window, len(data)); !bytes.Equal(got, data) {
					t.Fatalf("round trip differs at %d", firstDiff(got, data))
				}
			})
		}
	}
}

func firstDiff(got, want []byte) int {
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			return i
		}
	}

	return min(len(got), len(want))
}

// TestSpecExample decodes the uncompressed block example of [MS-PATCH]
// section 3 without its chunk-size prefix, which cabinets do not have.
func TestSpecExample(t *testing.T) {
	t.Parallel()

	frame := []byte{0x00, 0x30, 0x30, 0x00, 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0x61, 0x62, 0x63, 0x00}

	dec, err := NewDecoder(17)
	if err != nil {
		t.Fatal(err)
	}

	got, err := dec.Decode(frame, 3)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != "abc" {
		t.Fatalf("got %q", got)
	}
}

func TestE8Inverse(t *testing.T) {
	t.Parallel()

	data := x86ish(3*FrameSize, 7)
	orig := append([]byte(nil), data...)

	for off := 0; off < len(data); off += FrameSize {
		e8(data[off:off+FrameSize], off, int32(len(data)), true) //nolint:gosec // 96 KB
	}

	if bytes.Equal(data, orig) {
		t.Fatal("translation changed nothing")
	}

	for off := 0; off < len(data); off += FrameSize {
		e8(data[off:off+FrameSize], off, int32(len(data)), false) //nolint:gosec // 96 KB
	}

	if !bytes.Equal(data, orig) {
		t.Fatalf("restore differs at %d", firstDiff(data, orig))
	}
}

func TestSlots(t *testing.T) {
	t.Parallel()

	// [MS-PATCH] 2.6.2: slot 49 ends at 2^21-1, the 2 MB window.
	if got := basePosition[49] + 1<<footerBits[49] - 1; got != 1<<21-1 {
		t.Fatalf("slot 49 ends at %d", got)
	}

	for formatted := range uint32(1 << 21) {
		slot := slotOf(formatted)
		if formatted < basePosition[slot] || formatted-basePosition[slot] >= 1<<footerBits[slot] && slot >= 3 {
			t.Fatalf("formatted %d in slot %d", formatted, slot)
		}
	}
}

func TestCorruptFramesFail(t *testing.T) {
	t.Parallel()

	data := x86ish(100000, 9)

	frames, err := Compress(data, Options{})
	if err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewSource(4)) //nolint:gosec // reproducible test data

	for range 200 {
		dec, _ := NewDecoder(MaxWindow)

		broken := append([]byte(nil), frames[0]...)
		broken[rng.Intn(len(broken))] ^= byte(1 + rng.Intn(255)) //nolint:gosec // 1 to 255

		// Must not panic; an error or different data are both fine.
		_, _ = dec.Decode(broken, FrameSize)
		_, _ = dec.Decode(frames[1][:len(frames[1])/2], FrameSize)
	}
}

func FuzzDecode(f *testing.F) {
	frames, _ := Compress(x86ish(40000, 5), Options{Window: 15})
	f.Add(frames[0], 15)
	f.Fuzz(func(_ *testing.T, frame []byte, window int) {
		dec, err := NewDecoder(MinWindow + (window&0xFF)%7)
		if err != nil {
			return
		}

		_, _ = dec.Decode(frame, FrameSize)
	})
}

func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte("hello hello hello"), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, mode uint8) {
		opts := Options{Window: MinWindow + int(mode%7), Level: 1 + int(mode/7%9), Block: int(mode / 63 % 3)}

		frames, err := Compress(data, opts)
		if err != nil {
			t.Fatal(err)
		}

		if len(data) == 0 {
			return
		}

		if got := decodeAll(t, frames, opts.Window, len(data)); !bytes.Equal(got, data) {
			t.Fatalf("round trip differs at %d", firstDiff(got, data))
		}
	})
}

func TestSlotTables(t *testing.T) {
	t.Parallel()

	for slot := range footerBits {
		want := uint32(0)
		if slot >= 4 {
			want = uint32(min((slot-2)/2, 17))
		}

		if footerBits[slot] != want {
			t.Errorf("slot %d: %d footer bits, want %d", slot, footerBits[slot], want)
		}

		if slot > 0 && basePosition[slot] != basePosition[slot-1]+1<<footerBits[slot-1] {
			t.Errorf("slot %d: base %d", slot, basePosition[slot])
		}
	}
}

// TestWorkersDeterministic checks that the optimal parser's output does not
// depend on how many goroutines parse the segments.
func TestWorkersDeterministic(t *testing.T) {
	t.Parallel()

	data := x86ish(segmentSize+3*blockSpan+77, 13)

	one, err := Compress(data, Options{Level: optimalLevel, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	three, err := Compress(data, Options{Level: optimalLevel, Workers: 3})
	if err != nil {
		t.Fatal(err)
	}

	if len(one) != len(three) {
		t.Fatalf("%d frames with one worker, %d with three", len(one), len(three))
	}

	for i := range one {
		if !bytes.Equal(one[i], three[i]) {
			t.Fatalf("frame %d differs between one and three workers", i)
		}
	}

	if got := decodeAll(t, one, MaxWindow, len(data)); !bytes.Equal(got, data) {
		t.Fatalf("round trip differs at %d", firstDiff(got, data))
	}
}
