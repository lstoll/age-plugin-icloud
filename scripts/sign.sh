#!/bin/sh
# Assemble a dummy .app around the Go plugin, embed a provisioning profile,
# and codesign. No Xcode project.
#
# Local:
#   ./scripts/sign.sh
# Release (Developer ID, hardened runtime, timestamp):
#   RELEASE=1 CODESIGN_IDENTITY="Developer ID Application: …" \
#     PROVISIONING_PROFILE=/path/to.provisionprofile TEAM_ID=MZXW569JYG \
#     ./scripts/sign.sh
# CI: same as release, plus CI=true which disables identity/profile discovery.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUNDLE_ID="li.lds.age-plugin-icloud"
APP="$ROOT/age-plugin-icloud.app"
BIN="$APP/Contents/MacOS/age-plugin-icloud"
RELEASE="${RELEASE:-}"
CI="${CI:-}"
VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo 0.0.0)}"

IDENTITY="${CODESIGN_IDENTITY:-}"
if [ -z "$IDENTITY" ]; then
	if [ -n "$CI" ]; then
		echo "CI builds must set CODESIGN_IDENTITY" >&2
		exit 1
	fi
	pattern="Apple Development"
	if [ -n "$RELEASE" ]; then
		pattern="Developer ID Application"
	fi
	IDENTITY="$(security find-identity -v -p codesigning | awk -F'"' -v p="$pattern" '$0 ~ p {print $2; exit}')"
fi
if [ -z "$IDENTITY" ]; then
	echo "no signing identity; set CODESIGN_IDENTITY" >&2
	exit 1
fi

PROFILE="${PROVISIONING_PROFILE:-}"
if [ -z "$PROFILE" ]; then
	if [ -n "$CI" ]; then
		echo "CI builds must set PROVISIONING_PROFILE" >&2
		exit 1
	fi
	PROFILE="$(BUNDLE_ID="$BUNDLE_ID" python3 "$ROOT/packaging/find-profile.py")" || true
fi
if [ -z "$PROFILE" ] || [ ! -f "$PROFILE" ]; then
	cat >&2 <<EOF
no Mac provisioning profile for $BUNDLE_ID

Create one, then re-run with PROVISIONING_PROFILE= or leave it on this Mac:
  https://developer.apple.com/account/resources/profiles/list
  Development: Mac App Development, App ID $BUNDLE_ID
  Release:     Developer ID, App ID $BUNDLE_ID (not a wildcard)
  or: fastlane sigh --development --platform macos --app_identifier $BUNDLE_ID
  or: fastlane sigh --developer_id --platform macos --app_identifier $BUNDLE_ID
EOF
	exit 1
fi

TEAM_ID="${TEAM_ID:-}"
if [ -z "$TEAM_ID" ]; then
	if [ -n "$CI" ]; then
		echo "CI builds must set TEAM_ID" >&2
		exit 1
	fi
	TEAM_ID="$(security cms -D -i "$PROFILE" | plutil -extract TeamIdentifier.0 raw -)"
fi

echo "identity: $IDENTITY"
echo "team:     $TEAM_ID"
echo "profile:  $PROFILE"
echo "version:  $VERSION"
echo "release:  ${RELEASE:-0}"

cd "$ROOT"
unsigned="$(mktemp)"
trap 'rm -f "$unsigned"' EXIT
ldflags=""
if [ -n "$RELEASE" ]; then
	ldflags="-s -w"
fi
go build -trimpath -ldflags="$ldflags" -o "$unsigned" ./cmd/age-plugin-icloud

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS"
cp "$ROOT/packaging/Info.plist" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$APP/Contents/Info.plist" >/dev/null
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $VERSION" "$APP/Contents/Info.plist" >/dev/null
cp "$PROFILE" "$APP/Contents/embedded.provisionprofile"
cp "$unsigned" "$BIN"
chmod +x "$BIN"

entitlements="$(mktemp)"
trap 'rm -f "$unsigned" "$entitlements"' EXIT
cat > "$entitlements" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>com.apple.application-identifier</key>
	<string>${TEAM_ID}.${BUNDLE_ID}</string>
	<key>com.apple.developer.team-identifier</key>
	<string>${TEAM_ID}</string>
	<key>keychain-access-groups</key>
	<array>
		<string>${TEAM_ID}.${BUNDLE_ID}</string>
	</array>
</dict>
</plist>
EOF

sign() {
	target="$1"
	if [ -n "$RELEASE" ]; then
		codesign --force --sign "$IDENTITY" \
			--identifier "$BUNDLE_ID" \
			--entitlements "$entitlements" \
			--options runtime \
			--timestamp \
			"$target"
	else
		codesign --force --sign "$IDENTITY" \
			--identifier "$BUNDLE_ID" \
			--entitlements "$entitlements" \
			"$target"
	fi
}

sign "$BIN"
sign "$APP"
codesign --verify --strict "$APP"

echo
echo "built $BIN"
codesign -dv --entitlements - "$BIN" 2>&1 | grep -E 'Identifier=|TeamIdentifier=|Runtime=|application-identifier|keychain-access' || true
echo
echo "symlink onto PATH (do not copy the Mach-O out of the .app):"
echo "  ln -sf \"$BIN\" /usr/local/bin/age-plugin-icloud"
if [ -n "$RELEASE" ]; then
	echo "then: ./scripts/notarize.sh"
fi
