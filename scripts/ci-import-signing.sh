#!/bin/sh
# Import a .p12 and provisioning profile for headless CI signing.
#
# Required env:
#   APPLE_P12_BASE64
#   APPLE_P12_PASSWORD
#   APPLE_PROFILE_BASE64
# Optional:
#   KEYCHAIN_PATH (default: $RUNNER_TEMP/signing.keychain-db)
#   KEYCHAIN_PASSWORD
set -euo pipefail

if [ -z "${APPLE_P12_BASE64:-}" ] || [ -z "${APPLE_P12_PASSWORD:-}" ] || [ -z "${APPLE_PROFILE_BASE64:-}" ]; then
	echo "APPLE_P12_BASE64, APPLE_P12_PASSWORD, and APPLE_PROFILE_BASE64 are required" >&2
	exit 1
fi

TMP="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"
KEYCHAIN_PATH="${KEYCHAIN_PATH:-$TMP/age-plugin-icloud-signing.keychain-db}"
KEYCHAIN_PASSWORD="${KEYCHAIN_PASSWORD:-$(openssl rand -base64 24)}"
P12="$TMP/signing.p12"
PROFILE_OUT="${PROVISIONING_PROFILE:-$TMP/age-plugin-icloud.provisionprofile}"

export P12 PROFILE_OUT
python3 - <<'PY'
import base64, os, pathlib
pathlib.Path(os.environ["P12"]).write_bytes(base64.b64decode(os.environ["APPLE_P12_BASE64"]))
pathlib.Path(os.environ["PROFILE_OUT"]).write_bytes(base64.b64decode(os.environ["APPLE_PROFILE_BASE64"]))
PY

if [ -f "$KEYCHAIN_PATH" ]; then
	security delete-keychain "$KEYCHAIN_PATH" || true
fi
security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
security set-keychain-settings -lut 21600 "$KEYCHAIN_PATH"
security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
security import "$P12" -k "$KEYCHAIN_PATH" -P "$APPLE_P12_PASSWORD" \
	-T /usr/bin/codesign -T /usr/bin/security -T /usr/bin/productbuild
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH" >/dev/null
# Keep existing search list (login keychain) and put ours first.
existing="$(security list-keychains -d user | sed 's/"//g')"
# shellcheck disable=SC2086
security list-keychains -d user -s "$KEYCHAIN_PATH" $existing
security default-keychain -d user -s "$KEYCHAIN_PATH"

rm -f "$P12"

if [ -n "${GITHUB_ENV:-}" ]; then
	printf 'PROVISIONING_PROFILE=%s\n' "$PROFILE_OUT" >> "$GITHUB_ENV"
	printf 'KEYCHAIN_PATH=%s\n' "$KEYCHAIN_PATH" >> "$GITHUB_ENV"
fi
echo "PROVISIONING_PROFILE=$PROFILE_OUT"
