package config_test

import (
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/pkg/config"
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
		{sddl: "D:(A;;FA;;;S-1-5-21-1-2-3-500)", warn: true},
		{sddl: "D:(A;;FA;;;<BUILTIN\\Users>)", warn: true},
		{sddl: "D:(A;;FA;;;<[%USERDOMAIN]\\[%USERNAME]>)", warn: true},
		{sddl: "D:(A;;KA;;;BA)S:(AU;SAFA;FA;;;WD)"},
		{sddl: "D:(A;;FA;;;BA)(D;;FW;;;BU)", warn: true},
		// The engine runs as SYSTEM: a DACL without SYSTEM or Administrators
		// locked a folder against its own installer.
		{sddl: "D:AI(A;OICI;FA;;;NS)", warn: true},
		{sddl: "D:(A;;FA;;;<NT AUTHORITY\\NETWORK SERVICE>)", warn: true},
		{sddl: "D:(A;OICI;FA;;;<NT AUTHORITY\\SYSTEM>)(A;OICI;FA;;;NS)"},
		{sddl: "D:(A;OICI;FA;;;<Administrators>)"},
		{sddl: "D:(A;;FA;;;S-1-5-32-544)"},
		{sddl: "D:NO_ACCESS_CONTROL"},
		{sddl: "O:SYG:SY"},
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
		// Refused by ConvertStringSecurityDescriptorToSecurityDescriptorW
		// (FuzzCheckSDDLNative on windows-latest).
		{sddl: "D:(A;;FA;;;BA)D:(A;;FA;;;BA)", wantErr: "twice"},
		{sddl: "D:(A;;FA;x;;BA)", wantErr: "GUID"},
		{sddl: "D:(OA;;CR;x;;BA)", wantErr: "not a GUID"},
		{sddl: "D:(A;;FA;ab721a53-1e2f-11d0-9819-00aa0040529b;;BA)", wantErr: "no object GUID"},
		{sddl: "D:(A;;FA;;;BA;x)", wantErr: "resource attribute"},
		{sddl: "S:(RA;;FA;;;BA;x)", wantErr: "resource attribute"},
		{sddl: "D:(XA;;FX;;;WD)", wantErr: "condition"},
		{sddl: "S:(FL;;FA;;;BA)", wantErr: "condition"},
		{sddl: "S:(RA;;FA;;;BA)", wantErr: "resource attribute"},
		{sddl: "D:(A;;FA;;;S-1-5-21-1-2-3-4-5-6-7-8-9-10-11-12-13-14)", warn: true},
		{sddl: "D:(A;;FA;;;S-1-5-21-1-2-3-4-5-6-7-8-9-10-11-12-13-14-15)", wantErr: "not a SID"},
		{sddl: "D:(A;;FA;;;S-1-5-4294967296)", wantErr: "not a SID"},
		{sddl: "D:(A;;FA;;;S-1-281474976710656-1)", wantErr: "not a SID"},
		{sddl: "D:(OA;;CR;ab721a53-1e2f-11d0-9819-00aa0040529b;;BA)", warn: true},
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
