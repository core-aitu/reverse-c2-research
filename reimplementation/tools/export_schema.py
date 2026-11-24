#!/usr/bin/env python3
"""Render the observed PostgreSQL catalog as a deterministic baseline migration."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path


def quote(identifier: str) -> str:
    return '"' + identifier.replace('"', '""') + '"'


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--catalog", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    root = json.loads(args.catalog.read_text(encoding="utf-8"))["catalog"]
    lines = [
        "-- Generated from research/validation/stage5/evidence/postgres-catalog.json.",
        "-- This is a clean-room catalog transcription, not decompiled implementation code.",
        'CREATE EXTENSION IF NOT EXISTS "citext";',
        'CREATE EXTENSION IF NOT EXISTS "pg_trgm";',
        "",
    ]

    for enum in sorted(root["enums"], key=lambda item: item["enum_name"]):
        labels = ", ".join("'" + label.replace("'", "''") + "'" for label in enum["labels"])
        lines.append(f"CREATE TYPE {quote(enum['enum_name'])} AS ENUM ({labels});")
    lines.append("")

    sequences: set[str] = set()
    for table in root["tables"]:
        for column in table["columns"]:
            default = column["default"] or ""
            match = re.search(r"nextval\('([^']+)'::regclass\)", default)
            if match:
                sequences.add(match.group(1))
    for sequence in sorted(sequences):
        lines.append(f"CREATE SEQUENCE {sequence};")
    lines.append("")

    for table in sorted(root["tables"], key=lambda item: item["table_name"]):
        columns = []
        for column in sorted(table["columns"], key=lambda item: item["ordinal"]):
            definition = f"  {quote(column['name'])} {column['type']}"
            if column["default"] is not None:
                definition += f" DEFAULT {column['default']}"
            if column["not_null"]:
                definition += " NOT NULL"
            columns.append(definition)
        lines.append(f"CREATE TABLE {quote(table['table_name'])} (\n" + ",\n".join(columns) + "\n);")
    lines.append("")

    constraint_indexes: set[str] = set()
    # Referenced keys must exist before foreign keys are installed. The observed
    # catalog is table-ordered, not dependency-ordered, so emit keys first.
    for constraint_types in ({"p", "u"}, {"f"}):
        for table in sorted(root["tables"], key=lambda item: item["table_name"]):
            for constraint in table["constraints"]:
                if constraint["type"] not in constraint_types:
                    continue
                lines.append(
                    f"ALTER TABLE {quote(table['table_name'])} ADD CONSTRAINT "
                    f"{quote(constraint['name'])} {constraint['definition']};"
                )
                if constraint["type"] in {"p", "u"}:
                    constraint_indexes.add(constraint["name"])
    lines.append("")

    for table in sorted(root["tables"], key=lambda item: item["table_name"]):
        for index in table["indexes"]:
            if index["name"] not in constraint_indexes:
                lines.append(index["definition"] + ";")

    lines.append("")
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    main()
