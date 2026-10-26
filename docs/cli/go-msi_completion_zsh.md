# `go-msi completion zsh`

Print the completion script for Zsh

Prints the completion script for Zsh.

Completion must be enabled in Zsh ("autoload -U compinit; compinit").
Load it in the current shell with:

    source <(go-msi completion zsh)

or install it into a directory of $fpath for every new shell:

    go-msi completion zsh > "${fpath[1]}/_go-msi"

## Usage

```text
go-msi completion zsh [flags]
```

## Flags

```text
  -h, --help              help for zsh
      --no-descriptions   leave out the descriptions of the completions
```

## See also

- [`go-msi completion`](go-msi_completion.md): Print a shell completion script
