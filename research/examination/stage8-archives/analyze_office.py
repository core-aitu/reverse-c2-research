#!/usr/bin/env python3
"""Static OOXML/VBA inventory for stage 8. Never opens Office or runs macros."""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import math
import re
import sys
from pathlib import Path
from xml.etree import ElementTree

sys.path.append("/usr/lib/python3.14/site-packages")
import olefile  # type: ignore  # system package, used read-only


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


class DotNetRandom:
    """Legacy seeded System.Random used by the recovered Windows PowerShell."""

    def __init__(self, seed: int):
        maximum = 2147483647
        subtraction = maximum if seed == -2147483648 else abs(seed)
        current = 161803398 - subtraction
        if current < 0:
            current += maximum
        self.values = [0] * 56
        self.values[55] = current
        previous = 1
        for index in range(1, 55):
            slot = (21 * index) % 55
            self.values[slot] = previous
            previous = current - previous
            if previous < 0:
                previous += maximum
            current = self.values[slot]
        for _ in range(4):
            for index in range(1, 56):
                self.values[index] -= self.values[1 + (index + 30) % 55]
                if self.values[index] < 0:
                    self.values[index] += maximum
        self.first = 0
        self.second = 21

    def next(self, maximum_value: int) -> int:
        maximum = 2147483647
        self.first = 1 if self.first + 1 >= 56 else self.first + 1
        self.second = 1 if self.second + 1 >= 56 else self.second + 1
        value = self.values[self.first] - self.values[self.second]
        if value == maximum:
            value -= 1
        if value < 0:
            value += maximum
        self.values[self.first] = value
        return int((value * (1.0 / maximum)) * maximum_value)


def resume_multiline(value: str, seed: int) -> str:
    random = DotNetRandom(seed)
    positions = [random.next(len(value)) for _ in value]
    characters = list(value)
    for index in range(len(characters) - 1, -1, -1):
        target = positions[index]
        characters[index], characters[target] = characters[target], characters[index]
    return "".join(characters)


def decompress_vba(data: bytes, *, accept_bad_signature: bool = False) -> tuple[bytes, list[dict]]:
    if not data or data[0] != 1:
        raise ValueError("missing compressed-container signature")
    output = bytearray()
    chunks: list[dict] = []
    position = 1
    while position < len(data):
        if position + 2 > len(data):
            raise ValueError("truncated chunk header")
        header = int.from_bytes(data[position : position + 2], "little")
        position += 2
        chunk_size = (header & 0x0FFF) + 3
        signature = (header >> 12) & 7
        compressed = bool(header & 0x8000)
        if signature != 3 and not accept_bad_signature:
            raise ValueError(f"noncanonical chunk signature {signature}")
        chunk_end = min(position + chunk_size - 2, len(data))
        chunk_start = len(output)
        chunks.append(
            {
                "header": f"0x{header:04x}",
                "declared_size": chunk_size,
                "signature": signature,
                "compressed": compressed,
            }
        )
        if not compressed:
            output.extend(data[position:chunk_end])
            position = chunk_end
            continue
        while position < chunk_end and len(output) - chunk_start < 4096:
            flags = data[position]
            position += 1
            for bit in range(8):
                if position >= chunk_end:
                    break
                if not flags & (1 << bit):
                    output.append(data[position])
                    position += 1
                    continue
                if position + 2 > chunk_end:
                    raise ValueError("truncated copy token")
                token = int.from_bytes(data[position : position + 2], "little")
                position += 2
                distance = len(output) - chunk_start
                offset_bits = max(4, min(12, math.ceil(math.log2(distance)) if distance else 4))
                length_mask = (1 << (16 - offset_bits)) - 1
                length = (token & length_mask) + 3
                offset = (token >> (16 - offset_bits)) + 1
                if offset > distance:
                    raise ValueError("copy token precedes chunk output")
                for _ in range(length):
                    output.append(output[-offset])
        position = chunk_end
    return bytes(output), chunks


def recover_module(stream: bytes) -> dict | None:
    candidates: list[tuple[int, int, bytes, list[dict], bool]] = []
    for offset, byte in enumerate(stream):
        if byte != 1:
            continue
        for relaxed in (False, True):
            try:
                source, chunks = decompress_vba(stream[offset:], accept_bad_signature=relaxed)
            except (IndexError, ValueError):
                continue
            score = sum(
                marker in source
                for marker in (b"Attribute VB_Name", b"Sub ", b"End Sub", b"Function ")
            )
            if score:
                candidates.append((score, offset, source, chunks, relaxed))
                break
    if not candidates:
        return None
    score, offset, source, chunks, relaxed = max(candidates, key=lambda item: (item[0], len(item[2])))
    text = source.decode("cp1252", "replace")
    procedures = sorted(set(re.findall(r"(?im)^\s*(?:Private\s+|Public\s+)?(?:Sub|Function)\s+([A-Za-z0-9_]+)", text)))
    indicators = sorted(
        marker
        for marker in (
            "AutoOpen",
            "Workbook_Open",
            "CreateObject",
            "WScript.Shell",
            "powershell.exe",
            "EncodedCommand",
            "ExecutionPolicy Bypass",
        )
        if marker.lower() in text.lower()
    )
    encoded = None
    command_match = re.search(r"(?is)EncodedCommand.+?(?:\.Run|\Z)", text)
    if command_match:
        literals = []
        value = []
        inside = False
        index = 0
        line_start = text.rfind("\n", 0, command_match.start()) + 1
        candidate = text[line_start : command_match.end()]
        while index < len(candidate):
            char = candidate[index]
            if not inside:
                if char == '"':
                    inside = True
                    value = []
                index += 1
                continue
            if char == '"' and index + 1 < len(candidate) and candidate[index + 1] == '"':
                value.append('"')
                index += 2
                continue
            if char == '"':
                literals.append("".join(value))
                inside = False
                index += 1
                continue
            value.append(char)
            index += 1
        joined = "".join(literals)
        payload = joined.split('EncodedCommand "', 1)[-1]
        run = re.match(r"[A-Za-z0-9+/=]{80,}", payload)
        run = run.group(0) if run else ""
        if run:
            try:
                raw = base64.b64decode(run + "=" * (-len(run) % 4), validate=False)
                decoded = raw.decode("utf-16le", "replace")
                encoded = {
                    "base64_characters_recovered": len(run),
                    "decoded_sha256": sha256(raw),
                    "decoded_characters": len(decoded),
                    "decoded_preview": decoded[:800],
                    "resume_multiline_values": [
                        {
                            "encoded": value,
                            "seed": int(seed),
                            "decoded": resume_multiline(value, int(seed)),
                        }
                        for value, seed in re.findall(r"Resume-Multiline\s+'([^']*)'\s+(\d+)", decoded)
                    ],
                }
            except (ValueError, UnicodeError):
                pass
    return {
        "source_offset": offset,
        "source_bytes": len(source),
        "source_sha256": sha256(source),
        "recovery": "relaxed-noncanonical" if relaxed else "standard",
        "chunk_headers": chunks,
        "procedures": procedures,
        "indicators": indicators,
        "encoded_command": encoded,
    }


def analyze_vba(path: Path) -> dict:
    raw = path.read_bytes()
    result = {"path": str(path), "bytes": len(raw), "sha256": sha256(raw), "streams": [], "modules": []}
    with olefile.OleFileIO(path) as compound:
        for parts in compound.listdir(streams=True):
            stream = compound.openstream(parts).read()
            name = "/".join(parts)
            result["streams"].append({"name": name, "bytes": len(stream), "sha256": sha256(stream)})
            if len(parts) == 2 and parts[0] == "VBA" and parts[1] not in {"dir", "_VBA_PROJECT"}:
                module = recover_module(stream)
                result["modules"].append({"stream": name, "stream_sha256": sha256(stream), "source": module})
    return result


def parse_relationships(path: Path, root: Path) -> dict:
    raw = path.read_bytes()
    rows = []
    tree = ElementTree.fromstring(raw)
    for item in tree:
        rows.append(
            {
                "id": item.attrib.get("Id"),
                "type": item.attrib.get("Type"),
                "target": item.attrib.get("Target"),
                "target_mode": item.attrib.get("TargetMode", "Internal"),
            }
        )
    return {"path": str(path.relative_to(root)), "bytes": len(raw), "sha256": sha256(raw), "relationships": rows}


def analyze_document(root: Path) -> dict:
    files = []
    relationships = []
    vba = []
    embedded = []
    for path in sorted(item for item in root.rglob("*") if item.is_file()):
        raw = path.read_bytes()
        relative = path.relative_to(root).as_posix()
        files.append({"path": relative, "bytes": len(raw), "sha256": sha256(raw)})
        if path.name.endswith(".rels"):
            relationships.append(parse_relationships(path, root))
        if path.name == "vbaProject.bin":
            vba.append(analyze_vba(path))
        if "/embeddings/" in f"/{relative}" or "oleObject" in relative:
            embedded.append({"path": relative, "bytes": len(raw), "sha256": sha256(raw)})
    external = [
        row
        for relation_file in relationships
        for row in relation_file["relationships"]
        if row["target_mode"].lower() == "external"
    ]
    return {
        "name": root.name,
        "files": files,
        "relationships": relationships,
        "external_relationships": external,
        "embedded_objects": embedded,
        "vba_projects": vba,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("parts_root", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    report = {
        "method": "static OOXML/XML/OLE parsing; no Office process; no macro or payload execution",
        "documents": [analyze_document(path) for path in sorted(args.parts_root.iterdir()) if path.is_dir()],
    }
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
