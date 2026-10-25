//go:build race

package memtest

import "time"

// sampleInterval is how often PeakHeap reads the memory statistics. Under
// the race detector every read is slow, and the peak lasts long enough to
// catch at this coarser rate.
const sampleInterval = 20 * time.Millisecond
