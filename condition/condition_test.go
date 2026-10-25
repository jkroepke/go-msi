package condition

import "testing"

func TestValid(t *testing.T) {
	for _, s := range []string{
		"",
		"  ",
		"Installed",
		"NOT Installed",
		"Installed OR VersionNT64",
		"Installed AND (NOT REMOVE) AND (NOT UPGRADINGPRODUCTCODE)",
		`EXTRA_FLAGS AND (EXTRA_FLAGS >< "--config.file")`,
		`CONFIG_FILE AND CONFIG_FILE<>"config.yaml"`,
		`LISTEN_ADDR<>"" OR LISTEN_PORT<>9182`,
		"VersionNT>=600",
		"VersionNT >= 600 and not Installed",
		`REMOVE="ALL"`,
		`REMOVE~="all"`,
		"&FirewallException=3 AND NOT !FirewallException=3",
		"$Executable=3",
		"?Executable=2",
		"%PROCESSOR_ARCHITECTURE=\"AMD64\"",
		"1",
		"-1",
		"\"0\"",
		"a IMP b EQV c XOR d OR e AND f",
		"((a))",
		"a.b = 1.1",
		"a >> \"x\" AND b << \"y\"",
		"NOT NOT a",
	} {
		if err := Check(s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
}

func TestInvalid(t *testing.T) {
	for _, s := range []string{
		"0 >=",
		"~not",
		"0 < > 0",
		"==",
		"IMP",
		"!=",
		"(a",
		"a)",
		`"unterminated`,
		"a AND",
		"AND a",
		"a b",
		"a = = b",
		"%",
		"a ~ b",
		"-",
		"a ; b",
	} {
		if err := Check(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestProperties(t *testing.T) {
	got := Properties(`Installed OR (NOT WIX AND &F=3 AND %ENV="x" AND LISTEN_PORT<>9182)`)
	want := []string{"Installed", "WIX", "LISTEN_PORT"}

	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%v", got)
		}
	}
}
