package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

//nolint:gochecknoglobals // test flag
var update = flag.Bool("update", false, "rewrite docs/cli from the command tree")

// docsDir holds the generated command reference, published to the wiki.
var docsDir = filepath.Join("..", "..", "docs", "cli") //nolint:gochecknoglobals // test constant

// TestDocs checks that docs/cli matches the command tree. Run
// go generate ./cmd/msi-builder after changing a command or flag.
func TestDocs(t *testing.T) {
	t.Parallel()

	want := genDocs(newRootCmd(io.Discard, io.Discard))

	if *update {
		if err := os.RemoveAll(docsDir); err != nil {
			t.Fatal(err)
		}

		if err := os.MkdirAll(docsDir, 0o755); err != nil {
			t.Fatal(err)
		}

		for name, data := range want {
			if err := os.WriteFile(filepath.Join(docsDir, name), data, 0o644); err != nil { //nolint:gosec // documentation is world-readable
				t.Fatal(err)
			}
		}

		return
	}

	entries, err := os.ReadDir(docsDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, ent := range entries {
		if _, ok := want[ent.Name()]; !ok {
			t.Errorf("docs/cli/%s belongs to no command; run go generate ./cmd/msi-builder", ent.Name())
		}
	}

	for name, data := range want {
		// A Windows checkout may convert the pages to CRLF.
		got, err := os.ReadFile(filepath.Join(docsDir, name))
		if err != nil || !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), data) {
			t.Errorf("docs/cli/%s is out of date; run go generate ./cmd/msi-builder", name)
		}
	}
}

// genDocs renders one Markdown page per command, named after the command
// path. The first line is the title, which the wiki sync removes.
func genDocs(root *cobra.Command) map[string][]byte {
	// cobra adds these during Execute.
	root.InitDefaultHelpCmd()
	root.InitDefaultVersionFlag()

	pages := make(map[string][]byte)

	var walk func(cmd *cobra.Command)

	walk = func(cmd *cobra.Command) {
		cmd.InitDefaultHelpFlag()
		pages[docName(cmd)] = genPage(cmd)

		for _, sub := range cmd.Commands() {
			if sub.IsAvailableCommand() && !sub.IsAdditionalHelpTopicCommand() {
				walk(sub)
			}
		}
	}

	walk(root)

	return pages
}

func docName(cmd *cobra.Command) string {
	return strings.ReplaceAll(cmd.CommandPath(), " ", "_") + ".md"
}

func genPage(cmd *cobra.Command) []byte {
	var buf bytes.Buffer

	buf.WriteString("# `" + cmd.CommandPath() + "`\n\n" + cmd.Short + "\n\n")

	if cmd.Long != "" {
		buf.WriteString(strings.TrimSpace(cmd.Long) + "\n\n")
	}

	// Like the usage template: commands with subcommands run only to
	// report a usage error.
	switch {
	case cmd.HasAvailableSubCommands():
		buf.WriteString("## Usage\n\n```text\n" + cmd.CommandPath() + " [command]\n```\n\n")
	case cmd.Runnable():
		buf.WriteString("## Usage\n\n```text\n" + cmd.UseLine() + "\n```\n\n")
	}

	if cmd.Example != "" {
		buf.WriteString("## Examples\n\n```text\n" + cmd.Example + "\n```\n\n")
	}

	if flags := cmd.NonInheritedFlags(); flags.HasAvailableFlags() {
		buf.WriteString("## Flags\n\n```text\n" + flags.FlagUsages() + "```\n\n")
	}

	if flags := cmd.InheritedFlags(); flags.HasAvailableFlags() {
		buf.WriteString("## Inherited flags\n\n```text\n" + flags.FlagUsages() + "```\n\n")
	}

	var links []string

	if cmd.HasParent() {
		parent := cmd.Parent()
		links = append(links, "- [`"+parent.CommandPath()+"`]("+docName(parent)+"): "+parent.Short)
	}

	subs := slices.Clone(cmd.Commands())
	slices.SortFunc(subs, func(a, b *cobra.Command) int { return strings.Compare(a.Name(), b.Name()) })

	for _, sub := range subs {
		if sub.IsAvailableCommand() && !sub.IsAdditionalHelpTopicCommand() {
			links = append(links, "- [`"+sub.CommandPath()+"`]("+docName(sub)+"): "+sub.Short)
		}
	}

	if len(links) > 0 {
		buf.WriteString("## See also\n\n" + strings.Join(links, "\n") + "\n")
	}

	return append(bytes.TrimRight(buf.Bytes(), "\n"), '\n')
}
