//go:build !race

package cab //nolint:testpackage // shared with the internal tests

// raceEnabled reports a race detector build; see internal_race_test.go.
const raceEnabled = false
