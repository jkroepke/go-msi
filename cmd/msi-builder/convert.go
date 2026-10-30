package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jkroepke/msi-builder/internal/wixconvert"
)

func newConvertCmd() *cobra.Command {
	var (
		out   string
		force bool
	)

	cmd := &cobra.Command{
		Use:   "convert [flags] app.wxs",
		Short: "Convert a WiX source file to a YAML file",
		Long: `Convert reads a WiX source file (.wxs, WiX v3 or later) and writes the
msi.yaml that describes the same package, as far as msi-builder can express
it. It is a starting point for a port, not a byte-for-byte translation:
everything it cannot convert, such as preprocessor conditions, includes,
bind variables, hand-built dialogs and extensions without a counterpart,
becomes a "# TODO" comment in the YAML file and a line on stderr.

The YAML file goes to stdout, or to the file --output names. Review it, then
check it with msi-builder validate. docs/Migrating from WiX.md explains
each mapping.`,
		Example: `  msi-builder convert app.wxs
  msi-builder convert -o msi.yaml app.wxs`,
		Args:              exactlyOne("app.wxs"),
		ValidArgsFunction: fileArg("wxs"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return convert(cmd, args[0], out, force)
		},
	}

	cmd.Flags().StringVarP(&out, "output", "o", "", "write the YAML file here instead of stdout")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing output file")
	mustComplete(cmd.MarkFlagFilename("output", "yaml", "yml"))

	return cmd
}

func convert(cmd *cobra.Command, path, out string, force bool) error {
	input, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}

	defer func() { _ = input.Close() }()

	res, err := wixconvert.Convert(input, wixconvert.Options{Source: filepath.Base(path)})
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	stderr := cmd.ErrOrStderr()

	for _, n := range res.Notes {
		if n.Line > 0 {
			_, _ = fmt.Fprintf(stderr, "%s:%d: TODO: %s\n", path, n.Line, n.Message)
		} else {
			_, _ = fmt.Fprintf(stderr, "%s: TODO: %s\n", path, n.Message)
		}
	}

	if out == "" {
		_, err := cmd.OutOrStdout().Write(res.YAML)
		if err != nil {
			return fmt.Errorf("write: %w", err)
		}

		return nil
	}

	if err := writeNew(out, res.YAML, force); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stderr, "%s: written, %d TODO comments to review\n", out, len(res.Notes))

	return nil
}

// writeNew writes data to path. It refuses an existing file unless force is
// set.
func writeNew(path string, data []byte, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !force {
		flags |= os.O_EXCL
	}

	file, err := os.OpenFile(path, flags, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s exists; pass --force to overwrite it", path)
	}

	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		_ = file.Close()

		return fmt.Errorf("write: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}
