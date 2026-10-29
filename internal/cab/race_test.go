//go:build race

package cab_test

// raceEnabled reports a race detector build. The detector slows the LZX
// encoder down about fifty times, so the tests that compress megabytes with
// it shrink or leave that to the CI step that runs them without -race.
const raceEnabled = true
