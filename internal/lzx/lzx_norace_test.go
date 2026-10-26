//go:build !race

package lzx //nolint:testpackage // the internal tests use it

// raceEnabled reports a race detector build; see lzx_race_test.go.
const raceEnabled = false
