# `go-msi completion bash`

Print the completion script for Bash

Prints the completion script for Bash.

The script needs the bash-completion package. Load it in the current
shell with:

    source <(go-msi completion bash)

or install it for every new shell, for example on Linux:

    go-msi completion bash > /etc/bash_completion.d/go-msi

## Usage

```text
go-msi completion bash [flags]
```

## Flags

```text
  -h, --help              help for bash
      --no-descriptions   leave out the descriptions of the completions
```

## See also

- [`go-msi completion`](go-msi_completion.md): Print a shell completion script
