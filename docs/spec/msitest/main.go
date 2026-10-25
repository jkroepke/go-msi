package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var cases = map[string]func(){}

func main() {
	if len(os.Args) < 2 {
		var ks []string
		for k := range cases {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		fmt.Println("cases:", strings.Join(ks, " "))
		return
	}
	if d := os.Getenv("RESULTS"); d != "" {
		resultsDir = d
	}
	for _, a := range os.Args[1:] {
		f, ok := cases[a]
		if !ok {
			fmt.Println("unknown case", a)
			os.Exit(2)
		}
		f()
	}
}

// control builds the known-good control package into dir/name.
func control(c *caseLog, p *pkg, name string) string {
	out, _ := filepath.Abs(c.path(name))
	if err := p.build(out); err != nil {
		c.logf("BUILD FAILED: %v", err)
		return ""
	}
	c.logf("built %s sha256 %s", name, shaFile(out))
	return out
}

// installCycle installs, verifies files, uninstalls, verifies removal.
func installCycle(c *caseLog, msi string, files map[string][]byte) (inst, uninst msiexecResult, ok bool) {
	inst = runMsiexec(c, "install.log", "/i", msi)
	ok = inst.Exit == 0
	for path, want := range files {
		got, err := os.ReadFile(path)
		switch {
		case err != nil:
			c.logf("  installed file %s: MISSING", path)
			ok = false
		case string(got) != string(want):
			c.logf("  installed file %s: CONTENT MISMATCH (sha %s)", path, sha(got))
			ok = false
		default:
			c.logf("  installed file %s: ok sha %s", path, sha(got))
		}
	}
	uninst = runMsiexec(c, "uninstall.log", "/x", msi)
	for path := range files {
		if exists(path) {
			c.logf("  after uninstall %s still present", path)
			ok = false
		}
	}
	if uninst.Exit != 0 {
		ok = false
	}
	return
}
