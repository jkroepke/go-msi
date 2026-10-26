package main

import (
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jkroepke/go-msi/pkg/sign"
)

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

func (s *signFlags) register(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringVar(&s.pfx, "pfx", "", "sign with the key and certificates in this PKCS #12 file")
	flags.StringVar(&s.cert, "cert", "", "sign with this PEM certificate chain (signing certificate first)")
	flags.StringVar(&s.key, "key", "", "PEM private key for --cert")
	flags.StringVar(&s.passwordEnv, "password-env", "", "environment variable holding the --pfx password")
	flags.StringVar(&s.hash, "hash", "sha256", "signature digest: sha256, sha384, sha512 or sha1")
	flags.StringVar(&s.timestampURL, "timestamp-url", "", "RFC 3161 time-stamping authority")
	flags.StringVar(&s.description, "description", "", "program name shown in the UAC prompt")
	flags.StringVar(&s.url, "url", "", "more-information link of the signature")

	mustComplete(cmd.MarkFlagFilename("pfx", "pfx", "p12"))
	mustComplete(cmd.MarkFlagFilename("cert", "pem", "crt", "cer"))
	mustComplete(cmd.MarkFlagFilename("key", "pem", "key"))
	mustComplete(cmd.RegisterFlagCompletionFunc("password-env", completeEnvNames))
	mustComplete(cmd.RegisterFlagCompletionFunc("hash", fixedValues("sha256", "sha384", "sha512", "sha1")))

	for _, name := range []string{"timestamp-url", "description", "url"} {
		mustComplete(cmd.RegisterFlagCompletionFunc(name, cobra.NoFileCompletions))
	}
}

// completeEnvNames offers the names of the environment variables.
func completeEnvNames(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	names := make([]cobra.Completion, 0, len(os.Environ()))

	for _, kv := range os.Environ() {
		if name, _, ok := strings.Cut(kv, "="); ok && name != "" {
			names = append(names, name)
		}
	}

	slices.Sort(names)

	return names, cobra.ShellCompDirectiveNoFileComp
}

func (s *signFlags) enabled() bool { return s.pfx != "" || s.cert != "" || s.key != "" }

// check refuses signing options that would be ignored.
func (s *signFlags) check() error {
	if !s.enabled() && (s.timestampURL != "" || s.description != "" || s.url != "" || s.passwordEnv != "" || s.hash != "sha256") {
		return usageError{errors.New("signing options need --pfx, or --cert and --key")}
	}

	if s.passwordEnv != "" && s.pfx == "" {
		return usageError{errors.New("--password-env only applies to --pfx")}
	}

	return s.checkKey()
}

// checkKey refuses an unknown digest and incomplete or conflicting keys.
func (s *signFlags) checkKey() error {
	if _, ok := signHashes[s.hash]; !ok {
		return usageError{fmt.Errorf("--hash: unknown digest %q", s.hash)}
	}

	switch {
	case !s.enabled():
		return nil
	case s.pfx != "" && (s.cert != "" || s.key != ""):
		return usageError{errors.New("use either --pfx or --cert and --key")}
	case s.pfx == "" && (s.cert == "" || s.key == ""):
		return usageError{errors.New("--cert and --key go together")}
	}

	return nil
}

//nolint:gochecknoglobals // lookup table
var signHashes = map[string]crypto.Hash{"sha256": crypto.SHA256, "sha384": crypto.SHA384, "sha512": crypto.SHA512, "sha1": crypto.SHA1}

func (s *signFlags) options() (sign.Options, error) {
	opts := sign.Options{TimestampURL: s.timestampURL, Description: s.description, URL: s.url}

	var ok bool
	if opts.Hash, ok = signHashes[s.hash]; !ok {
		return opts, usageError{fmt.Errorf("--hash: unknown digest %q", s.hash)}
	}

	var err error

	opts.Signer, opts.Certificates, err = s.loadKey()

	return opts, err
}

func (s *signFlags) loadKey() (crypto.Signer, []*x509.Certificate, error) {
	switch {
	case s.pfx != "" && (s.cert != "" || s.key != ""):
		return nil, nil, usageError{errors.New("use either --pfx or --cert and --key")}
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
		return nil, nil, usageError{errors.New("signing needs --pfx, or --cert and --key")}
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
func (s *signFlags) signBuild(ctx context.Context, pkg []byte, product string) ([]byte, error) {
	if !s.enabled() {
		return pkg, nil
	}

	if s.description == "" {
		s.description = product
	}

	return s.sign(ctx, pkg)
}

func (s *signFlags) sign(ctx context.Context, pkg []byte) ([]byte, error) {
	opts, err := s.options()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	signed, err := sign.Sign(ctx, pkg, opts)
	if err != nil {
		return nil, err //nolint:wrapcheck // sign errors carry their prefix
	}

	return signed, nil
}

// signCmdFlags are the flags of sign.
type signCmdFlags struct {
	signing signFlags
	out     string
	quiet   bool
}

func newSignCmd() *cobra.Command {
	var flags signCmdFlags

	cmd := &cobra.Command{
		Use:   "sign [flags] package.msi",
		Short: "Add an Authenticode signature to a package",
		Long: `Sign adds an Authenticode signature to a package, replacing an existing
one. It writes in place unless --output is given; a failed signing keeps the
original file. The key comes from a PKCS #12 file (--pfx) or from a PEM
certificate chain and key (--cert and --key). See docs/signing.md.`,
		Example: `  go-msi sign --pfx codesign.pfx --password-env PFX_PASSWORD hello.msi
  go-msi sign --cert chain.pem --key key.pem --timestamp-url http://timestamp.digicert.com hello.msi`,
		Args:              exactlyOne("package"),
		ValidArgsFunction: fileArg("msi"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return flags.run(cmd.Context(), cmd.OutOrStdout(), args[0])
		},
	}

	flags.signing.register(cmd)
	cmd.Flags().StringVarP(&flags.out, "output", "o", "", "output file (default: sign in place)")
	cmd.Flags().BoolVarP(&flags.quiet, "quiet", "q", false, "print nothing on success")
	mustComplete(cmd.MarkFlagFilename("output", "msi"))

	return cmd
}

func (f *signCmdFlags) run(ctx context.Context, stdout io.Writer, input string) error {
	if err := f.signing.check(); err != nil {
		return err
	}

	out := f.out
	if out == "" {
		out = input
	}

	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	signed, err := f.signing.sign(ctx, data)
	if err != nil {
		return err
	}

	if err := writeAtomic(out, signed); err != nil {
		return err
	}

	if !f.quiet {
		return printSignature(stdout, out, signed)
	}

	return nil
}

// writeAtomic replaces path through a temporary file in the same
// directory, so a failed write keeps the original package. An existing
// file keeps its mode, and a symlink to an existing file keeps pointing at
// the replaced file.
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

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify package.msi",
		Short: "Check the Authenticode signature of a package",
		Long: `Verify checks the content digest, the signature and the timestamp token
of a signed package and prints the signer. It does not check certificate
trust or revocation; Windows does that at install time. The exit code is 1
when the package is not signed or the signature does not match.`,
		Example:           "  go-msi verify hello.msi",
		Args:              exactlyOne("package"),
		ValidArgsFunction: fileArg("msi"),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read: %w", err)
			}

			return printSignature(cmd.OutOrStdout(), args[0], data)
		},
	}
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
