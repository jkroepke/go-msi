// Package memtest measures the peak heap of a function for tests that must
// run in bounded memory.
package memtest

import (
	"io"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// Pattern returns a reader of size bytes that look like a binary: runs of a
// pseudo-random page interleaved with counters, so that DEFLATE neither
// stores it raw nor collapses it to nothing. The same size gives the same
// bytes.
func Pattern(size int64) *PatternReader {
	page := make([]byte, 4096)
	state := uint32(0x9E3779B9)

	for i := range page {
		state = state*1664525 + 1013904223
		page[i] = byte(state >> 24)
	}

	return &PatternReader{page: page, size: size}
}

// PatternReader is the reader Pattern returns.
type PatternReader struct {
	page []byte
	size int64
	pos  int64
}

func (p *PatternReader) Read(buf []byte) (int, error) {
	if p.pos >= p.size {
		return 0, io.EOF
	}

	count := int(min(int64(len(buf)), p.size-p.pos))

	for i := range count {
		off := p.pos + int64(i)
		if off%64 < 8 {
			buf[i] = byte(off >> (8 * (off % 8))) //nolint:gosec // one byte of the counter is wanted
		} else {
			buf[i] = p.page[off%int64(len(p.page))]
		}
	}

	p.pos += int64(count)

	return count, nil
}

// Close makes the reader an io.ReadCloser.
func (p *PatternReader) Close() error { return nil }

// Peak is what PeakHeap measured.
type Peak struct {
	// Before is the heap in use when fn started, after a collection.
	Before uint64
	// Max is the largest heap in use observed while fn ran.
	Max uint64
}

// Used is the heap fn added at its peak.
func (p Peak) Used() uint64 { return p.Max - p.Before }

// PeakHeap runs the function under a soft memory limit and samples the heap
// in use every millisecond while it runs. The limit makes the collector
// reclaim garbage promptly, so Max reflects live data plus a small allocation
// burst rather than collector pacing; code that holds more live data than the
// limit still exceeds it.
func PeakHeap(limit int64, run func()) Peak {
	previous := debug.SetMemoryLimit(limit)
	defer debug.SetMemoryLimit(previous)

	runtime.GC() //nolint:revive // a collection gives a baseline that holds only live data

	var stats runtime.MemStats

	runtime.ReadMemStats(&stats)

	peak := Peak{Before: stats.HeapAlloc}
	stop := make(chan struct{})
	result := make(chan uint64, 1)

	var wg sync.WaitGroup

	wg.Go(func() {
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()

		var observed uint64

		for {
			var sample runtime.MemStats

			runtime.ReadMemStats(&sample)
			observed = max(observed, sample.HeapAlloc)

			select {
			case <-stop:
				result <- observed

				return
			case <-ticker.C:
			}
		}
	})

	run()
	close(stop)
	wg.Wait()

	peak.Max = max(<-result, peak.Before)

	return peak
}
