# age-plugin-icloud

:warning: *This is mostly vibe coded, and homelab quality code. It has not been deeply security reviewed, and the design does have limitations*

An [age](https://age-encryption.org) plugin that keeps an MLKEM768-X25519 secret in iCloud Keychain. Recipients are ordinary `age1pq1...` public keys, so encryption does not need this plugin.

Limitations:
* Secret is stored in the iCloud keychain, and synced. It is protected to the normal Keychain + Sync levels, the secret is extractable.
* The biometric unlock is enforced by the plugin, not they keychain. The unlock time is stored as an attribute on the secret. The command will not allow changing the access level, but anything granted permission to the keychain item can.

## Install

macOS only. The plugin is a dummy `.app`, symlink the command in to the .app (required for the needed signing)

Download the zip from [Releases](https://github.com/lstoll/age-plugin-icloud/releases).

```bash
ln -sf /path/to/age-plugin-icloud.app/Contents/MacOS/age-plugin-icloud <somewhere on $PATH>/age-plugin-icloud
```

## Usage

Generate (once, on any signed-in Mac). Default name is `default`. `--access-control` is stored on the item (default `5m`).

```bash
age-plugin-icloud --generate > ~/.age/icloud.txt
age-plugin-icloud --generate --name sops-secret --access-control=everyTime
age-plugin-icloud --generate --name ssh --access-control=none
```

Stdout is the identity pointer plus a commented `age1pq` recipient. The secret is only in Keychain. Reprint later with `--list` (optionally `--name`).

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

Decrypt on any Mac with that Apple ID (plugin + Keychain).

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