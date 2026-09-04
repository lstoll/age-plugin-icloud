#!/bin/sh
# Notarize and staple age-plugin-icloud.app (Developer ID / Gatekeeper).
#
# App Store Connect API key (CI):
#   APPLE_API_KEY_ID  APPLE_API_ISSUER  APPLE_API_KEY_P8
# or a stored notarytool profile:
#   NOTARYTOOL_PROFILE
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="${1:-$ROOT/age-plugin-icloud.app}"
if [ ! -d "$APP" ]; then
	echo "missing $APP" >&2
	exit 1
fi

ZIP="${NOTARIZE_ZIP:-$ROOT/dist/age-plugin-icloud.zip}"
mkdir -p "$(dirname "$ZIP")"
rm -f "$ZIP"
ditto -c -k --keepParent "$APP" "$ZIP"

if [ -n "${NOTARYTOOL_PROFILE:-}" ]; then
	xcrun notarytool submit "$ZIP" --keychain-profile "$NOTARYTOOL_PROFILE" --wait
elif [ -n "${APPLE_API_KEY_P8:-}" ] && [ -n "${APPLE_API_KEY_ID:-}" ] && [ -n "${APPLE_API_ISSUER:-}" ]; then
	keyfile="${APPLE_API_KEY_FILE:-}"
	if [ -z "$keyfile" ]; then
		keyfile="$(mktemp)"
		printf '%s\n' "$APPLE_API_KEY_P8" > "$keyfile"
		trap 'rm -f "$keyfile"' EXIT
	fi
	xcrun notarytool submit "$ZIP" \
		--key "$keyfile" \
		--key-id "$APPLE_API_KEY_ID" \
		--issuer "$APPLE_API_ISSUER" \
		--wait
else
	cat >&2 <<EOF
set NOTARYTOOL_PROFILE or APPLE_API_KEY_ID + APPLE_API_ISSUER + APPLE_API_KEY_P8
EOF
	exit 1
fi

xcrun stapler staple "$APP"
echo "stapled $APP"
