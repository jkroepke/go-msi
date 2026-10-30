//go:build windows

package nativetest_test

import (
	"path/filepath"
	"strings"
	"testing"
)

const (
	progressWait    = "Waiting for [ProductName] to settle"
	progressWaitDE  = "Warte auf [ProductName]"
	progressPrepare = "Preparing [ProductName]"
)

// progressConfig is the full UI configuration of languagesConfig with two
// custom actions that have progress texts: an immediate one and a deferred
// run action that waits about five seconds, so the progress page shows its
// text long enough for the driver to read it. The German entry translates
// the deferred one.
func progressConfig(t *testing.T) string {
	t.Helper()

	const actions = "customActions:\n"

	doc := languagesConfig(t)
	if strings.Count(doc, actions) != 1 || strings.Count(doc, "    de:\n") != 1 {
		t.Fatal("the languages configuration no longer holds one customActions and one de entry")
	}

	doc = strings.Replace(doc, actions, actions+
		"  - id: WaitStep\n"+
		"    run: { command: '\"[System64Folder]PING.EXE\" -n 6 127.0.0.1' }\n"+
		"    execute: deferred\n"+
		"    impersonate: false\n"+
		"    return: check\n"+
		"    progress: "+progressWait+"\n"+
		"    schedule:\n"+
		"      - { sequence: execute, before: InstallFinalize, condition: NOT Installed }\n"+
		"  - id: SetMarker\n"+
		"    setProperty: { name: MARKER, value: x }\n"+
		"    progress: "+progressPrepare+"\n"+
		"    schedule:\n"+
		"      - { sequence: execute, after: CostFinalize }\n", 1)

	return strings.Replace(doc, "    de:\n", "    de:\n      progress: { WaitStep: '"+progressWaitDE+"' }\n", 1)
}

// TestMsiexecProgressText installs the progress package quietly, in the
// base language and with the German transform, and reports the log lines
// of the two actions (CA-08): does the engine log the ActionText
// description of an immediate and of a deferred action, is it formatted,
// and does the transform replace it?
func TestMsiexecProgressText(t *testing.T) { //nolint:paralleltest // one msiexec at a time
	if testing.Short() {
		t.Skip("msiexec install")
	}

	waitForMsiexec(t)

	dir := t.TempDir()
	pkg, res := buildFullUIFrom(t, dir, progressConfig(t), "1.0.0", "auto")

	cleanupFullUI(t, pkg)

	var report strings.Builder

	for _, language := range []string{"", "1031"} {
		log := filepath.Join(dir, "install"+language+".log")
		args := []string{"/i", pkg, "/qn", "/l*v", log}

		if language != "" {
			args = append(args, "ProductLanguage="+language)
		}

		if code := msiexec(t, args...); code != 0 {
			t.Fatalf("install %s: exit %d\n%s", language, code, excerpt(readLog(log), "WaitStep", "SetMarker", "Error"))
		}

		text := readLog(log)
		report.WriteString("== install ProductLanguage=" + language + "\n" + excerpt(text, "WaitStep", "SetMarker"))

		if code := msiexec(t, "/x", res.ProductCode, "/qn"); code != 0 {
			t.Errorf("uninstall %s: exit %d", language, code)
		}

		// The deferred action's description goes into the script,
		// formatted when the script is written. The immediate action's
		// is not in a /qn log at all.
		want := "ActionStart(Name=WaitStep,Description=Waiting for " + fullUIName + " to settle,)"
		if language == "1031" {
			want = "ActionStart(Name=WaitStep,Description=Warte auf " + fullUIName + ",)"
		}

		if !strings.Contains(text, want) {
			t.Errorf("CA-08: the log of the install %s has no %q", language, want)
		}

		if strings.Contains(text, "Preparing ") {
			t.Errorf("CA-08: the /qn log of the install %s shows the immediate action's description", language)
		}
	}

	t.Logf("CA-08 log lines:\n%s", report.String())
}

// TestFullUIProgressText runs the wizard of the progress package and
// reports whether the progress page shows the deferred action's text
// while it runs (CA-08).
func TestFullUIProgressText(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	requireFullUI(t)
	waitForMsiexec(t)

	dir := t.TempDir()
	pkg, _ := buildFullUIFrom(t, dir, progressConfig(t), "1.0.0", "auto")

	cleanupFullUI(t, pkg)

	log := filepath.Join(dir, "install.log")

	code, seen := fullUI(t, dir, "/i", pkg, "/l*v", log)
	if code != 0 {
		t.Fatalf("full UI install: exit %d, want 0\nUI:\n%s", code, seen)
	}

	// The progress page formats the text: [ProductName] is resolved.
	if !strings.Contains(seen, "Waiting for "+fullUIName+" to settle") {
		t.Error("CA-08: no window showed the formatted progress text of WaitStep")
	}

	t.Logf("CA-08 UI:\n%s\nlog:\n%s", excerpt(seen, "Waiting", "Preparing"), excerpt(readLog(log), "SetMarker", "WaitStep", "Preparing"))
}
