package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unicode/utf16"
)

var resultsDir = "results"

func md5sum(b []byte) []byte { s := md5.Sum(b); return s[:] }

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func shaFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return "ERR " + err.Error()
	}
	return sha(b)
}

type caseLog struct {
	id, variant string
	dir         string
	lines       []string
}

func newCase(id, variant string) *caseLog {
	d := filepath.Join(resultsDir, id, variant)
	os.MkdirAll(d, 0o755)
	c := &caseLog{id: id, variant: variant, dir: d}
	c.logf("# %s / %s", id, variant)
	return c
}

func (c *caseLog) path(name string) string { return filepath.Join(c.dir, name) }

func (c *caseLog) logf(format string, a ...any) {
	s := fmt.Sprintf(format, a...)
	fmt.Println(s)
	c.lines = append(c.lines, s)
}

func (c *caseLog) close() {
	os.WriteFile(c.path("log.md"), []byte(strings.Join(c.lines, "\n")+"\n"), 0o644)
}

type msiexecResult struct {
	Exit int
	Log  string
	Hits []string
}

var logPatterns = regexp.MustCompile(`(?i)(Note: 1: 2\d\d\d[^\r\n]*|Return value 3[^\r\n]*|MainEngineThread is returning \d+|Error \d{4}[^\r\n]{0,160}|Disallowing installation of component[^\r\n]*|Product: [^\r\n]{0,200}(succeeded|failed|completed)[^\r\n]*)`)

// Notes every clean install logs: missing optional tables (2205/2228),
// missing optional streams (2262), the in-progress file (2203) and UI fonts (2898).
var benignNote = regexp.MustCompile(`^Note: 1: (2205|2228|2262|2898|2203 2: [^ ]*inprogressinstallinfo)`)

func runMsiexec(c *caseLog, logName string, args ...string) msiexecResult {
	lp := c.path(logName)
	full := append(args, "/qn", "/norestart", "/L*V", lp)
	cmd := exec.Command("msiexec.exe", full...)
	err := cmd.Run()
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			exit = -1
		}
	}
	r := msiexecResult{Exit: exit}
	if b, err := os.ReadFile(lp); err == nil {
		r.Log = decodeLog(b)
		seen := map[string]bool{}
		for _, m := range logPatterns.FindAllString(r.Log, -1) {
			m = strings.TrimSpace(m)
			if benignNote.MatchString(m) {
				continue
			}
			if !seen[m] {
				seen[m] = true
				r.Hits = append(r.Hits, m)
			}
		}
	}
	c.logf("msiexec %s -> exit %d", strings.Join(args, " "), exit)
	for i, h := range r.Hits {
		if i >= 12 {
			c.logf("  ... %d more log hits", len(r.Hits)-i)
			break
		}
		c.logf("  log: %s", h)
	}
	return r
}

func decodeLog(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		u := make([]uint16, (len(b)-2)/2)
		for i := range u {
			u[i] = uint16(b[2+2*i]) | uint16(b[3+2*i])<<8
		}
		return string(utf16.Decode(u))
	}
	return string(b)
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func pf64() string { return os.Getenv("ProgramW6432") }
func pf32() string { return os.Getenv("ProgramFiles(x86)") }

func regQuery(key string) string {
	out, _ := exec.Command("reg.exe", "query", key).CombinedOutput()
	return string(out)
}

func cmdOut(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	out, _ := cmd.CombinedOutput()
	return string(out)
}
