#!/bin/sh
# Developer ID–sign, notarize, and zip.
#
# Local: login Keychain identity + packaging/age-plugin-icloud.provisionprofile
#        + notarytool profile lstoll-dev (override with NOTARYTOOL_PROFILE).
# CI:    DEVELOPER_ID_P12_BASE64, DEVELOPER_ID_P12_PASSWORD, ASC_API_KEY_P8.
# Identity, team, ASC key id, and issuer are not secret (hardcoded below).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export CODESIGN_IDENTITY="${CODESIGN_IDENTITY:-Developer ID Application: Lincoln Stoll (MZXW569JYG)}"
export TEAM_ID="${TEAM_ID:-MZXW569JYG}"
export APPLE_API_KEY_ID="${APPLE_API_KEY_ID:-A7QQHL8ZW2}"
export APPLE_API_ISSUER="${APPLE_API_ISSUER:-29bfa82d-ac5d-4dae-b30d-5a106c81d146}"
export RELEASE="${RELEASE:-1}"
VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo 0.0.0)}"
export VERSION
export PROVISIONING_PROFILE="${PROVISIONING_PROFILE:-$ROOT/packaging/age-plugin-icloud.provisionprofile}"

if [ ! -f "$PROVISIONING_PROFILE" ]; then
	echo "missing $PROVISIONING_PROFILE" >&2
	exit 1
fi

if [ -n "${GITHUB_ACTIONS:-}" ]; then
	if [ -z "${DEVELOPER_ID_P12_BASE64:-}" ] || [ -z "${DEVELOPER_ID_P12_PASSWORD:-}" ]; then
		echo "DEVELOPER_ID_P12_BASE64 and DEVELOPER_ID_P12_PASSWORD are required in GitHub Actions" >&2
		exit 1
	fi
	if [ -z "${ASC_API_KEY_P8:-}" ]; then
		echo "ASC_API_KEY_P8 is required in GitHub Actions" >&2
		exit 1
	fi
	export CI=true
	export APPLE_API_KEY_P8="$ASC_API_KEY_P8"
	./scripts/ci-import-signing.sh
else
	export NOTARYTOOL_PROFILE="${NOTARYTOOL_PROFILE:-lstoll-dev}"
fi

./scripts/sign.sh
./scripts/notarize.sh

mkdir -p "$ROOT/dist"
zip="$ROOT/dist/age-plugin-icloud-${VERSION}-darwin.zip"
rm -f "$zip"
ditto -c -k --keepParent "$ROOT/age-plugin-icloud.app" "$zip"
echo "wrote $zip"
