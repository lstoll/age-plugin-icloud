# age-plugin-icloud

An [age](https://age-encryption.org) plugin that keeps an MLKEM768-X25519 secret in **iCloud Keychain**. Recipients are ordinary `age1pq1...` public keys, so encryption does not need this plugin. Decryption does: the on-disk identity is a pointer, not the key.

Losing every Mac signed into that Apple ID loses the key. Encrypt important files to a second recipient if you want a backup you can export.

## Install

macOS only. The plugin is packaged as a dummy `.app` so it can embed a provisioning profile (required for the data-protection keychain). `age` must find `age-plugin-icloud` on `PATH` as a **symlink into that bundle**, not a copied Mach-O.

```bash
./scripts/sign.sh
ln -sf "$(pwd)/age-plugin-icloud.app/Contents/MacOS/age-plugin-icloud" /usr/local/bin/age-plugin-icloud
```

`scripts/sign.sh` runs `go build`, wraps the binary, finds a Mac development profile on this machine, and `codesign`s. Use the same Apple team on every Mac that should decrypt. CI (`go test`) does not sign.

## Usage

Generate (once, on any signed-in Mac). Default name is `default`. `--access-control=userPresence` is stored on the item (default); generate does not prompt. Decrypt calls LocalAuthentication when the annotation is `userPresence`. That is not a Keychain ACL (Apple rejects that on synced items).

```bash
age-plugin-icloud --generate > ~/.age/icloud.txt
age-plugin-icloud --generate --name work > ~/.age/icloud-work.txt
age-plugin-icloud --generate --name ssh --access-control=none
```

Stdout is the identity **pointer** plus a commented `age1pq` recipient. The secret is only in Keychain. Reprint later with `--list` (optionally `--name`); that omits `# created` because it is not stored.

```bash
age-plugin-icloud --list --name work > ~/.age/icloud-work.txt
```

Encrypt with the native recipient (any age 1.3.2+ client, no plugin):

```bash
age -r age1pq1... -o secret.age file
```

Encrypt from the identity file or `-j icloud` (reads the stored public key only):

```bash
age -e -i ~/.age/icloud.txt -o secret.age file
age -e -j icloud -o secret.age file
```

Decrypt on any Mac with that Apple ID (plugin + Keychain; Touch ID or passcode if the identity was generated with `userPresence`):

```bash
age -d -i ~/.age/icloud.txt secret.age
age -d -j icloud secret.age
```

`-j icloud` with an empty identity decrypts with every stored name and encrypts to `default`.

```bash
age-plugin-icloud --list
age-plugin-icloud --list --name work
age-plugin-icloud --delete --name work
```

There is no `--export` / `--import`.

## iCloud Keychain

Items sync via iCloud Keychain (`kSecAttrSynchronizable` + `AfterFirstUnlock`). Apple rejects `kSecAttrAccessControl` on those items (`errSecParam` `-50`). `--access-control` is therefore an annotation in `kSecAttrGeneric`, not a Keychain ACL: generate does not prompt, and decrypt calls `LAContext.EvaluatePolicy` when the annotation is `userPresence`. That is policy in this plugin, not Secure Enclave enforcement. SSH/headless decrypt needs `--access-control=none`.

## Signing

Data-protection keychain (the “iCloud” list in Keychain Access) requires **`com.apple.application-identifier`** and **`keychain-access-groups`** (the App ID, not a sharing group) authorized by an embedded provisioning profile. There is no Xcode project; `scripts/sign.sh` builds a dummy `age-plugin-icloud.app`.

The profile is created once (not every build):

- [developer.apple.com profiles](https://developer.apple.com/account/resources/profiles/list) — Mac App Development, App ID `li.lds.age-plugin-icloud`, this Mac
- or `fastlane sigh --development --platform macos --app_identifier li.lds.age-plugin-icloud`
- or one Xcode automatic-signing build of any macOS app with that bundle ID

Then `sign.sh` picks it up from `~/Library/Developer/Xcode/UserData/Provisioning Profiles/` (or `PROVISIONING_PROFILE=`). Override identity with `CODESIGN_IDENTITY`.

A later distribution build is `RELEASE=1` plus an explicit identity, team, and Developer ID profile (no home-dir search when `CI=true`), then `scripts/notarize.sh`. That path is what `.github/workflows/release.yml` will use; it is not required for local use. Ad-hoc `codesign -s -` has no Team ID and will not sync. Copying the inner Mach-O out of the `.app` drops the profile and macOS kills the process at launch.

## vs age-plugin-se

[age-plugin-se](https://github.com/remko/age-plugin-se) holds keys in the Secure Enclave. Those keys cannot leave the device, so they cannot sync. This plugin stores a software seed that iCloud Keychain can sync, and uses native X-Wing (`age1pq` / `mlkem768x25519`) rather than tagged hardware recipients.
