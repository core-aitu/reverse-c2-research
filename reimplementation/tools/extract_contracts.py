#!/usr/bin/env python3
"""Extract a normalized REST contract index from a local ILSpy reconstruction.

The decompiled source is an ephemeral oracle and must stay outside the repository.
Only normalized interface facts are written to the clean-room tree.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path


METHOD = re.compile(
    r"public virtual async Task(?:<(?P<return>[^\n]+?)>)? "
    r"(?P<name>\w+Async)\((?P<parameters>[^\n]*)CancellationToken cancellationToken\)"
)
HTTP_METHOD = re.compile(r'request_\.Method = new HttpMethod\("([A-Z]+)"\)')
PATH_LITERAL = re.compile(r'stringBuilder\.Append\("([^"\\]*(?:\\.[^"\\]*)*)"\)')
PATH_PARAMETER = re.compile(
    r"stringBuilder\.Append\(Uri\.EscapeDataString\(ConvertToString\((?P<name>\w+),"
)
PATH_CHAR = re.compile(r"stringBuilder\.Append\('(?P<char>[^']+)'\)")
STATUS = re.compile(r"status_ == (\d{3})")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def parse(source: str) -> list[dict[str, object]]:
    starts = list(METHOD.finditer(source))
    operations: list[dict[str, object]] = []
    for index, match in enumerate(starts):
        end = starts[index + 1].start() if index + 1 < len(starts) else len(source)
        body = source[match.start() : end]
        method_match = HTTP_METHOD.search(body)
        if not method_match:
            raise ValueError(f"HTTP method not found for {match.group('name')}")

        uri_end = body.find("string uriString")
        uri_block = body[:uri_end] if uri_end >= 0 else body
        pieces: list[str] = []
        for token in re.finditer(
            rf"{PATH_LITERAL.pattern}|{PATH_PARAMETER.pattern}|{PATH_CHAR.pattern}", uri_block
        ):
            literal = token.group(1)
            parameter = token.group("name")
            character = token.group("char")
            if literal is not None:
                pieces.append(bytes(literal, "utf-8").decode("unicode_escape"))
            elif parameter:
                pieces.append("{" + parameter + "}")
            elif character:
                pieces.append(character)

        parameters = match.group("parameters").rstrip().rstrip(",").strip()
        query_names = re.findall(
            r'Uri\.EscapeDataString\("([^"}]+)"\)\)\.Append\(\'=\'', body
        )
        raw_path = "".join(pieces).lstrip("/")
        path = "/" + raw_path if raw_path.startswith("api/") else "/api/" + raw_path
        operations.append(
            {
                "operationId": match.group("name"),
                "method": method_match.group(1),
                "path": path,
                "returnType": match.group("return") or "void",
                "parameters": parameters,
                "queryParameters": sorted(set(query_names)),
                "successStatuses": sorted({int(code) for code in STATUS.findall(body)}),
                "auth": "public"
                if match.group("name") in {"AuthenticateAsync", "RefreshTokenAsync"}
                else "required-by-reimplementation",
            }
        )
    return operations


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--assembly", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--openapi", type=Path)
    args = parser.parse_args()

    operations = parse(args.source.read_text(encoding="utf-8"))
    if len(operations) != 202:
        raise SystemExit(f"expected 202 operations, extracted {len(operations)}")

    manifest = {
        "schemaVersion": 1,
        "product": "SpecterInsight 7.0.0",
        "evidence": {
            "kind": "normalized-clean-room-interface-facts",
            "assemblySha256": sha256(args.assembly),
            "source": "local ILSpy reconstruction; decompiled source is not retained",
            "references": ["FND-036", "FND-040", "FND-041"],
        },
        "operationCount": len(operations),
        "operations": operations,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )
    if args.openapi:
        paths: dict[str, dict[str, object]] = {}
        for operation in operations:
            parameters = [
                {
                    "name": name,
                    "in": "path",
                    "required": True,
                    "schema": {"type": "string"},
                }
                for name in re.findall(r"\{([^}]+)\}", str(operation["path"]))
            ]
            parameters.extend(
                {
                    "name": name,
                    "in": "query",
                    "required": False,
                    "schema": {"type": "string"},
                }
                for name in operation["queryParameters"]
            )
            spec: dict[str, object] = {
                "operationId": operation["operationId"],
                "tags": [str(operation["path"]).split("/")[2]],
                "parameters": parameters,
                "responses": {
                    str(status): {"description": "Observed success response"}
                    for status in operation["successStatuses"]
                },
                "security": []
                if operation["auth"] == "public"
                else [{"bearerAuth": []}],
                "x-specter-return-type": operation["returnType"],
                "x-provenance": "normalized from pinned ApiClient.dll",
            }
            paths.setdefault(str(operation["path"]), {})[str(operation["method"]).lower()] = spec
        openapi = {
            "openapi": "3.1.0",
            "info": {
                "title": "SpecterInsight 7.0.0 clean-room compatibility contract",
                "version": "0.1.0",
            },
            "paths": paths,
            "components": {
                "securitySchemes": {
                    "bearerAuth": {"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}
                }
            },
        }
        args.openapi.parent.mkdir(parents=True, exist_ok=True)
        args.openapi.write_text(
            json.dumps(openapi, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
        )


if __name__ == "__main__":
    main()
