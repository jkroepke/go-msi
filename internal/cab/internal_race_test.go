//go:build race

package cab //nolint:testpackage // the internal tests use it

// raceEnabled reports a race detector build for the internal tests, like the
// one of package cab_test in race_test.go.
const raceEnabled = true
