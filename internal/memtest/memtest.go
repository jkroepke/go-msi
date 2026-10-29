// Package memtest measures the peak heap of a function for tests that must
// run in bounded memory.
package memtest

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
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
	// Before is the live heap when fn started, after a collection.
	Before uint64
	// Max is the largest live heap that two successive sampled collections
	// both marked while fn ran: the larger of min(live[i], live[i+1]).
	Max uint64
	// Marked is the largest live heap a single collection marked. A
	// collection also counts what is allocated while it marks, so a long mark
	// on a busy machine inflates it; it is only for logs.
	Marked uint64
	// Allocated is the largest heap in use observed while fn ran, garbage
	// not yet collected included. It depends on collector pacing and the
	// load of the machine, so it is only for logs.
	Allocated uint64
}

// Used is the live heap fn added at its peak.
func (p Peak) Used() uint64 { return p.Max - p.Before }

// String describes the measurement for test logs.
func (p Peak) String() string {
	return fmt.Sprintf("live heap before %d, peak %d (one cycle %d), allocated peak %d", p.Before, p.Max, p.Marked, p.Allocated)
}

// sampleInterval is how often PeakHeap reads the heap metrics. A read does
// not stop the world, so it is cheap also under the race detector.
const sampleInterval = time.Millisecond

const (
	cyclesMetric    = "/gc/cycles/total:gc-cycles"
	liveMetric      = "/gc/heap/live:bytes"
	allocatedMetric = "/memory/classes/heap/objects:bytes"
)

// PeakHeap runs the function under a soft memory limit and records the live
// heap each collection marked while it ran.
//
// The heap in use (HeapAlloc) is not a stable measure: between collections it
// holds garbage up to the collector's goal, twice the live heap by default,
// so its peak moves with GOGC and allocation rate. The live heap of one
// collection is not stable either: objects allocated while it marks count as
// live, and a mark takes longer on a busy machine. Data fn holds stays live
// across collections, while garbage allocated during one mark is gone by the
// next, so Max takes the smaller of each pair of successive collections.
// The limit makes sure code that holds more than limit bytes and keeps
// allocating is collected, and so marked, repeatedly while it does; a
// one-shot spike that a single collection marks shows up only in Marked.
func PeakHeap(limit int64, run func()) Peak {
	previous := debug.SetMemoryLimit(limit)
	defer debug.SetMemoryLimit(previous)

	runtime.GC() //nolint:revive // a collection gives a baseline that holds only live data

	samples := []metrics.Sample{{Name: cyclesMetric}, {Name: liveMetric}, {Name: allocatedMetric}}
	metrics.Read(samples)

	before := samples[1].Value.Uint64()
	stop := make(chan struct{})
	result := make(chan Peak, 1)

	var wg sync.WaitGroup

	wg.Go(func() {
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()

		observed := Peak{Before: before, Max: before, Marked: before}
		cycles := samples[0].Value.Uint64()
		last := before

		for done := false; !done; {
			select {
			case <-stop:
				// A last collection ends the series. It only sees data that
				// outlives fn, so a peak only the final cycle marked shows
				// up in Marked alone.
				runtime.GC()

				done = true
			case <-ticker.C:
			}

			metrics.Read(samples)

			if n := samples[0].Value.Uint64(); n != cycles {
				live := samples[1].Value.Uint64()
				observed.Max = max(observed.Max, min(last, live))
				observed.Marked = max(observed.Marked, live)
				cycles, last = n, live
			}

			observed.Allocated = max(observed.Allocated, samples[2].Value.Uint64())
		}

		result <- observed
	})

	run()
	close(stop)
	wg.Wait()

	return <-result
}
