#!/usr/bin/env python3
"""Print the newest Mac provisioning profile that covers BUNDLE_ID."""

import os
import plistlib
import subprocess
import sys
from pathlib import Path


def main() -> int:
    bundle = os.environ.get("BUNDLE_ID", "li.lds.age-plugin-icloud")
    dirs = [
        Path.home() / "Library/Developer/Xcode/UserData/Provisioning Profiles",
        Path.home() / "Library/MobileDevice/Provisioning Profiles",
    ]
    extra = os.environ.get("PROVISIONING_PROFILE_DIR")
    if extra:
        dirs.insert(0, Path(extra))

    best, best_exp = None, None
    for d in dirs:
        if not d.is_dir():
            continue
        for f in d.iterdir():
            if f.suffix not in {".mobileprovision", ".provisionprofile"}:
                continue
            try:
                raw = subprocess.check_output(
                    ["security", "cms", "-D", "-i", str(f)], stderr=subprocess.DEVNULL
                )
                p = plistlib.loads(raw)
            except Exception:
                continue
            if "OSX" not in (p.get("Platform") or []):
                continue
            ent = p.get("Entitlements") or {}
            appid = (
                ent.get("com.apple.application-identifier")
                or ent.get("application-identifier")
                or ""
            )
            prefixes = p.get("ApplicationIdentifierPrefix") or p.get("TeamIdentifier") or []
            if not any(appid in {f"{prefix}.{bundle}", f"{prefix}.*"} for prefix in prefixes):
                continue
            groups = ent.get("keychain-access-groups") or []
            if groups and not any(g.endswith(".*") or g.endswith("." + bundle) for g in groups):
                continue
            exp = p.get("ExpirationDate")
            if best is None or (exp is not None and (best_exp is None or exp > best_exp)):
                best, best_exp = f, exp
    if best is None:
        return 1
    print(best)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
