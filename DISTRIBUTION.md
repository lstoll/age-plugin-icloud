# Building and distributing age-plugin-icloud

End-user install is in [README.md](README.md). This file is how to produce a signed `.app`.

The plugin is a dummy `.app` with an embedded provisioning profile. There is no Xcode project. Data-protection keychain (the “iCloud” list in Keychain Access) requires `com.apple.application-identifier` and `keychain-access-groups` (the App ID, not a sharing group). Every Mac that should decrypt must run a binary signed with the **same Team ID**; a rebuild with another team cannot read those items.

Ad-hoc `codesign -s -` has no Team ID and will not sync. Copying the inner Mach-O out of the `.app` drops the profile and macOS kills the process at launch.

## Local development (Apple Development)

For iterating on this Mac only. Not notarized; not for other people.

1. Check out this repo next to [`lstoll/keychain`](https://github.com/lstoll/keychain). `go.mod` has `replace lds.li/keychain => ../keychain`.
2. Create a **Mac App Development** provisioning profile for App ID `li.lds.age-plugin-icloud` (this Mac), once:
   - [developer.apple.com profiles](https://developer.apple.com/account/resources/profiles/list)
   - or `fastlane sigh --development --platform macos --app_identifier li.lds.age-plugin-icloud`
   - or one Xcode automatic-signing build of any macOS app with that bundle ID
3. Build and sign. `scripts/sign.sh` runs `go build`, wraps the binary, and picks a matching profile from `~/Library/Developer/Xcode/UserData/Provisioning Profiles/` (or set `PROVISIONING_PROFILE=`). Override identity with `CODESIGN_IDENTITY`.

```bash
./scripts/sign.sh
ln -sf "$(pwd)/age-plugin-icloud.app/Contents/MacOS/age-plugin-icloud" /usr/local/bin/age-plugin-icloud
```

## Local distribution (Developer ID + notarize)

Same team (`MZXW569JYG`). Recipients run **this** `.app`.

One-time Apple:

1. [Developer ID Application](https://developer.apple.com/account/resources/certificates/list) certificate (not Apple Development).
2. [Developer ID](https://developer.apple.com/account/resources/profiles/list) provisioning profile for **explicit** App ID `li.lds.age-plugin-icloud` (a wildcard is not enough for Gatekeeper).
3. Store notary credentials: `xcrun notarytool store-credentials` (or use the API key env vars in the next section).

Then:

```bash
RELEASE=1 CODESIGN_IDENTITY="Developer ID Application: Lincoln Stoll (MZXW569JYG)" \
  TEAM_ID=MZXW569JYG PROVISIONING_PROFILE=/path/to/developer-id.provisionprofile \
  ./scripts/sign.sh
NOTARYTOOL_PROFILE=age-plugin-icloud ./scripts/notarize.sh
mkdir -p dist
ditto -c -k --keepParent age-plugin-icloud.app dist/age-plugin-icloud.zip
```

Install remains a symlink into `Contents/MacOS/`.

## CI (GitHub Actions)

Unsigned `go test` is [`.github/workflows/ci.yml`](.github/workflows/ci.yml) (every push and pull request). It checks out `lstoll/keychain` into `./keychain` and rewrites the local `replace`. Push the keychain iCloud/LAContext APIs before the first green run, or tests will not compile.

Signed, notarized zips are [`.github/workflows/release.yml`](.github/workflows/release.yml). Pull requests are not signed.

### How to get a new binary

| Trigger | Version | GitHub Release |
|---|---|---|
| Push to `main` / `master` | `YYYYMMDD-HHMMSS-<sha7>` (UTC) | Prerelease `snapshot-<version>`, not latest |
| `git tag v0.1.0 && git push origin v0.1.0` | `0.1.0` | Named release, marked latest |
| Actions → **release** → Run workflow | same as a main push | Prerelease snapshot |

Asset name: `age-plugin-icloud-<version>-darwin.zip`.

### One-time secrets

Same Developer ID cert, Developer ID profile, and an [App Store Connect API key](https://appstoreconnect.apple.com/access/integrations/api) (Developer / Account Holder, at least Developer access). Export the cert as a `.p12`. Download the `.p8`.

| Secret | Value |
|---|---|
| `APPLE_P12_BASE64` | `base64 -i cert.p12` |
| `APPLE_P12_PASSWORD` | password used when exporting the p12 |
| `APPLE_PROFILE_BASE64` | `base64 -i foo.provisionprofile` |
| `APPLE_CODESIGN_IDENTITY` | `Developer ID Application: Lincoln Stoll (MZXW569JYG)` |
| `APPLE_TEAM_ID` | `MZXW569JYG` |
| `APPLE_API_KEY_ID` | key id (`AB12CD34EF`) |
| `APPLE_API_ISSUER` | issuer UUID |
| `APPLE_API_KEY_P8` | contents of `AuthKey_….p8` (PEM, multiline is fine) |

`CI=true` on `scripts/sign.sh` disables home-dir identity/profile discovery; the workflow always passes identity, team, and profile explicitly.
