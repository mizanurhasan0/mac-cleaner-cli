#!/bin/bash
# Builds dist/Mac-Cleaner.command: ONE self-contained, double-clickable file.
# It embeds a universal (Apple Silicon + Intel) mac-cleaner-cli binary.
set -euo pipefail
cd "$(dirname "$0")"

VERSION=$(grep -E '^var version' main.go | sed -E 's/.*"(.*)".*/\1/')
OUT=dist
rm -rf "$OUT" && mkdir -p "$OUT/work"

echo "Building mac-cleaner-cli $VERSION (arm64 + amd64)…"
for arch in arm64 amd64; do
  CGO_ENABLED=0 GOOS=darwin GOARCH=$arch \
    go build -trimpath -ldflags "-s -w" -o "$OUT/work/mac-cleaner-cli-$arch" .
done
lipo -create -output "$OUT/work/mac-cleaner-cli" "$OUT/work/mac-cleaner-cli-arm64" "$OUT/work/mac-cleaner-cli-amd64"

cat > "$OUT/Mac-Cleaner.command" <<'LAUNCHER'
#!/bin/bash
# mac-cleaner-cli: a light Mac cleaner. Double-click this file, or run: bash Mac-Cleaner.command
# Scans first, nothing is removed until you choose and confirm.
# Extra args are passed through, e.g.  bash Mac-Cleaner.command quick --dry-run
set -e
clear 2>/dev/null || true
WORK=$(mktemp -d "${TMPDIR:-/tmp}/mac-cleaner.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
sed '1,/^__MAC_CLEANER_PAYLOAD__$/d' "$0" | base64 -D | gunzip > "$WORK/mac-cleaner-cli"
chmod +x "$WORK/mac-cleaner-cli"
xattr -c "$WORK/mac-cleaner-cli" 2>/dev/null || true
"$WORK/mac-cleaner-cli" "$@" || true
echo
read -r -n 1 -p "Press any key to close…" _ || true
# Close this Terminal window, but only when it was opened by double-clicking this file
# (its title contains "Mac-Cleaner.command"), never a window the user opened themselves.
if [ "$TERM_PROGRAM" = "Apple_Terminal" ]; then
  TTY_NAME=$(tty 2>/dev/null || true)
  # Find the window id now, while this shell is still alive and the tty still matches.
  WIN_ID=$(osascript 2>/dev/null <<OSA || true
tell application "Terminal"
  repeat with w in windows
    try
      if (tty of selected tab of w) is "$TTY_NAME" and (name of w) contains "Mac-Cleaner.command" then return id of w
    end try
  end repeat
end tell
OSA
)
  if [ -n "$WIN_ID" ]; then
    ( sleep 0.4; osascript -e "tell application \"Terminal\" to close (every window whose id is $WIN_ID)" >/dev/null 2>&1 ) >/dev/null 2>&1 &
  fi
fi
exit 0
__MAC_CLEANER_PAYLOAD__
LAUNCHER

gzip -9 -c "$OUT/work/mac-cleaner-cli" | base64 >> "$OUT/Mac-Cleaner.command"
chmod +x "$OUT/Mac-Cleaner.command"
rm -rf "$OUT/work"
ls -lh "$OUT/Mac-Cleaner.command"
