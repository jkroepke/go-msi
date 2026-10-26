#!/bin/sh
# build.sh builds gomsirun.dll for x86, x64 and arm64 from src/gomsirun.c
# with clang and lld. The DLL is freestanding (no C runtime, no SDK), so the
# only build inputs are the source, the import definitions below and the
# compiler. /Brepro makes the output reproducible for one clang version;
# CI rebuilds the DLLs and compares them with the committed ones.
set -eu
cd "$(dirname "$0")"
CLANG=${CLANG:-clang}
DLLTOOL=${DLLTOOL:-llvm-dlltool}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# imports writes a .def for a system DLL; the second column is the stdcall
# argument size, which only the x86 import names carry.
imports() {
  echo "LIBRARY $1.dll"
  echo "EXPORTS"
  shift
  for f in "$@"; do
    if [ "$arch" = i386 ]; then
      echo "${f%%:*}@${f##*:}"
    else
      echo "${f%%:*}"
    fi
  done
}

for target in i686:i386:x86 x86_64:i386:x86-64:x64 aarch64:arm64:arm64; do
  triple=${target%%:*}
  rest=${target#*:}
  arch=${rest%:*}
  out=${rest##*:}
  mkdir -p "$tmp/$out"
  imports kernel32 CreatePipe:16 SetHandleInformation:12 CreateProcessW:40 ReadFile:20 \
    WaitForSingleObject:8 GetExitCodeProcess:8 CloseHandle:4 GetCurrentThread:0 \
    GetProcessHeap:0 HeapAlloc:12 HeapReAlloc:16 HeapFree:12 GetLastError:0 \
    MultiByteToWideChar:24 TerminateProcess:8 >"$tmp/$out/kernel32.def"
  imports advapi32 OpenThreadToken:16 DuplicateTokenEx:24 CreateProcessAsUserW:44 >"$tmp/$out/advapi32.def"
  imports msi MsiGetPropertyW:16 MsiCreateRecord:4 MsiRecordSetStringW:12 MsiProcessMessage:12 \
    MsiCloseHandle:4 >"$tmp/$out/msi.def"
  for lib in kernel32 advapi32 msi; do
    "$DLLTOOL" -m "$arch" -k -d "$tmp/$out/$lib.def" -l "$tmp/$out/$lib.lib"
  done
  "$CLANG" --target="$triple-pc-windows-msvc" -std=c11 -O2 -Wall -Wextra -Werror \
    -fno-builtin -ffreestanding -fno-stack-protector -mno-stack-arg-probe \
    -fno-exceptions \
    -c src/gomsirun.c -o "$tmp/$out/gomsirun.obj"
  "$CLANG" --target="$triple-pc-windows-msvc" -fuse-ld=lld -nostdlib -shared \
    -o "gomsirun-$out.dll" "$tmp/$out/gomsirun.obj" \
    "$tmp/$out/kernel32.lib" "$tmp/$out/advapi32.lib" "$tmp/$out/msi.lib" \
    -Wl,/Brepro -Wl,/nodefaultlib -Wl,/entry:_DllMainCRTStartup -Wl,/export:GoMsiRun \
    -Wl,/release -Wl,/opt:ref -Wl,/opt:icf -Wl,/subsystem:windows
done
ls -l gomsirun-*.dll
