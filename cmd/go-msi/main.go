// Command go-msi builds Windows Installer packages from a YAML description.
//
//	go-msi build [-o out.msi] [--var NAME=VALUE]... [--env] msi.yaml
//	go-msi validate [--var NAME=VALUE]... [--env] msi.yaml
//	go-msi inspect [--table NAME] package.msi
//	go-msi sign [signing options] [-o out.msi] package.msi
//	go-msi verify package.msi
//	go-msi version
package main

import (
	"context"
	"crypto"
	"crypto/x509"
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

	"github.com/jkroepke/go-msi/pkg/builder"
	"github.com/jkroepke/go-msi/pkg/config"
	"github.com/jkroepke/go-msi/pkg/msidb"
	"github.com/jkroepke/go-msi/pkg/sign"
)

// Set by the linker at release time.
var (
	version = "dev"
	commit  = "none"    //nolint:gochecknoglobals // set by the linker
	date    = "unknown" //nolint:gochecknoglobals // set by the linker
)

const usage = `usage: go-msi <command> [options]

commands:
  build     a package:   go-msi build [-o out.msi] [--var NAME=VALUE]... [--env] [--cfb 3|4] [signing options] msi.yaml
  validate  check a file:      go-msi validate [--var NAME=VALUE]... [--env] msi.yaml
  inspect   dump a package:    go-msi inspect [--table NAME] package.msi
  sign      add a signature:   go-msi sign [-o out.msi] [signing options] package.msi
  verify    check a signature: go-msi verify package.msi
  version   print the version

signing options:
  --pfx FILE | --cert FILE --key FILE   key and certificate chain
  --password-env NAME                   environment variable with the .pfx password
  --hash sha256|sha384|sha512|sha1      digest algorithm (default sha256)
  --timestamp-url URL                   RFC 3161 time-stamping authority
  --description TEXT, --url URL         program name and link shown by Windows
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command line and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int { //nolint:cyclop // one case per command
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
		err = cmdInspect(args[1:], stdout, stderr)
	case "sign":
		err = cmdSign(args[1:], stdout, stderr)
	case "verify":
		err = cmdVerify(args[1:], stdout, stderr)
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

	cfg, err := config.Load(path, opts)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}

	return cfg, nil
}

func printDiagnostics(w io.Writer, ds config.Diagnostics) {
	for _, d := range ds {
		_, _ = fmt.Fprintln(w, d.String())
	}
}

func cmdValidate(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var load loadFlags

	load.register(flags)

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("flags: %w", err)
	}

	if flags.NArg() != 1 {
		return errors.New("validate wants exactly one msi.yaml")
	}

	cfg, err := load.load(flags.Arg(0))
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
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		load       loadFlags
		out        string
		cfbVersion int
		skipVerify bool
		quiet      bool
		signing    signFlags
	)

	load.register(flags)
	signing.register(flags)
	flags.StringVar(&out, "o", "", "output file (default: <product name>-<version>.msi next to the YAML file)")
	flags.IntVar(&cfbVersion, "cfb", 4, "compound file version, 3 (512-byte sectors) or 4 (4096-byte sectors)")
	flags.BoolVar(&skipVerify, "skip-verify", false, "do not read the finished package back")
	flags.BoolVar(&quiet, "q", false, "print nothing on success")

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("flags: %w", err)
	}

	if flags.NArg() != 1 {
		return errors.New("build wants exactly one msi.yaml")
	}

	if err := signing.check(); err != nil {
		return err
	}

	cfg, err := load.load(flags.Arg(0))
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

	if res.Package, err = signing.signBuild(res.Package, cfg.Product.Name); err != nil {
		return err
	}

	if err := os.WriteFile(out, res.Package, 0o644); err != nil { //nolint:gosec // a package is world-readable
		return fmt.Errorf("write %s: %w", out, err)
	}

	if !quiet {
		_, _ = fmt.Fprintf(stdout, "%s: %d bytes, %d files, ProductCode %s, PackageCode %s\n",
			out, len(res.Package), len(res.Files), res.ProductCode, res.PackageCode)
	}

	return nil
}

func cmdInspect(args []string, stdout, stderr io.Writer) error { //nolint:cyclop // one pass over the format; splitting would scatter it
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var table string

	flags.StringVar(&table, "table", "", "print only this table")

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("flags: %w", err)
	}

	if flags.NArg() != 1 {
		return errors.New("inspect wants exactly one package")
	}

	data, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	database, err := msidb.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("%s: %w", flags.Arg(0), err)
	}

	if table != "" {
		t := database.Table(table)
		if t == nil {
			return fmt.Errorf("no table %s", table)
		}

		printTable(stdout, t)

		return nil
	}

	_, _ = fmt.Fprintf(stdout, "codepage: %d\n", database.Codepage)

	for _, p := range database.Summary {
		_, _ = fmt.Fprintf(stdout, "summary %2d: %s\n", p.ID, formatValue(p.Value))
	}

	names := make([]string, 0, len(database.Streams))
	for _, s := range database.Streams {
		names = append(names, fmt.Sprintf("%s (%d bytes)", s.Name, len(s.Data)))
	}

	slices.Sort(names)

	if len(names) > 0 {
		_, _ = fmt.Fprintf(stdout, "streams: %s\n", strings.Join(names, ", "))
	}

	for _, p := range database.Problems {
		_, _ = fmt.Fprintf(stdout, "problem: %s\n", p)
	}

	for _, t := range database.Tables {
		_, _ = fmt.Fprintln(stdout)
		printTable(stdout, t)
	}

	return nil
}

func printTable(out io.Writer, tbl *msidb.Table) {
	cols := make([]string, len(tbl.Columns))
	types := make([]string, len(tbl.Columns))

	for i, c := range tbl.Columns {
		cols[i] = c.Name
		types[i] = c.Type.String()

		if c.Type.Key {
			types[i] += "!"
		}
	}

	_, _ = fmt.Fprintf(out, "== %s (%d rows)\n%s\n%s\n", tbl.Name, len(tbl.Rows), strings.Join(cols, "\t"), strings.Join(types, "\t"))

	for _, r := range tbl.Rows {
		cells := make([]string, len(r))
		for i, v := range r {
			cells[i] = formatValue(v)
		}

		_, _ = fmt.Fprintln(out, strings.Join(cells, "\t"))
	}
}

func formatValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case []byte:
		return fmt.Sprintf("[binary %d bytes]", len(val))
	case time.Time:
		return val.UTC().Format(time.RFC3339)
	default:
		return fmt.Sprint(val)
	}
}

// signFlags are the signing options of build and sign.
type signFlags struct {
	pfx          string
	cert         string
	key          string
	passwordEnv  string
	hash         string
	timestampURL string
	description  string
	url          string
}

func (s *signFlags) register(flags *flag.FlagSet) {
	flags.StringVar(&s.pfx, "pfx", "", "sign with the key and certificates in this PKCS #12 file")
	flags.StringVar(&s.cert, "cert", "", "sign with this PEM certificate chain (signing certificate first)")
	flags.StringVar(&s.key, "key", "", "PEM private key for --cert")
	flags.StringVar(&s.passwordEnv, "password-env", "", "environment variable holding the --pfx password")
	flags.StringVar(&s.hash, "hash", "sha256", "signature digest: sha256, sha384, sha512 or sha1")
	flags.StringVar(&s.timestampURL, "timestamp-url", "", "RFC 3161 time-stamping authority")
	flags.StringVar(&s.description, "description", "", "program name shown in the UAC prompt")
	flags.StringVar(&s.url, "url", "", "more-information link of the signature")
}

func (s *signFlags) enabled() bool { return s.pfx != "" || s.cert != "" || s.key != "" }

// check refuses signing options that would be ignored.
func (s *signFlags) check() error {
	if !s.enabled() && (s.timestampURL != "" || s.description != "" || s.url != "" || s.passwordEnv != "" || s.hash != "sha256") {
		return errors.New("signing options need --pfx, or --cert and --key")
	}

	if s.passwordEnv != "" && s.pfx == "" {
		return errors.New("--password-env only applies to --pfx")
	}

	return nil
}

func (s *signFlags) options() (sign.Options, error) {
	opts := sign.Options{TimestampURL: s.timestampURL, Description: s.description, URL: s.url}

	hashes := map[string]crypto.Hash{"sha256": crypto.SHA256, "sha384": crypto.SHA384, "sha512": crypto.SHA512, "sha1": crypto.SHA1}

	var ok bool
	if opts.Hash, ok = hashes[s.hash]; !ok {
		return opts, fmt.Errorf("--hash: unknown digest %q", s.hash)
	}

	var err error

	opts.Signer, opts.Certificates, err = s.loadKey()

	return opts, err
}

func (s *signFlags) loadKey() (crypto.Signer, []*x509.Certificate, error) {
	switch {
	case s.pfx != "" && (s.cert != "" || s.key != ""):
		return nil, nil, errors.New("use either --pfx or --cert and --key")
	case s.pfx != "":
		return s.loadPFX()
	case s.cert != "" && s.key != "":
		certPEM, err := os.ReadFile(s.cert)
		if err != nil {
			return nil, nil, fmt.Errorf("read: %w", err)
		}

		keyPEM, err := os.ReadFile(s.key)
		if err != nil {
			return nil, nil, fmt.Errorf("read: %w", err)
		}

		return sign.LoadPEM(certPEM, keyPEM) //nolint:wrapcheck // sign errors carry their prefix
	default:
		return nil, nil, errors.New("signing needs --pfx, or --cert and --key")
	}
}

func (s *signFlags) loadPFX() (crypto.Signer, []*x509.Certificate, error) {
	data, err := os.ReadFile(s.pfx)
	if err != nil {
		return nil, nil, fmt.Errorf("read: %w", err)
	}

	var password string

	if s.passwordEnv != "" {
		var ok bool
		if password, ok = os.LookupEnv(s.passwordEnv); !ok {
			return nil, nil, fmt.Errorf("--password-env: %s is not set", s.passwordEnv)
		}
	}

	return sign.LoadPKCS12(data, password) //nolint:wrapcheck // sign errors carry their prefix
}

// signBuild signs a freshly built package when signing was asked for. The
// program name defaults to the product name.
func (s *signFlags) signBuild(pkg []byte, product string) ([]byte, error) {
	if !s.enabled() {
		return pkg, nil
	}

	if s.description == "" {
		s.description = product
	}

	return s.sign(pkg)
}

func (s *signFlags) sign(pkg []byte) ([]byte, error) {
	opts, err := s.options()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	signed, err := sign.Sign(ctx, pkg, opts)
	if err != nil {
		return nil, err //nolint:wrapcheck // sign errors carry their prefix
	}

	return signed, nil
}

func cmdSign(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("sign", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		signing signFlags
		out     string
		quiet   bool
	)

	signing.register(flags)
	flags.StringVar(&out, "o", "", "output file (default: sign in place)")
	flags.BoolVar(&quiet, "q", false, "print nothing on success")

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("flags: %w", err)
	}

	if flags.NArg() != 1 {
		return errors.New("sign wants exactly one package")
	}

	if err := signing.check(); err != nil {
		return err
	}

	in := flags.Arg(0)
	if out == "" {
		out = in
	}

	data, err := os.ReadFile(in)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	signed, err := signing.sign(data)
	if err != nil {
		return err
	}

	if err := writeAtomic(out, signed); err != nil {
		return err
	}

	if !quiet {
		return printSignature(stdout, out, signed)
	}

	return nil
}

// writeAtomic replaces path through a temporary file in the same
// directory, so a failed write keeps the original package. An existing
// file keeps its mode, and a symlink keeps pointing at the replaced file.
func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)

	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target

		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".go-msi-*.msi")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	defer func() { _ = os.Remove(tmp.Name()) }()

	for _, step := range []func() error{
		func() error {
			_, err := tmp.Write(data)

			return err //nolint:wrapcheck // wrapped below
		},
		func() error { return tmp.Chmod(mode) },
		tmp.Sync,
	} {
		if err := step(); err != nil {
			_ = tmp.Close()

			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func cmdVerify(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(stderr)

	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("flags: %w", err)
	}

	if flags.NArg() != 1 {
		return errors.New("verify wants exactly one package")
	}

	data, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	return printSignature(stdout, flags.Arg(0), data)
}

// printSignature verifies pkg and prints a one-line summary.
func printSignature(stdout io.Writer, name string, pkg []byte) error {
	sig, err := sign.Verify(pkg)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	mode := "basic"
	if sig.Extended {
		mode = "extended"
	}

	stamp := "no timestamp"
	if !sig.Timestamp.IsZero() {
		stamp = "timestamped " + sig.Timestamp.UTC().Format(time.RFC3339)
	}

	_, _ = fmt.Fprintf(stdout, "%s: signature valid (%s, %v, %s), signer %q, issuer %q\n",
		name, mode, sig.Hash, stamp, sig.Signer.Subject.String(), sig.Signer.Issuer.String())

	return nil
}
