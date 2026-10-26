//go:build race

package cab //nolint:testpackage // shared with the internal tests

// raceEnabled reports a race detector build. With -race and atomic coverage
// the LZX encoder runs about fifteen to fifty times slower, so the tests that
// compress the most shrink and CI runs them at full size without -race.
const raceEnabled = true
