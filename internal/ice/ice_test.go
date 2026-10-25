package ice_test

import (
	"reflect"
	"testing"

	"github.com/jkroepke/go-msi/internal/ice"
)

func TestParse(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		msg  string
		want ice.Finding
		ok   bool
	}{
		{
			"ICE08\t1\tComponent: B has a duplicate GUID: A\thttp://help\tComponent\tComponentId\tB\r\n",
			ice.Finding{ICE: "ICE08", Type: ice.Error, Description: "Component: B has a duplicate GUID: A", HelpURL: "http://help", Table: "Component", Column: "ComponentId", Keys: []string{"B"}},
			true,
		},
		{"ICE00\t3\tICE00 - Created 01/01/2000. Last Modified 01/01/2000.", ice.Finding{ICE: "ICE00", Type: ice.Info, Description: "ICE00 - Created 01/01/2000. Last Modified 01/01/2000."}, true},
		{"ICE33\t2\tReg key\t\tRegistry\tRegistry\tk1\tk2", ice.Finding{ICE: "ICE33", Type: ice.Warning, Description: "Reg key", Table: "Registry", Column: "Registry", Keys: []string{"k1", "k2"}}, true},
		{"Action start 12:00:00: ICE08.", ice.Finding{}, false},
		{"ICE08\tx\tno type", ice.Finding{}, false},
		{"", ice.Finding{}, false},
	} {
		got, ok := ice.Parse(tc.msg)
		if ok != tc.ok || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Parse(%q) = %#v, %v; want %#v, %v", tc.msg, got, ok, tc.want, tc.ok)
		}
	}
}

func TestFailed(t *testing.T) {
	t.Parallel()

	findings := []ice.Finding{{ICE: "a", Type: ice.Info}, {ICE: "b", Type: ice.Error}, {ICE: "c", Type: ice.Warning}, {ICE: "d", Type: ice.Failure}}

	got := ice.Failed(findings)
	if len(got) != 2 || got[0].ICE != "b" || got[1].ICE != "d" {
		t.Errorf("Failed = %v", got)
	}

	if s := findings[1].String(); s != "b error: " {
		t.Errorf("String = %q", s)
	}
}
