// Package ice runs Internal Consistency Evaluators (ICEs) against a package
// with msi.dll, without MsiVal2 or Orca. It merges a validation file such as
// Darice.cub into a copy of the package, opens that copy as an install session,
// runs every action listed in the merged
// _ICESequence table and collects the messages the actions post.
//
// It only exists for tests: go-msi never runs the packages it builds. Run
// works on Windows only; elsewhere it returns ErrUnsupported.
package ice

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrUnsupported is returned by Run outside Windows.
var ErrUnsupported = errors.New("ice: needs msi.dll (Windows only)")

// Type is the message type an ICE posts in its second field.
type Type int

// Message types, as the evaluation engine documents them.
const (
	Failure Type = 0 // the ICE itself failed
	Error   Type = 1
	Warning Type = 2
	Info    Type = 3
)

func (t Type) String() string {
	switch t {
	case Failure:
		return "failure"
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Info:
		return "info"
	default:
		return "type " + strconv.Itoa(int(t))
	}
}

// Finding is one ICE message.
type Finding struct {
	ICE         string
	Description string
	HelpURL     string
	Table       string
	Column      string
	Keys        []string
	Type        Type
}

func (f Finding) String() string {
	var text strings.Builder

	fmt.Fprintf(&text, "%s %s: %s", f.ICE, f.Type, f.Description)

	if f.Table != "" {
		fmt.Fprintf(&text, " [%s", f.Table)

		if f.Column != "" {
			text.WriteString("." + f.Column)
		}

		if len(f.Keys) > 0 {
			text.WriteString(" " + strings.Join(f.Keys, "/"))
		}

		text.WriteString("]")
	}

	return text.String()
}

// Parse decodes a message of the form
// ICE<TAB>type<TAB>description<TAB>help URL<TAB>table<TAB>column<TAB>key…
// It reports false for text that is not an ICE message.
func Parse(msg string) (Finding, bool) {
	fields := strings.Split(strings.TrimRight(msg, "\r\n"), "\t")
	if len(fields) < 3 || fields[0] == "" {
		return Finding{}, false
	}

	typ, err := strconv.Atoi(fields[1])
	if err != nil {
		return Finding{}, false
	}

	finding := Finding{ICE: fields[0], Type: Type(typ), Description: fields[2]}

	rest := fields[3:]
	for i, dst := range []*string{&finding.HelpURL, &finding.Table, &finding.Column} {
		if i < len(rest) {
			*dst = rest[i]
		}
	}

	if len(rest) > 3 {
		finding.Keys = rest[3:]
	}

	return finding, true
}

// Failed returns the findings of type Error or Failure.
func Failed(findings []Finding) []Finding {
	var out []Finding

	for _, f := range findings {
		if f.Type == Error || f.Type == Failure {
			out = append(out, f)
		}
	}

	return out
}
