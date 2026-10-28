package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// completionShell describes the completion script of one shell.
type completionShell struct {
	gen   func(root *cobra.Command, w io.Writer, descriptions bool) error
	name  string
	title string
	setup string
}

//nolint:gochecknoglobals // static table
var completionShells = []completionShell{
	{
		name:  "bash",
		title: "Bash",
		setup: `The script needs the bash-completion package. Load it in the current
shell with:

    source <(msi-builder completion bash)

or install it for every new shell, for example on Linux:

    msi-builder completion bash > /etc/bash_completion.d/msi-builder`,
		gen: func(root *cobra.Command, w io.Writer, descriptions bool) error {
			return root.GenBashCompletionV2(w, descriptions)
		},
	},
	{
		name:  "fish",
		title: "fish",
		setup: `Load it in the current shell with:

    msi-builder completion fish | source

or install it for every new shell:

    msi-builder completion fish > ~/.config/fish/completions/msi-builder.fish`,
		gen: func(root *cobra.Command, w io.Writer, descriptions bool) error {
			return root.GenFishCompletion(w, descriptions)
		},
	},
	{
		name:  "powershell",
		title: "PowerShell",
		setup: `Load it in the current session with:

    msi-builder completion powershell | Out-String | Invoke-Expression

and add that line to your PowerShell profile to load it in every session.`,
		gen: func(root *cobra.Command, w io.Writer, descriptions bool) error {
			if descriptions {
				return root.GenPowerShellCompletionWithDesc(w)
			}

			return root.GenPowerShellCompletion(w)
		},
	},
	{
		name:  "zsh",
		title: "Zsh",
		setup: `Completion must be enabled in Zsh ("autoload -U compinit; compinit").
Load it in the current shell with:

    source <(msi-builder completion zsh)

or install it into a directory of $fpath for every new shell:

    msi-builder completion zsh > "${fpath[1]}/_msi-builder"`,
		gen: func(root *cobra.Command, w io.Writer, descriptions bool) error {
			if descriptions {
				return root.GenZshCompletion(w)
			}

			return root.GenZshCompletionNoDesc(w)
		},
	},
}

// newCompletionCmd replaces cobra's default completion command, so the help
// text follows the project's documentation style.
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Print a shell completion script",
		Long: `Completion prints the script that completes msi-builder commands, flags,
file arguments, signature digests and, for inspect --table, the tables of
the package on the command line. Each shell has its own command.`,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageError{errors.New("completion wants a shell: bash, fish, powershell or zsh")}
			}

			return usageError{fmt.Errorf("unknown shell %q, want bash, fish, powershell or zsh", args[0])}
		},
	}

	for _, shell := range completionShells {
		var noDescriptions bool

		sub := &cobra.Command{
			Use:               shell.name,
			Short:             "Print the completion script for " + shell.title,
			Long:              "Prints the completion script for " + shell.title + ".\n\n" + shell.setup,
			Args:              noArgs,
			ValidArgsFunction: cobra.NoFileCompletions,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return shell.gen(cmd.Root(), cmd.OutOrStdout(), !noDescriptions)
			},
		}

		sub.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "leave out the descriptions of the completions")
		cmd.AddCommand(sub)
	}

	return cmd
}
