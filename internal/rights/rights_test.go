package rights_test

import (
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/rights"
)

func TestMask(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		want    string
		rights  []string
		mask    uint32
		service bool
	}{
		{rights: []string{"full"}, mask: 0x1F01FF},
		{rights: []string{"readExecute", "0x10000"}, mask: 0x1300A9},
		{rights: []string{"start", "stop"}, mask: 0x30, service: true},
		{rights: []string{"read"}, mask: 0x2008D, service: true},
		{rights: []string{"0X1F"}, mask: 0x1F},
		{rights: []string{"start"}, want: `unknown right "start"`},
		{rights: []string{"modify"}, service: true, want: `unknown right "modify"`},
		{rights: []string{"0x10000000"}, want: "generic"},
		{rights: []string{"0x0"}, want: "non-zero"},
		{rights: []string{"0xZZ"}, want: "non-zero"},
		{want: "empty"},
	} {
		mask, err := rights.Mask(tc.rights, tc.service)

		switch {
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%v: err %v, want %q", tc.rights, err, tc.want)
		case tc.want == "" && (err != nil || mask != tc.mask):
			t.Errorf("%v: %#x, %v, want %#x", tc.rights, mask, err, tc.mask)
		}
	}
}
