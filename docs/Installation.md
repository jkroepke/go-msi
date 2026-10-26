# Installation

go-msi is one static binary for Linux, macOS and Windows on amd64 and arm64.

## Release binaries

Download an archive from the
[releases page](https://github.com/jkroepke/go-msi/releases/latest): a `.tar.gz`
for Linux and macOS, a `.zip` for Windows. Extract `go-msi` and put it on your
`PATH`.

## Go

```shell
go install github.com/jkroepke/go-msi/cmd/go-msi@latest
```

## Container image

The image runs on Linux for amd64 and arm64. Mount the directory with the
configuration and its files at `/work`:

```shell
docker run --rm -v "$PWD:/work" ghcr.io/jkroepke/go-msi build -o hello.msi msi.yaml
```

## Shell completion

`go-msi completion <shell>` prints a completion script for Bash, Zsh, fish or
PowerShell, and `go-msi completion <shell> --help` shows how to load it. See
[`go-msi completion`](cli/go-msi_completion.md).

## Check the installation

```shell
go-msi version
```

Continue with [Getting Started](Getting%20Started.md).
