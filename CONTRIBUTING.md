# Contributing

Contributions are welcome through GitHub pull requests.

## Before You Start

Small fixes and documentation improvements can go directly to a pull request.
Before starting a large feature, breaking change, or substantial refactoring,
[open an issue](https://github.com/jkroepke/go-msi/issues/new/choose)
to discuss the proposal with the maintainer.

Read these documents for an overview of the project and its requirements:

- [YAML schema](docs/design/yaml-schema.md): the input format.
- [MSI specification](docs/spec/MSISPEC.md) and its
  [Windows test results](docs/spec/MSISPEC-RESULTS.md): the format go-msi writes.

Use the Go version specified in [`go.mod`](go.mod). Never update the Go version
in `go.mod`.

## Project Rules

- go-msi is pure Go. Never add cgo, and never add a dependency that needs it.
  The binary must build with `CGO_ENABLED=0` for Linux, macOS and Windows.
- go-msi only builds MSI packages. It never runs them. Signing and MSIX are out
  of scope.
- go-msi is a clean-room implementation. Write code from the specification in
  [`docs/spec`](docs/spec) and from Microsoft's documentation. Never copy or
  port code from other MSI tools, in particular from projects under the LGPL
  or other licenses incompatible with the [Apache License 2.0](LICENSE).

## Required Checks

Before committing a change, run:

```shell
gofmt -l .
go mod tidy -diff
GOTOOLCHAIN=go1.27.3 go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go test ./...
```

golangci-lint must be built with the Go version from `go.mod`. A release binary
built with an older Go refuses to lint this module.

Changes that affect the generated packages also need a test on Windows. The
Windows test harness lives in [`docs/spec/msitest`](docs/spec/msitest).

If a check cannot run because of missing dependencies or network restrictions,
state that limitation in the pull request's testing notes.

GitHub security alerts are the primary mechanism for dependency CVE tracking.
Do not add or require `govulncheck` as a mandatory local or CI check unless the
maintainer explicitly requests it. It may be used as an optional diagnostic
when investigating a specific Go vulnerability or dependency risk.

## Submit a Pull Request

1. Fork the repository and create a branch for your change.
2. Keep the change focused, and update tests and documentation when needed.
3. Sign off each commit with `git commit -s` to certify the
   [Developer Certificate of Origin](https://developercertificate.org/).
4. Open a pull request and explain the change, its reason, and how you tested
   it. Cite relevant files or lines and include the required check results.

Pull request titles are published in user-facing release notes. Write each title
as a concise description of the user-visible change and start it with `feat:`,
`fix:`, `docs:`, or `chore:` as required by CI. Use the `!` form, such as
`feat!:`, for a breaking change. Every pull request also needs one of the labels
`💥 breaking-change`, `✨ enhancement`, `🐞 bug`, `📖 docs`, `🛠️ dependencies` or
`chore`.

## Documentation

Documentation lives in [`docs`](docs) and is published to the
[wiki](https://github.com/jkroepke/go-msi/wiki) on every push to `main`.
The first line of each file is removed on publish, because the wiki shows the
filename as the page title.
