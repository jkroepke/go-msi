//go:build race

package lzx //nolint:testpackage // the internal tests use it

// raceEnabled reports a race detector build. With -race and atomic coverage
// the LZX encoder runs up to fifty times slower, so the tests that compress
// the most shrink and CI runs them at full size without -race.
const raceEnabled = true
