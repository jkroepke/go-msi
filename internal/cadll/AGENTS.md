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
  the imports are only kernel32, advapi32, msi and ntdll (NtCreateFile, NtSetInformationFile, RtlNtStatusToDosError). `testdata/probe` is the
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
- Folder removal (spec 17.9) is the immediate export `BuilderRemoveFolders`
  (checks, after RemoveFiles) and `BuilderRemoveFoldersExec` for the
  rollback (`u`), deferred (`m`, or `d` with rollback disabled) and commit
  (`c`) actions. Do not go back to temporary `RemoveFile` rows: RemoveFiles
  deletes by path and followed a junction swapped in after the listing
  (FILE-11, CI run 41058565605), an arbitrary delete as LocalSystem. The
  deferred side therefore works by handle: open without following
  (`FILE_FLAG_OPEN_REPARSE_POINT`), re-check the final path by that
  handle, rename by that handle into `X:\BuilderRemoveFolders.<token>`,
  and delete entries opened relative to their folder's handle with
  `NtCreateFile`, deleting links as links. Rename into a directory handle
  with `NtSetInformationFile(FileRenameInformation)`:
  `SetFileInformationByHandle(FileRenameInfo)` with `RootDirectory` set
  fails with error 87 (FILE-11, CI run 41064763472). Public folder
  properties are secure by default (or `configurable`), so a standard
  user's command line can reach an elevated session: the immediate side removes nothing when
  `MsiRunningElevated` is set and its token holds no Administrators
  group. The backup folder's name (the token) is visible to anyone who
  lists the volume root; never rely on it being secret, only on
  `openBackup`'s owner and protected-DACL check. The
  guards (drive roots, standard folder properties and their parents
  compared by final path, profiles by token SID, the Windows folder, a
  reparse point as the folder, a junction or link above it; not a volume
  mounted in a folder above it) only ever skip an entry, and no action
  fails the installation: a cleanup must not block an uninstall. Test guards only on
  folders the test creates (FILE-11 points `TemplateFolder` into one), so
  a broken guard cannot delete anything that matters.
- Never take code from another installer toolkit's custom action library;
  the Windows Installer custom action contract is documented by Microsoft
  and the spec (section 9.4).
