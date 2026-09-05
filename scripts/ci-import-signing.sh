#!/bin/sh
# Import a Developer ID .p12 for headless CI signing.
#
# Required env:
#   DEVELOPER_ID_P12_BASE64
#   DEVELOPER_ID_P12_PASSWORD
# Optional:
#   KEYCHAIN_PATH (default: $RUNNER_TEMP/signing.keychain-db)
#   KEYCHAIN_PASSWORD
set -euo pipefail

if [ -z "${DEVELOPER_ID_P12_BASE64:-}" ] || [ -z "${DEVELOPER_ID_P12_PASSWORD:-}" ]; then
	echo "DEVELOPER_ID_P12_BASE64 and DEVELOPER_ID_P12_PASSWORD are required" >&2
	exit 1
fi

TMP="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"
KEYCHAIN_PATH="${KEYCHAIN_PATH:-$TMP/age-plugin-icloud-signing.keychain-db}"
KEYCHAIN_PASSWORD="${KEYCHAIN_PASSWORD:-$(openssl rand -base64 24)}"
P12="$TMP/signing.p12"

export P12
python3 - <<'PY'
import base64, os, pathlib
pathlib.Path(os.environ["P12"]).write_bytes(base64.b64decode(os.environ["DEVELOPER_ID_P12_BASE64"]))
PY

if [ -f "$KEYCHAIN_PATH" ]; then
	security delete-keychain "$KEYCHAIN_PATH" || true
fi
security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
security set-keychain-settings -lut 21600 "$KEYCHAIN_PATH"
security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
security import "$P12" -k "$KEYCHAIN_PATH" -P "$DEVELOPER_ID_P12_PASSWORD" \
	-T /usr/bin/codesign -T /usr/bin/security -T /usr/bin/productbuild
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH" >/dev/null
# Keep existing search list (login keychain) and put ours first.
existing="$(security list-keychains -d user | sed 's/"//g')"
# shellcheck disable=SC2086
security list-keychains -d user -s "$KEYCHAIN_PATH" $existing
# Do not steal the login keychain on a developer Mac.
if [ -n "${GITHUB_ACTIONS:-}" ]; then
	security default-keychain -d user -s "$KEYCHAIN_PATH"
fi

rm -f "$P12"

if [ -n "${GITHUB_ENV:-}" ]; then
	printf 'KEYCHAIN_PATH=%s\n' "$KEYCHAIN_PATH" >> "$GITHUB_ENV"
fi
