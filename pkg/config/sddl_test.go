package config_test

import (
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/pkg/config"
)

func TestCheckSDDL(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		sddl    string
		wantErr string
		warn    bool
	}{
		{sddl: "D:PAI(A;;FA;;;BA)(A;;FRFX;;;BU)"},
		{sddl: "O:BAG:BAD:(A;OICI;FA;;;SY)(A;OICI;0x1200a9;;;BU)"},
		{sddl: "D:(A;;FA;;;S-1-5-21-1-2-3-500)"},
		{sddl: "D:(A;;FA;;;<BUILTIN\\Users>)"},
		{sddl: "D:(A;;FA;;;<[%USERDOMAIN]\\[%USERNAME]>)"},
		{sddl: "D:(A;;KA;;;BA)S:(AU;SAFA;FA;;;WD)"},
		{sddl: "D:(A;;FA;;;BA)(D;;FW;;;BU)", warn: true},
		{sddl: "", wantErr: "empty"},
		{sddl: "D:(A;;FA;;;[PERMSID])", wantErr: "1943"},
		{sddl: "D:(A;;FA;;;<[PERMSID]>)", wantErr: "<account>"},
		{sddl: "X:(A;;FA;;;BA)", wantErr: "expected O:"},
		{sddl: "D:(Q;;FA;;;BA)", wantErr: "ACE type"},
		{sddl: "D:(A;;FZ;;;BA)", wantErr: "rights"},
		{sddl: "D:(A;;FA;;;ZZ)", wantErr: "not a SID"},
		{sddl: "D:(A;;FA;;BA)", wantErr: "fields"},
		{sddl: "D:Q(A;;FA;;;BA)", wantErr: "ACL flags"},
		{sddl: "O:XX", wantErr: "not a SID"},
	} {
		warnings, err := config.CheckSDDL(tc.sddl)

		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%q: %v", tc.sddl, err)
		case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
			t.Errorf("%q: error %v, want %q", tc.sddl, err, tc.wantErr)
		case tc.warn != (len(warnings) > 0):
			t.Errorf("%q: warnings %v", tc.sddl, warnings)
		}
	}
}
