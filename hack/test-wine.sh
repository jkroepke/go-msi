#!/usr/bin/env bash
# Black-box test: build examples/minimal and install it with Wine's msiexec.
# Wine is only run as an installer engine; its sources are never consulted.
set -euo pipefail

cd "$(dirname "$0")/.."

export WINEDEBUG="${WINEDEBUG:--all}"
export WINEPREFIX="${WINEPREFIX:-$(mktemp -d)}"
export WINEDLLOVERRIDES="mscoree=d;mshtml=d"

work=$(mktemp -d)
cp examples/minimal/msi.yaml "$work/"
go run ./internal/testpe/cmd -o "$work/hello.exe"
go run ./cmd/go-msi build --output "$work/minimal.msi" "$work/msi.yaml"

WINE="${WINE:-$(command -v wine || command -v wine64 || ls /usr/lib/wine/wine64)}"

run() {
  if command -v xvfb-run >/dev/null; then xvfb-run -a "$WINE" "$@"; else "$WINE" "$@"; fi
}

run msiexec /i "$("$WINE" winepath -w "$work/minimal.msi")" /qn /l*v "$("$WINE" winepath -w "$work/install.log")" || {
  echo "install failed; log follows" >&2
  iconv -f UTF-16 -t UTF-8 "$work/install.log" 2>/dev/null || cat "$work/install.log"
  exit 1
}

installed="$WINEPREFIX/drive_c/Program Files/Hello/hello.exe"
if [ ! -f "$installed" ]; then
  echo "hello.exe was not installed to $installed" >&2
  ls -R "$WINEPREFIX/drive_c/Program Files" >&2 || true
  exit 1
fi
cmp "$installed" "$work/hello.exe"

run msiexec /x "$("$WINE" winepath -w "$work/minimal.msi")" /qn /l*v "$("$WINE" winepath -w "$work/uninstall.log")"
if [ -f "$installed" ]; then
  echo "hello.exe is still there after the uninstall" >&2
  exit 1
fi
"$WINE" wineboot -k 2>/dev/null || wineserver -k || true
echo "wine: install and uninstall of examples/minimal succeeded"
