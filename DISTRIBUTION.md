# Signing and release

Ad-hoc `codesign -s -` has no Team ID and will not sync. Copying the inner Mach-O out of the `.app` drops the profile and macOS kills the process at launch.

## One command

```bash
./scripts/dist.sh
```

Signs with **Developer ID Application: Lincoln Stoll (MZXW569JYG)**, embeds `packaging/age-plugin-icloud.provisionprofile`, notarizes, and writes `dist/age-plugin-icloud-<version>-darwin.zip`.

Local: the cert is already in the login Keychain. Notary uses `notarytool` profile `lstoll-dev` (`NOTARYTOOL_PROFILE` overrides).

CI: GitHub Environment `release` supplies `DEVELOPER_ID_P12_BASE64`, `DEVELOPER_ID_P12_PASSWORD`, and `ASC_API_KEY_P8`. Team, identity, ASC key id `A7QQHL8ZW2`, and issuer are hardcoded in `scripts/dist.sh`.

## Local development (Apple Development)

For iterating on this Mac only. Not notarized; not for other people.

1. Check out this repo next to [`lstoll/keychain`](https://github.com/lstoll/keychain). `go.mod` has `replace lds.li/keychain => ../keychain`.
2. Create a **Mac App Development** provisioning profile for App ID `li.lds.age-plugin-icloud` (this Mac).
3. `./scripts/sign.sh` (picks Apple Development + a matching local profile).

```bash
./scripts/sign.sh
ln -sf "$(pwd)/age-plugin-icloud.app/Contents/MacOS/age-plugin-icloud" /usr/local/bin/age-plugin-icloud
```

## CI (GitHub Actions)

Unsigned `go test` is [`.github/workflows/ci.yml`](.github/workflows/ci.yml). It checks out `lstoll/keychain` into `./keychain` and rewrites the local `replace`. Push the keychain iCloud/LAContext APIs before the first green run.

Signed zips are [`.github/workflows/release.yml`](.github/workflows/release.yml). That job uses environment `release` and runs only on `lstoll/age-plugin-icloud` (not forks). Same-repo PRs from the owner get an artifact; GitHub Releases stay on `main` / `v*`. Never `pull_request_target`.

| Trigger | Version | GitHub Release |
|---|---|---|
| Push to `main` / `master` | `YYYYMMDD-HHMMSS-<sha7>` (UTC) | Prerelease `snapshot-<version>`, not latest |
| `git tag v0.1.0 && git push origin v0.1.0` | `0.1.0` | Named release, marked latest |
| Actions → **release** → Run workflow | same as a main push | Prerelease snapshot |
| `./scripts/dist.sh` on this Mac | `git describe` | local `dist/*.zip` |

Asset name: `age-plugin-icloud-<version>-darwin.zip`.
