#!/usr/bin/env bash
# Package the macOS GUI as a code-signed .app inside a DMG.
#
# The old flow cross-compiled on Linux and zipped a hand-made .app: no code
# signature, no PkgInfo, no sealed resources. Downloaded from GitHub the app
# gets a quarantine attribute, so Gatekeeper reports it as damaged. mygo's
# packager builds a proper bundle, signs it (ad hoc by default, or with a
# Developer ID plus notarization when signing secrets are available) and
# wraps it in a DMG, the format macOS expects. This must run on macOS.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ "$(uname -s)" != "Darwin" ]; then
  echo "package-macos.sh needs macOS (codesign, hdiutil, notarytool)" >&2
  exit 1
fi
command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }

VERSION="${VERSION:-$(git describe --always --tags | sed 's/^v//')}"
VERSION="${VERSION#v}"
# A manual run can use a branch name as the version; it must not contain a
# path separator, which would break the DMG file name.
VERSION="${VERSION//\//-}"
GOARCH="${GOARCH:-$(go env GOARCH)}"
OUT="${OUT:-dist/macos}"

mkdir -p build/icons dist
go run ./tools/genicon -out build/icons

# The committed template keeps the release version and signing secrets out
# of git. A Developer ID is only used when both the identity and a notary
# keychain profile are available; otherwise mygo signs ad hoc.
jq --arg v "$VERSION" '.version = $v' deploy/mygo.json > mygo.json
if [ -n "${MACOS_SIGNING_IDENTITY:-}" ] && [ -n "${NOTARY_KEYCHAIN:-}" ]; then
  jq --arg id "$MACOS_SIGNING_IDENTITY" --arg kc "$NOTARY_KEYCHAIN" \
    '.macos.signingIdentity = $id
     | .macos.notarize = {keychainProfile: "notary", keychain: $kc}' \
    mygo.json > mygo.json.tmp
  mv mygo.json.tmp mygo.json
fi

go tool mygo build -platform "darwin/$GOARCH" -o "$OUT" .

APP="$OUT/darwin-$GOARCH/wslpp-gui.app"
DMG="$(echo "$OUT/darwin-$GOARCH"/*.dmg)"
test -d "$APP" || { echo "missing $APP" >&2; exit 1; }
test -f "$DMG" || { echo "missing DMG in $OUT/darwin-$GOARCH" >&2; exit 1; }

codesign --verify --deep --strict --verbose=2 "$APP"
codesign -dvv "$APP" 2>&1 | sed -n '1,12p'

if jq -e '.macos.notarize' mygo.json >/dev/null; then
  xcrun stapler validate "$APP"
  xcrun stapler validate "$DMG"
fi

cp "$DMG" "dist/wslpp-gui-darwin-$GOARCH.dmg"
echo "Built dist/wslpp-gui-darwin-$GOARCH.dmg"
