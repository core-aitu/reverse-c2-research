#!/usr/bin/env python3
"""Publish a signed beacon matrix and its metadata to the gateway release API."""
import argparse
import hashlib
import json
import mimetypes
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import quote
from urllib.request import Request, urlopen


TARGETS = (
    ("linux", "amd64", "specter-beacon-linux-amd64"),
    ("linux", "arm64", "specter-beacon-linux-arm64"),
    ("windows", "amd64", "specter-beacon-windows-amd64.exe"),
    ("darwin", "arm64", "specter-beacon-darwin-arm64"),
)


def upload(url, token, path, content_type, extra_headers=None):
    content = path.read_bytes()
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": content_type,
        "Content-Length": str(len(content)),
        "X-Content-SHA256": hashlib.sha256(content).hexdigest(),
    }
    headers.update(extra_headers or {})
    try:
        with urlopen(Request(url, data=content, headers=headers, method="PUT"), timeout=120) as response:
            return json.load(response)
    except HTTPError as error:
        raise SystemExit(f"publish failed: {error.code} {error.read().decode(errors='replace')}") from error


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--token", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--dist", default="dist")
    args = parser.parse_args()
    root = Path(args.dist)
    assets = (
        ("sbom", root / "specter-beacon.spdx.json", "application/spdx+json"),
        ("checksums", root / "SHA256SUMS", "text/plain"),
        ("signature", root / "SHA256SUMS.sig", "application/octet-stream"),
    )
    for _, _, name in TARGETS:
        if not (root / name).is_file():
            raise SystemExit(f"missing release binary: {root / name}")
    for _, path, _ in assets:
        if not path.is_file():
            raise SystemExit(f"missing release asset: {path}")
    base = args.endpoint.rstrip("/")
    for operating_system, architecture, name in TARGETS:
        release = upload(
            f"{base}/api/beacon-releases/{quote(args.version)}/{operating_system}/{architecture}",
            args.token,
            root / name,
            mimetypes.guess_type(name)[0] or "application/octet-stream",
        )
        release_id = quote(release["id"])
        for asset, path, media_type in assets:
            release = upload(f"{base}/api/beacon-releases/{release_id}/assets/{asset}", args.token, path, media_type)
        print(json.dumps({"id": release["id"], "target": f"{operating_system}/{architecture}", "signed": release["signed"]}))


if __name__ == "__main__":
    main()
