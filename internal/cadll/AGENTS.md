# internal/cadll

The custom action DLL of `run` actions, permission grants and `close`. Read
the root `AGENTS.md` first.

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
  `CreateProcessW` ignores an impersonation token. Only `ERROR_NO_TOKEN`
  from `OpenThreadToken` takes the `CreateProcessW` path; any other error
  fails the action rather than start the command as the process. CA-03
  showed no thread token in practice: impersonated deferred and immediate
  actions run in a custom action server whose process token is the user's
  (the log says `token: the process`, `whoami` prints the user), so the
  `CreateProcessAsUserW` path is unexercised. The child inherits the custom
  action server's environment block.
- The child is created suspended with a `PROC_THREAD_ATTRIBUTE_HANDLE_LIST`
  that holds only the pipe's write end, so no other inheritable handle of
  the custom action server (another run action's pipe, for one) goes along,
  put into a job with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, then resumed. A
  cancel ends the job, and closing the job at the end of the action ends
  whatever the command left running. When the job cannot be created or
  assigned (a server in a job that forbids nesting), the action logs it and
  goes on with the child alone.
- Output is read from one named pipe for stdout and stderr (a random
  `\\.\pipe\BuilderRun-<32 hex>` name from `RtlGenRandom`, one instance,
  inbound: only a named pipe can be read overlapped; the random name, the
  single instance and the default pipe security, which gives accounts
  other than the creator, LocalSystem and administrators read access only,
  keep other accounts from writing into the log). A line ends at `\n`, `\r`
  or `\r\n`; each is logged through `MsiProcessMessage(INSTALLMESSAGE_INFO)`
  with the fixed template `BuilderRun: [1]`, so brackets in the output are
  not formatted. The buffer is 64 KiB (`LINE_BUFFER`); a longer line is
  logged in pieces. The overlapped read waits on the child too: once it has
  exited, another process that keeps the pipe open gets `EXIT_GRACE_MS`
  (5 s), then the job is ended and the rest is not read; while nothing
  arrives, a zero progress message every `CANCEL_POLL_MS` (1 s) asks the
  installer about a cancel, which `IDCANCEL` from a logged line also
  signals (that a progress report returns the cancel and stays out of the
  log is documented, not yet seen: the tests have no UI handler). A read
  that fails for another reason closes the pipe and the action waits for
  the child without it, still polling for a cancel. A line is decoded as UTF-8 when valid, else in the OEM code
  page. `chcp 65001` inside a `cmd /c` line does not make cmd.exe's `echo`
  write UTF-8 on the pipe (CA-03 on the x64 and Arm64 runners: the line
  stayed OEM and `€` came out as `?`); PowerShell with
  `[Console]::OutputEncoding` does.
- The build has no stack probes (`-mno-stack-arg-probe`) and no stack
  protector, so keep locals well under a page (4 KB): the line buffer and
  every string of unknown length live on the heap. Constants that do not
  fit an `int` are macros, not enumerators (C11 6.7.2.2). The struct
  layouts the DLL declares itself are pinned by `_Static_assert`s on their
  documented sizes for 32- and 64-bit builds.
- `cadll_test.go` checks the checksums, the PE machine, the export and that
  the imports are only kernel32, advapi32, msi and user32. `testdata/probe` is the
  CA-03 package that `internal/builder/nativetest` installs on Windows
  (`TestMsiexecRunActions`), natively on the x64 and arm64 runners. The x64
  package under emulation on arm64 passed once (CA-03) and is not rerun.
- Permission grants (spec 17.7) use the exports `BuilderGrantSchedule`
  (immediate) and `BuilderGrant` (the deferred `BuilderGrantExec`, its
  rollback and the commit action, told apart by the first character of
  `CustomActionData`: `+`, `-`, `c`, and `x` for the commit of a removal). The schedule action reads the
  `BuilderGrants` property, evaluates each line's condition, formats account
  and object separately and rejects `|` and control characters in what a
  property could inject. The deferred action records the rights it adds in
  `HKCU\Software\msi-builder\GrantRollback\<ProductCode>` (the LocalSystem
  hive) before it writes the DACL, so the rollback takes away only those;
  never make the rollback revoke the authored mask, which strips rights an
  object had before a failed repair or upgrade. An account from a property
  is remembered by the commit action (`a` lines,
  `HKLM\SOFTWARE\msi-builder\GrantAccounts\<ProductCode>`, value named
  after the template), because `msiexec /fa` and Apps & Features
  repairs run without the first install's command line properties (observed
  on the CI runner, SEC-02). Wide strings in this part
  are `u"..."` literals, which clang types as `unsigned short` arrays.
- `close` (spec 17.8) uses the exports `BuilderCloseSchedule` (immediate,
  before InstallValidate, as the user: `WM_CLOSE` to the visible unowned
  top-level windows of every process whose image path equals the file's,
  to its hidden ones only when it has none, then a wait) and `BuilderClose`
  (the deferred `BuilderCloseExec`, LocalSystem, after StopServices:
  `TerminateProcess` with 1602). They are the only user32 imports; keep
  user32 out of every other path. A process matches by
  `QueryFullProcessImageNameW` and `CompareStringOrdinal` without case,
  never by name alone, so copies elsewhere survive. The paths
  `BuilderCloseExec` ends come from type-51 actions of the engine
  (`[BuilderCloseExec][DIR]file|`), never from the schedule action: the
  impersonated custom action server runs with the user's token, so data it
  sets for a LocalSystem action is under the user's control. Grants
  (above) predate this and still hand their data over from the schedule
  action. Neither close action fails the install; the schedule action
  returns `ERROR_INSTALL_USEREXIT` only after a cancel.
- Never take code from another installer toolkit's custom action library;
  the Windows Installer custom action contract is documented by Microsoft
  and the spec (section 9.4).
