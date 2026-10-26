# `go-msi inspect`

Print the tables, streams and summary of a package

Inspect prints the code page, the summary information, the streams and
every table of a package, or a single table with --table. Columns are
separated by tabs; the second line of a table holds the column types, with
"!" marking key columns. Cabinets stay in the file and are listed by size.

## Usage

```text
go-msi inspect [flags] package.msi
```

## Examples

```text
  go-msi inspect hello.msi
  go-msi inspect --table File hello.msi
```

## Flags

```text
  -h, --help           help for inspect
      --table string   print only this table
```

## See also

- [`go-msi`](go-msi.md): Build Windows Installer (MSI) packages from a YAML file
