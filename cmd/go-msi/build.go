package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jkroepke/go-msi/pkg/builder"
	"github.com/jkroepke/go-msi/pkg/config"
)

// vars collects repeated --var NAME=VALUE flags.
type vars map[string]string

func (v vars) String() string { return "" }

func (v vars) Type() string { return "NAME=VALUE" }

func (v vars) Set(s string) error {
	name, value, ok := strings.Cut(s, "=")
	if !ok || name == "" {
		return fmt.Errorf("want NAME=VALUE, got %q", s)
	}

	v[name] = value

	return nil
}

// loadFlags are the flags shared by build and validate.
type loadFlags struct {
	vars vars
	env  bool
}

func (l *loadFlags) register(cmd *cobra.Command) {
	l.vars = make(vars)
	cmd.Flags().Var(l.vars, "var", "set a variable referenced as ${NAME}; repeatable")
	cmd.Flags().BoolVar(&l.env, "env", false, "let environment variables fill ${NAME} references")
	mustComplete(cmd.RegisterFlagCompletionFunc("var", cobra.NoFileCompletions))
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

func newValidateCmd() *cobra.Command {
	var load loadFlags

	cmd := &cobra.Command{
		Use:   "validate [flags] msi.yaml",
		Short: "Check a YAML file without building a package",
		Long: `Validate loads msi.yaml, checks it against the schema and runs the
configuration checks of a build. It reads no input files: checks that need
them, such as PE versions, run only in build. Diagnostics go to stderr; the
exit code is 1 when there is an error.`,
		Example:           "  go-msi validate --var VERSION=1.2.3 msi.yaml",
		Args:              exactlyOne("msi.yaml"),
		ValidArgsFunction: fileArg("yaml", "yml"),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := load.load(args[0])
			if err != nil {
				return err
			}

			ds := config.Validate(cfg)
			printDiagnostics(cmd.ErrOrStderr(), ds)

			if err := ds.Err(); err != nil {
				return errors.New("validation failed")
			}

			return nil
		},
	}

	load.register(cmd)

	return cmd
}

// buildFlags are the flags of build.
type buildFlags struct {
	load       loadFlags
	signing    signFlags
	out        string
	cfbVersion int
	skipVerify bool
	quiet      bool
}

func newBuildCmd() *cobra.Command {
	var flags buildFlags

	cmd := &cobra.Command{
		Use:   "build [flags] msi.yaml",
		Short: "Build a package from a YAML file",
		Long: `Build writes the package that msi.yaml describes. Relative paths in the
YAML file resolve against its directory. The package is read back and
compared with its sources unless --skip-verify is given, and it is signed
when a key is given (see the signing flags and docs/signing.md).

Diagnostics go to stderr; on success one summary line goes to stdout.`,
		Example: `  go-msi build -o hello.msi msi.yaml
  go-msi build --var VERSION=1.2.3 --var ARCH=x64 msi.yaml
  go-msi build --pfx codesign.pfx --password-env PFX_PASSWORD \
    --timestamp-url http://timestamp.digicert.com -o hello.msi msi.yaml`,
		Args:              exactlyOne("msi.yaml"),
		ValidArgsFunction: fileArg("yaml", "yml"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return flags.run(cmd, args[0])
		},
	}

	flags.load.register(cmd)
	cmd.Flags().StringVarP(&flags.out, "output", "o", "", "output file (default: <product name>-<version>.msi next to the YAML file)")
	cmd.Flags().IntVar(&flags.cfbVersion, "cfb", 4, "compound file version, 3 (512-byte sectors) or 4 (4096-byte sectors)")
	cmd.Flags().BoolVar(&flags.skipVerify, "skip-verify", false, "do not read the finished package back")
	cmd.Flags().BoolVarP(&flags.quiet, "quiet", "q", false, "print nothing on success")
	flags.signing.register(cmd)

	mustComplete(cmd.MarkFlagFilename("output", "msi"))
	mustComplete(cmd.RegisterFlagCompletionFunc("cfb", fixedValues("3", "4")))

	return cmd
}

func (f *buildFlags) run(cmd *cobra.Command, path string) error {
	if err := f.check(); err != nil {
		return err
	}

	cfg, err := f.load.load(path)
	if err != nil {
		return err
	}

	out := f.output(cfg)

	// Signing works on the whole package, so a signed build streams to a
	// temporary file first and the signed bytes replace out atomically; a
	// package already at out survives a failed signing.
	target := out

	if f.signing.enabled() {
		if target, err = tempPackage(out); err != nil {
			return err
		}

		defer func() { _ = os.Remove(target) }()
	}

	res, err := builder.Build(cfg, builder.Options{
		CFBVersion: f.cfbVersion,
		ToolName:   "go-msi " + version,
		SkipVerify: f.skipVerify,
		Output:     target,
	})

	printDiagnostics(cmd.ErrOrStderr(), res.Diagnostics)

	if err != nil {
		if res.Diagnostics.Err() != nil {
			return errors.New("build failed")
		}

		return fmt.Errorf("build failed: %w", err)
	}

	if f.signing.enabled() {
		if res.Size, err = f.signInto(cmd.Context(), target, out, cfg.Product.Name); err != nil {
			return err
		}
	}

	if !f.quiet {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %d bytes, %d files, ProductCode %s, PackageCode %s\n",
			out, res.Size, len(res.Files), res.ProductCode, res.PackageCode)
	}

	return nil
}

// check refuses flag values before any file is read.
func (f *buildFlags) check() error {
	if f.cfbVersion != 3 && f.cfbVersion != 4 {
		return usageError{fmt.Errorf("--cfb: want 3 or 4, got %d", f.cfbVersion)}
	}

	return f.signing.check()
}

// output is the package path: --output, or <name>-<version>.msi next to
// the YAML file.
func (f *buildFlags) output(cfg *config.Config) string {
	if f.out != "" {
		return f.out
	}

	return filepath.Join(cfg.BaseDir, cfg.Product.Name+"-"+cfg.Product.Version+".msi")
}

// tempPackage creates an empty temporary file next to out.
func tempPackage(out string) (string, error) {
	tmp, err := os.CreateTemp(filepath.Dir(out), ".go-msi-*.unsigned")
	if err != nil {
		return "", fmt.Errorf("create a temporary file: %w", err)
	}

	_ = tmp.Close()

	return tmp.Name(), nil
}

// signInto signs the unsigned package at target, writes the result to out and
// returns its size.
func (f *buildFlags) signInto(ctx context.Context, target, out, product string) (int64, error) {
	pkg, err := os.ReadFile(target)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", target, err)
	}

	signed, err := f.signing.signBuild(ctx, pkg, product)
	if err != nil {
		return 0, err
	}

	if err := writeAtomic(out, signed); err != nil {
		return 0, err
	}

	return int64(len(signed)), nil
}
