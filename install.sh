#!/bin/bash
# mac-cleaner-cli launcher: downloads the latest Mac-Cleaner.command and runs it.
# Nothing is installed and nothing stays on your Mac after you quit.
#
#   bash -c "$(curl -fsSL https://raw.githubusercontent.com/mizanurhasan0/mac-cleaner-cli/main/install.sh)"
#
# Copyright (c) 2026 Mizanur Hasan. MIT License.
set -euo pipefail

URL="https://github.com/mizanurhasan0/mac-cleaner-cli/raw/main/dist/Mac-Cleaner.command"

if [ "$(uname -s)" != "Darwin" ]; then
  echo "mac-cleaner-cli only runs on macOS." >&2
  exit 1
fi

TMP=$(mktemp -d "${TMPDIR:-/tmp}/mac-cleaner-dl.XXXXXX")
trap 'rm -rf "$TMP"' EXIT

echo "Downloading mac-cleaner-cli…"
curl -fsSL "$URL" -o "$TMP/Mac-Cleaner.command"
bash "$TMP/Mac-Cleaner.command" "$@"
