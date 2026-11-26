#!/usr/bin/env python3
"""Create or verify a SHA-256 inventory of local research reports and ADR documents."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys

from research_reports import read_json

ROOT = Path(__file__).resolve().parents[2]
DIRECTORIES = ("research/reports", "reimplementation/docs/adr")
MAX_FILE = 8 * 1024 * 1024
SCHEMA = "research-document-inventory/v1"


def inventory(root):
    root = Path(root).resolve(strict=True)
    entries = []
    for relative in DIRECTORIES:
        directory = root / relative
        # Do not follow symlinks in any of the selected directory components.
        if directory.resolve(strict=True) != directory or not directory.is_dir():
            raise ValueError(f"invalid inventory directory: {relative}")
        for path in sorted(directory.glob("*.md")):
            if path.is_symlink() or not path.is_file():
                raise ValueError(f"non-regular document: {path.name}")
            digest = hashlib.sha256()
            size = 0
            with path.open("rb") as stream:
                while chunk := stream.read(64 * 1024):
                    size += len(chunk)
                    if size > MAX_FILE:
                        raise ValueError(f"document too large: {path.name}")
                    digest.update(chunk)
            entries.append({"path": path.relative_to(root).as_posix(), "bytes": size, "sha256": digest.hexdigest()})
    return {"schema": SCHEMA, "files": sorted(entries, key=lambda e: e["path"])}


def indexed(manifest):
    if not isinstance(manifest, dict) or set(manifest) != {"schema", "files"} or manifest["schema"] != SCHEMA or not isinstance(manifest["files"], list):
        raise ValueError("invalid inventory manifest")
    entries = {}
    for entry in manifest["files"]:
        if not isinstance(entry, dict) or set(entry) != {"path", "bytes", "sha256"}:
            raise ValueError("invalid manifest entry")
        path = entry["path"]
        if not isinstance(path, str) or not any(re.fullmatch(re.escape(d) + r"/[A-Za-z0-9_.-]+\.md", path) for d in DIRECTORIES):
            raise ValueError("path outside document inventory scope")
        if path in entries or type(entry["bytes"]) is not int or not 0 <= entry["bytes"] <= MAX_FILE or not isinstance(entry["sha256"], str) or not re.fullmatch(r"[0-9a-f]{64}", entry["sha256"]):
            raise ValueError("duplicate or invalid inventory entry")
        entries[path] = entry
    return entries


def verify(expected, actual):
    before, after = indexed(expected), indexed(actual)
    added = sorted(after.keys() - before.keys())
    missing = sorted(before.keys() - after.keys())
    changed = sorted(k for k in before.keys() & after.keys() if before[k] != after[k])
    return {"schema": "research-document-verification/v1", "ok": not (added or missing or changed),
            "matched": len(before.keys() & after.keys()) - len(changed),
            "added": added, "missing": missing, "changed": changed}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("snapshot")
    commands.add_parser("verify").add_argument("manifest")
    args = parser.parse_args(argv)
    try:
        actual = inventory(ROOT)
        result = actual if args.command == "snapshot" else verify(read_json(args.manifest), actual)
        print(json.dumps(result, indent=2))
        return 0 if args.command == "snapshot" or result["ok"] else 1
    except (OSError, ValueError, TypeError, OverflowError) as error:
        print(f"evidence-integrity: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
