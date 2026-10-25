//go:build !windows

package ice

// Run is only available on Windows.
func Run(_, _ string) ([]Finding, error) { return nil, ErrUnsupported }
