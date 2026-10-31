// Package formatted inspects Windows Installer formatted strings
// (specification section 8.9) for the builder and the configuration
// checks.
package formatted

import "strings"

// Nested reports whether text has a [ inside a [...] reference, such as
// [[NAME]], [A[B]] or [[%ENV]]: the property such a reference names is only
// known at install time. An escape such as [\[] outside a reference is
// text.
func Nested(text string) bool {
	open := false

	for i := 0; i < len(text); i++ {
		switch {
		case !open && strings.HasPrefix(text[i:], `[\`) && len(text) > i+3 && text[i+3] == ']':
			i += 3
		case text[i] == '[':
			if open {
				return true
			}

			open = true
		case text[i] == ']':
			open = false
		}
	}

	return false
}
