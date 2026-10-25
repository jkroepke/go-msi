//go:build !race

package memtest

import "time"

// sampleInterval is how often PeakHeap reads the memory statistics. Each
// read stops the world, which is cheap without the race detector.
const sampleInterval = time.Millisecond
