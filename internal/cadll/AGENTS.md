# internal/cadll

The custom action DLL of `run` actions. Read the root `AGENTS.md` first.

- `src/gomsirun.c` is the whole DLL: freestanding C11, no C runtime, no SDK
  header. Every Win32 and MSI prototype it uses is declared in the file from
  Microsoft's documentation of that function. Keep it that way: a header or
  CRT dependency would pull in a toolchain's runtime and its licence, and the
  size would grow from a few kilobytes to hundreds.
- `build.sh` cross-builds `gomsirun-{x86,x64,arm64}.dll` with clang and lld
  (`--target=<arch>-pc-windows-msvc`, `-nostdlib`, `/Brepro`). The import
  libraries come from `.def` lists in the script through `llvm-dlltool`; the
  x86 entries carry the stdcall argument sizes. After a change to the source
  or the script: run `build.sh`, run `sha256sum gomsirun-*.dll > SHA256SUMS`
  in this directory, and commit all of it. CI rebuilds with clang 18 on
  Ubuntu 24.04 and fails when the committed DLLs differ, so build with the
  same version (`CLANG=clang-18 DLLTOOL=llvm-dlltool-18 ./build.sh`); other
  clang versions give different bytes.
- The exported entry point is `GoMsiRun` (`UINT __stdcall (MSIHANDLE)`).
  The builder (`pkg/builder/actions.go`, `runAction`) passes
  `<directory>|<command>` in `CustomActionData` (deferred, rollback, commit)
  or the `GoMsiRun` property (immediate); `|` cannot occur in a path, so the
  first one ends the directory.
- The DLL starts the child with `CreateProcessAsUserW` and the thread's
  impersonation token when there is one, else `CreateProcessW`:
  `CreateProcessW` ignores an impersonation token. The child inherits the
  custom action server's environment block.
- Output is read from one pipe for stdout and stderr, split at `\n`, logged
  per line through `MsiProcessMessage(INSTALLMESSAGE_INFO)` with the fixed
  template `GoMsiRun: [1]`, so brackets in the output are not formatted. A
  line is decoded as UTF-8 when valid, else in the OEM code page.
- `cadll_test.go` checks the checksums, the PE machine, the export and that
  the imports are only kernel32, advapi32 and msi. `testdata/probe` is the
  CA-03 package that `pkg/builder/nativetest` installs on Windows
  (`TestMsiexecRunActions`), natively on x64 and arm64 runners and under
  x64 emulation on arm64.
- Never take code from another installer toolkit's custom action library;
  the Windows Installer custom action contract is documented by Microsoft
  and the spec (section 9.4).
