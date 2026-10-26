//go:build !race

package lzx //nolint:testpackage // shared with the internal tests

// raceEnabled reports a race detector build; see lzx_race_test.go.
const raceEnabled = false
