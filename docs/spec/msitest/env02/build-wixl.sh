#!/usr/bin/env bash
# Builds env02/wixl.msi with msitools' wixl, used as a black box in a
# Debian trixie container (msitools 0.106+repack-1). Run from this directory.
set -euo pipefail
docker run --rm -v "$PWD:/w" -w /w mirror.gcr.io/library/debian:trixie bash -c '
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq >/dev/null
  apt-get install -y -qq --no-install-recommends wixl=0.106+repack-1 >/dev/null
  dpkg-query -W wixl
  wixl -a x64 -o wixl.msi wixl.wxs
  sha256sum wixl.msi'
