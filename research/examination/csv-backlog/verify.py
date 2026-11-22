"""Bind the CSV quotation diagnosis to local bytes, exports, and repeated observations."""
import argparse
import hashlib
import json
from collections import Counter
from pathlib import Path


def digest(data):
    return hashlib.sha256(data).hexdigest()


def require(condition, explanation):
    if not condition:
        raise ValueError(explanation)


def inspect(root, here):
    il = json.loads((here / "server-il.json").read_text())
    binary = (root / "server/SpecterInsight.Server.dll").read_bytes()
    require(digest(binary) == il["assembly"]["sha256"], "Server hash mismatch")
    hashes = {}
    for method in il["methods"]:
        body = method["body"]
        offset = body["file_offset"]
        header = binary[offset] & 3
        require(header in (2, 3), "Invalid CIL header")
        size = 1 if header == 2 else 4 * (int.from_bytes(binary[offset:offset+2], "little") >> 12)
        start = offset + size
        method_bytes = binary[start:start+body["il_size"]]
        hashes[method["name"]] = digest(method_bytes)
        require(hashes[method["name"]] == body["il_sha256"], "IL body mismatch")
        if method["name"] == "LoadVendors":
            for site, operation in [(69, b"\x1f\x2c"), (87, b"\x1f\x2c"), (168, b'\x1f\x22')]:
                require(method_bytes[site:site+2] == operation, f"Delimiter/quote mismatch {site}")
            calls = {edge["il_offset"]: edge for edge in il["call_edges"] if edge["caller_token"] == method["token"]}
            for site, name in [(71,"IndexOf"), (93,"IndexOf"), (158,"Substring"), (170,"Trim"), (234,"TryAdd")]:
                target = calls[site]
                instruction = bytes([0x6F]) + int(target["target_token"], 16).to_bytes(4, "little")
                require(target["target_name"] == name and method_bytes[site:site+5] == instruction,
                        f"Unexpected callvirt at IL+{site}")

    export = (root / "elastic/elastic/dashboards.ndjson").read_bytes()
    records = [json.loads(line) for line in export.splitlines()]
    objects = [record for record in records if "type" in record]
    encoded = [json.dumps(item, ensure_ascii=False).casefold() for item in objects]
    literals = ["MacAddress_vendor", "PhysicalAddress_vendor", "vendor", "manufacturer", "PhysicalAddress"]
    export_counts = {literal: sum(literal.casefold() in doc for doc in encoded) for literal in literals}
    ui = (root / "client/SpecterInsight.UI.dll").read_bytes()
    ui_counts = {literal: {enc: ui.lower().count(literal.lower().encode(enc)) for enc in ("utf-8", "utf-16-le")}
                 for literal in ["MacAddress_vendor", "PhysicalAddress_vendor"]}
    runs = [(here / f"runtime-{i}.json").read_bytes() for i in (1, 2)]
    expected = {"distinct":39527, "matched":22829, "commas":16662, "missing":35,
                "other":[{"prefix":"2C7AFE","expected":'IEE&E "Black" ops',"actual":'IEE&E ""Black"" ops'}]}
    require(runs[0] == runs[1] and json.loads(runs[0]) == expected, "CSV observations differ")
    return {"server_sha256":digest(binary), "il_sha256":hashes, "ui_sha256":digest(ui),
            "catalog_sha256":digest((root / "resources/mac-vendors-export.csv").read_bytes()),
            "kibana_export_sha256":digest(export), "kibana_saved_object_types":dict(Counter(o["type"] for o in objects)),
            "kibana_trailer_records":len(records)-len(objects), "saved_objects_matching_literal_case_insensitive":export_counts,
            "ui_pe_exact_literal_hits_case_insensitive":ui_counts, "matching_run_sha256":digest(runs[0])}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("distribution", type=Path)
    args = parser.parse_args()
    print(json.dumps(inspect(args.distribution, Path(__file__).resolve().parent), indent=2))
