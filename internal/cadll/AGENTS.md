# internal/cadll

The custom action DLL of `run` actions and permission grants. Read the root
`AGENTS.md` first.

- `src/builderrun.c` is the whole DLL: freestanding C11, no C runtime, no SDK
  header. Every Win32 and MSI prototype it uses is declared in the file from
  Microsoft's documentation of that function. Keep it that way: a header or
  CRT dependency would pull in a toolchain's runtime and its licence, and the
  size would grow from a few kilobytes to hundreds.
- `build.sh` cross-builds `builderrun-{x86,x64,arm64}.dll` with clang and lld
  (`--target=<arch>-pc-windows-msvc`, `-nostdlib`, `/Brepro`). The import
  libraries come from `.def` lists in the script through `llvm-dlltool`; the
  x86 entries carry the stdcall argument sizes. After a change to the source
  or the script: run `go generate ./internal/cadll/` (it runs `build.sh`,
  which also rewrites `SHA256SUMS`) and commit the DLLs and the sums. CI
  rebuilds with clang 18 on Ubuntu 24.04 and fails when the committed DLLs
  differ, so `build.sh` refuses any other clang major version; point it at
  the right binaries with `CLANG=clang-18 DLLTOOL=llvm-dlltool-18`. `go
  build` never runs generators, so building msi-builder still needs no C
  toolchain.
- The exported entry point is `BuilderRun` (`UINT __stdcall (MSIHANDLE)`).
  It was `GoMsiRun` before the rename to msi-builder. Never give it, or any
  other name of our own in a package, the prefix `Msi`: Microsoft
  reserves it for the engine (spec 17.6).
  The builder (`internal/builder/actions.go`, `runAction`) passes
  `<directory>|<command>` in `CustomActionData` (deferred, rollback, commit)
  or the `BuilderRun` property (immediate); `|` cannot occur in a path, so the
  first one ends the directory.
- The DLL starts the child with `CreateProcessAsUserW` and the thread's
  impersonation token when there is one, else `CreateProcessW`:
  `CreateProcessW` ignores an impersonation token. CA-03 showed no thread
  token in practice: impersonated deferred and immediate actions run in a
  custom action server whose process token is the user's (the log says
  `token: the process`, `whoami` prints the user), so the
  `CreateProcessAsUserW` path is unexercised. The child inherits the custom
  action server's environment block.
- Output is read from one pipe for stdout and stderr, split at `\n`, logged
  per line through `MsiProcessMessage(INSTALLMESSAGE_INFO)` with the fixed
  template `BuilderRun: [1]`, so brackets in the output are not formatted.
  An `IDCANCEL` return from that call terminates the child; it is the only
  cancel signal, so a silent child runs to its end. A
  line is decoded as UTF-8 when valid, else in the OEM code page. `chcp
  65001` inside a `cmd /c` line does not make cmd.exe's `echo` write UTF-8
  on the pipe (CA-03 on the x64 and Arm64 runners: the line stayed OEM and
  `€` came out as `?`); PowerShell with `[Console]::OutputEncoding` does.
- `cadll_test.go` checks the checksums, the PE machine, the export and that
  the imports are only kernel32, advapi32 and msi. `testdata/probe` is the
  CA-03 package that `internal/builder/nativetest` installs on Windows
  (`TestMsiexecRunActions`), natively on the x64 and arm64 runners. The x64
  package under emulation on arm64 passed once (CA-03) and is not rerun.
- Permission grants (spec 17.7) use the exports `BuilderGrantSchedule`
  (immediate) and `BuilderGrant` (the deferred `BuilderGrantExec`, its
  rollback and the commit action, told apart by the first character of
  `CustomActionData`: `+`, `-`, `c`, and `x` on removal). The schedule action reads the
  `BuilderGrants` property, evaluates each line's condition, formats account
  and object separately and rejects `|` and control characters in what a
  property could inject. The deferred action records the rights it adds in
  `HKCU\Software\msi-builder\GrantRollback\<ProductCode>` (the LocalSystem
  hive) before it writes the DACL, so the rollback takes away only those;
  never make the rollback revoke the authored mask, which strips rights an
  object had before a failed repair or upgrade. An account from a property
  is remembered (`a` lines, `HKLM\SOFTWARE\msi-builder\GrantAccounts\<ProductCode>`,
  value named after the template), because `msiexec /fa` and Apps & Features
  repairs run without the first install's command line properties (observed
  on the CI runner, SEC-02). Wide strings in this part
  are `u"..."` literals, which clang types as `unsigned short` arrays.
- Never take code from another installer toolkit's custom action library;
  the Windows Installer custom action contract is documented by Microsoft
  and the spec (section 9.4).
