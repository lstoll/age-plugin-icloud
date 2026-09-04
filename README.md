# age-plugin-icloud

An [age](https://age-encryption.org) plugin that keeps an MLKEM768-X25519 secret in **iCloud Keychain**. Recipients are ordinary `age1pq1...` public keys, so encryption does not need this plugin. Decryption does: the on-disk identity is a pointer, not the key.

Losing every Mac signed into that Apple ID loses the key. Encrypt important files to a second recipient if you want a backup you can export.

## Install

macOS only. The plugin is a dummy `.app` (needed so the data-protection keychain will accept it). `age` must find `age-plugin-icloud` on `PATH` as a **symlink into that bundle**, not a copied Mach-O.

Download the zip from [Releases](https://github.com/lstoll/age-plugin-icloud/releases) (`v*` for a named release, or a snapshot prerelease). Unzip somewhere stable, then:

```bash
ln -sf /path/to/age-plugin-icloud.app/Contents/MacOS/age-plugin-icloud /usr/local/bin/age-plugin-icloud
```

Copying the inner binary out of the `.app` will not work. Building and signing is in [DISTRIBUTION.md](DISTRIBUTION.md).

## Usage

Generate (once, on any signed-in Mac). Default name is `default`. `--access-control` is stored on the item (default `5m`); generate does not prompt.

```bash
age-plugin-icloud --generate > ~/.age/icloud.txt
age-plugin-icloud --generate --name work --access-control=everyTime
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

Decrypt on any Mac with that Apple ID (plugin + Keychain). `5m` (default) and `everyTime` prompt for Touch ID or passcode; `none` does not:

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

Items sync via iCloud Keychain. `--access-control` is a plugin prompt, not an Apple Keychain ACL (Apple will not attach those to synced items):

- `none` — no prompt
- `everyTime` — Touch ID or passcode on every decrypt
- `5m` (default) — prompt, then skip for five minutes on this Mac. Other Macs still prompt.

Generate does not prompt. SSH/headless decrypt needs `--access-control=none`. Older items stored as `userPresence` are treated as `5m`.

## vs age-plugin-se

[age-plugin-se](https://github.com/remko/age-plugin-se) holds keys in the Secure Enclave. Those keys cannot leave the device, so they cannot sync. This plugin stores a software seed that iCloud Keychain can sync, and uses native X-Wing (`age1pq` / `mlkem768x25519`) rather than tagged hardware recipients.
