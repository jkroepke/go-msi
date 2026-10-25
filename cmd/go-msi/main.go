// Command go-msi builds Windows Installer packages from a YAML description.
//
//	go-msi build [-o out.msi] [--var NAME=VALUE]... [--env] msi.yaml
//	go-msi validate [--var NAME=VALUE]... [--env] msi.yaml
//	go-msi inspect [--table NAME] package.msi
//	go-msi version
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jkroepke/go-msi/builder"
	"github.com/jkroepke/go-msi/config"
	"github.com/jkroepke/go-msi/msidb"
)

// Set by the linker at release time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `usage: go-msi <command> [options]

commands:
  build     a package:   go-msi build [-o out.msi] [--var NAME=VALUE]... [--env] [--cfb 3|4] msi.yaml
  validate  check a file:      go-msi validate [--var NAME=VALUE]... [--env] msi.yaml
  inspect   dump a package:    go-msi inspect [--table NAME] package.msi
  version   print the version
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command line and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, usage)

		return 2
	}

	var err error

	switch args[0] {
	case "build":
		err = cmdBuild(args[1:], stdout, stderr)
	case "validate":
		err = cmdValidate(args[1:], stderr)
	case "inspect":
		err = cmdInspect(args[1:], stdout)
	case "version":
		_, _ = fmt.Fprintf(stdout, "go-msi %s (commit %s, built %s)\n", version, commit, date)
	case "-h", "--help", "help":
		_, _ = io.WriteString(stdout, usage)
	default:
		_, _ = fmt.Fprintf(stderr, "go-msi: unknown command %q\n%s", args[0], usage)

		return 2
	}

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 2
		}

		_, _ = fmt.Fprintf(stderr, "go-msi: %v\n", err)

		return 1
	}

	return 0
}

// vars collects repeated --var NAME=VALUE flags.
type vars map[string]string

func (v vars) String() string { return "" }

func (v vars) Set(s string) error {
	name, value, ok := strings.Cut(s, "=")
	if !ok || name == "" {
		return fmt.Errorf("--var wants NAME=VALUE, got %q", s)
	}

	v[name] = value

	return nil
}

// loadFlags are the flags shared by build and validate.
type loadFlags struct {
	vars vars
	env  bool
}

func (l *loadFlags) register(fs *flag.FlagSet) {
	l.vars = make(vars)
	fs.Var(l.vars, "var", "set a variable (NAME=VALUE); repeatable")
	fs.BoolVar(&l.env, "env", false, "let environment variables fill ${NAME} references")
}

func (l *loadFlags) load(path string) (*config.Config, error) {
	opts := config.Options{Vars: l.vars}
	if l.env {
		opts.LookupEnv = os.LookupEnv
	}

	return config.Load(path, opts)
}

func printDiagnostics(w io.Writer, ds config.Diagnostics) {
	for _, d := range ds {
		_, _ = fmt.Fprintln(w, d.String())
	}
}

func cmdValidate(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var lf loadFlags

	lf.register(fs)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("validate wants exactly one msi.yaml")
	}

	cfg, err := lf.load(fs.Arg(0))
	if err != nil {
		return err
	}

	ds := config.Validate(cfg)
	printDiagnostics(stderr, ds)

	if err := ds.Err(); err != nil {
		return errors.New("validation failed")
	}

	return nil
}

func cmdBuild(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		lf         loadFlags
		out        string
		cfbVersion int
		skipVerify bool
		quiet      bool
	)

	lf.register(fs)
	fs.StringVar(&out, "o", "", "output file (default: <product name>-<version>.msi next to the YAML file)")
	fs.IntVar(&cfbVersion, "cfb", 4, "compound file version, 3 (512-byte sectors) or 4 (4096-byte sectors)")
	fs.BoolVar(&skipVerify, "skip-verify", false, "do not read the finished package back")
	fs.BoolVar(&quiet, "q", false, "print nothing on success")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("build wants exactly one msi.yaml")
	}

	cfg, err := lf.load(fs.Arg(0))
	if err != nil {
		return err
	}

	res, err := builder.Build(cfg, builder.Options{
		CFBVersion: cfbVersion,
		ToolName:   "go-msi " + version,
		SkipVerify: skipVerify,
	})

	printDiagnostics(stderr, res.Diagnostics)

	if err != nil {
		return errors.New("build failed")
	}

	if out == "" {
		out = filepath.Join(cfg.BaseDir, cfg.Product.Name+"-"+cfg.Product.Version+".msi")
	}

	if err := os.WriteFile(out, res.Package, 0o644); err != nil { //nolint:gosec // a package is world-readable
		return err
	}

	if !quiet {
		_, _ = fmt.Fprintf(stdout, "%s: %d bytes, %d files, ProductCode %s, PackageCode %s\n",
			out, len(res.Package), len(res.Files), res.ProductCode, res.PackageCode)
	}

	return nil
}

func cmdInspect(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stdout)

	var table string

	fs.StringVar(&table, "table", "", "print only this table")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("inspect wants exactly one package")
	}

	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}

	db, err := msidb.Unmarshal(data)
	if err != nil {
		return err
	}

	if table != "" {
		t := db.Table(table)
		if t == nil {
			return fmt.Errorf("no table %s", table)
		}

		printTable(stdout, t)

		return nil
	}

	_, _ = fmt.Fprintf(stdout, "codepage: %d\n", db.Codepage)

	for _, p := range db.Summary {
		_, _ = fmt.Fprintf(stdout, "summary %2d: %s\n", p.ID, formatValue(p.Value))
	}

	names := make([]string, 0, len(db.Streams))
	for _, s := range db.Streams {
		names = append(names, fmt.Sprintf("%s (%d bytes)", s.Name, len(s.Data)))
	}

	slices.Sort(names)

	if len(names) > 0 {
		_, _ = fmt.Fprintf(stdout, "streams: %s\n", strings.Join(names, ", "))
	}

	for _, p := range db.Problems {
		_, _ = fmt.Fprintf(stdout, "problem: %s\n", p)
	}

	for _, t := range db.Tables {
		_, _ = fmt.Fprintln(stdout)
		printTable(stdout, t)
	}

	return nil
}

func printTable(w io.Writer, t *msidb.Table) {
	cols := make([]string, len(t.Columns))
	types := make([]string, len(t.Columns))

	for i, c := range t.Columns {
		cols[i] = c.Name
		types[i] = c.Type.String()

		if c.Type.Key {
			types[i] += "!"
		}
	}

	_, _ = fmt.Fprintf(w, "== %s (%d rows)\n%s\n%s\n", t.Name, len(t.Rows), strings.Join(cols, "\t"), strings.Join(types, "\t"))

	for _, r := range t.Rows {
		cells := make([]string, len(r))
		for i, v := range r {
			cells[i] = formatValue(v)
		}

		_, _ = fmt.Fprintln(w, strings.Join(cells, "\t"))
	}
}

func formatValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case []byte:
		return fmt.Sprintf("[binary %d bytes]", len(x))
	case time.Time:
		return x.UTC().Format(time.RFC3339)
	default:
		return fmt.Sprint(x)
	}
}
