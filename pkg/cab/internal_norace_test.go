//go:build !race

package cab //nolint:testpackage // the internal tests use it

// raceEnabled reports a race detector build; see race_test.go.
const raceEnabled = false
