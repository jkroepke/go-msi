# `go-msi completion powershell`

Print the completion script for PowerShell

Prints the completion script for PowerShell.

Load it in the current session with:

    go-msi completion powershell | Out-String | Invoke-Expression

and add that line to your PowerShell profile to load it in every session.

## Usage

```text
go-msi completion powershell [flags]
```

## Flags

```text
  -h, --help              help for powershell
      --no-descriptions   leave out the descriptions of the completions
```

## See also

- [`go-msi completion`](go-msi_completion.md): Print a shell completion script
