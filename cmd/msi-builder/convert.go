package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/wixconvert"
)

func newConvertCmd() *cobra.Command {
	var (
		out   string
		built string
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

With --msi and the package WiX built from the source file, every component
whose Guid WiX generated (Guid="*" or none) keeps the ComponentId it has in
that package, when convert finds it there by its ID or its key file, and
each file a WiX Files glob harvested keeps its code in the guids of the
folder source the glob became; the others get a "# TODO" comment. Without
them, msi-builder generates other component codes, and
an upgrade that removes the old version after installing the new one
(MajorUpgrade Schedule="afterInstallExecute", and presumably
"afterInstallFinalize") deletes the files of every component whose code
changed.

The YAML file goes to stdout, or to the file --output names. Review it, then
check it with msi-builder validate.`,
		Example: `  msi-builder convert app.wxs
  msi-builder convert -o msi.yaml app.wxs
  msi-builder convert --msi app-1.4.0.msi -o msi.yaml app.wxs`,
		Args:              exactlyOne("app.wxs"),
		ValidArgsFunction: fileArg("wxs"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if force && out == "" {
				return usageError{errors.New("--force needs --output")}
			}

			return convert(cmd, args[0], built, out, force)
		},
	}

	cmd.Flags().StringVarP(&out, "output", "o", "", "write the YAML file here instead of stdout")
	cmd.Flags().StringVar(&built, "msi", "", "keep the component codes of this package, built by WiX from app.wxs")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing output file")
	mustComplete(cmd.MarkFlagFilename("output", "yaml", "yml"))
	mustComplete(cmd.MarkFlagFilename("msi", "msi"))

	return cmd
}

func convert(cmd *cobra.Command, path, built, out string, force bool) error {
	opts := wixconvert.Options{Source: filepath.Base(path)}

	if built != "" {
		pkg, err := builtPackage(built)
		if err != nil {
			return err
		}

		opts.Built = pkg
	}

	input, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}

	defer func() { _ = input.Close() }()

	res, err := wixconvert.Convert(input, opts)
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

// builtPackage reads the components of a package, the long names of their
// key files and the folders.
func builtPackage(path string) (*wixconvert.BuiltPackage, error) {
	database, closePackage, err := openPackage(path)
	if err != nil {
		return nil, err
	}

	defer closePackage()

	components := database.Table("Component")
	if components == nil {
		return nil, fmt.Errorf("%s has no Component table", path)
	}

	names := make(map[string]string)

	if files := database.Table("File"); files != nil {
		key, name := files.Column("File"), files.Column("FileName")
		for _, row := range files.Rows {
			long := cellString(row, name)
			if _, after, ok := strings.Cut(long, "|"); ok {
				long = after
			}

			names[cellString(row, key)] = long
		}
	}

	cols := []int{
		components.Column("Component"), components.Column("ComponentId"),
		components.Column("Directory_"), components.Column("Attributes"), components.Column("KeyPath"),
	}
	if slices.Contains(cols, -1) {
		return nil, fmt.Errorf("%s: the Component table lacks a standard column", path)
	}

	pkg := &wixconvert.BuiltPackage{Name: filepath.Base(path), Directories: builtDirectories(database)}

	for _, row := range components.Rows {
		comp := wixconvert.BuiltComponent{
			ID: cellString(row, cols[0]), Code: cellString(row, cols[1]), Directory: cellString(row, cols[2]),
		}

		// Bits 4 and 32: the key path is a registry value or an ODBC
		// data source, not a file.
		if attributes, ok := cellInt(row, cols[3]); ok && attributes&(4|32) == 0 {
			comp.KeyFile = names[cellString(row, cols[4])]
		}

		pkg.Components = append(pkg.Components, comp)
	}

	return pkg, nil
}

// builtDirectories reads the Directory table: the ID, the parent and the
// long target name of each folder.
func builtDirectories(database *msidb.Database) []wixconvert.BuiltDirectory {
	table := database.Table("Directory")
	if table == nil {
		return nil
	}

	key, parent, name := table.Column("Directory"), table.Column("Directory_Parent"), table.Column("DefaultDir")
	dirs := make([]wixconvert.BuiltDirectory, 0, len(table.Rows))

	for _, row := range table.Rows {
		// DefaultDir is target[:source], each short|long.
		target, _, _ := strings.Cut(cellString(row, name), ":")
		if _, long, ok := strings.Cut(target, "|"); ok {
			target = long
		}

		dirs = append(dirs, wixconvert.BuiltDirectory{ID: cellString(row, key), Parent: cellString(row, parent), Name: target})
	}

	return dirs
}

func cellString(row []any, col int) string {
	if col < 0 || col >= len(row) {
		return ""
	}

	if s, ok := row[col].(string); ok {
		return s
	}

	return ""
}

func cellInt(row []any, col int) (int, bool) {
	if col < 0 || col >= len(row) {
		return 0, false
	}

	switch value := row[col].(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case nil:
		return 0, true
	}

	return 0, false
}
