// Command testpe writes a synthetic PE32+ executable with a version resource,
// for tests and CI jobs that need an input file but no real program.
//
//	go run ./internal/testpe/cmd -o hello.exe [-version 1.0.0.0] [-lang 1033]
package main

import (
	"flag"
	"os"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

func main() {
	out := flag.String("o", "hello.exe", "output file")
	ver := flag.String("version", "1.0.0.0", "file and product version, or \"\" for none")
	lang := flag.Int("lang", 1033, "version resource language")

	flag.Parse()

	var langs []int
	if *lang > 0 {
		langs = []int{*lang}
	}

	data := testpe.Build(testpe.Options{FileVersion: *ver, Languages: langs})

	if err := os.WriteFile(*out, data, 0o755); err != nil { //nolint:gosec // an executable
		panic(err)
	}
}
