"""Read selected audit call sites and the provenance of saved database observations."""
import gzip
import hashlib
import json
import sys
from pathlib import Path

import dnfile
from dncil.cil.body.reader import read_method_body_from_bytes

# Byte offsets start at the first instruction, excluding the method header.
SITES = {
    "client/SpecterInsight.UI.dll": {
        0x936: [568, 579, 585, 596],
        0x39: [258, 293],
        0x41: [10, 23, 28],
        0x950: [18, 19, 104, 115, 117, 122, 133, 145, 156, 161, 164],
        0x954: [37, 42, 47, 55, 66, 71, 72, 75, 86, 99, 105, 111, 116, 121, 124, 129, 136],
        0x94e: [32, 114, 121],
        0x952: [7, 13, 24, 35],
        0x942: [1, 6, 12, 13],
        0x3f: [19, 104, 110, 115, 134, 169, 186],
        0x37: [10],
        0x47: [61, 171, 262, 266, 267, 283, 285, 290, 377, 382, 383],
    },
    "client/SpecterInsight.ApiClient.dll": {0x23: [1, 6, 11, 12, 17]},
    "server/SpecterInsight.Server.dll": {0x21d1: [46, 140, 167, 180, 185, 194, 203, 208, 222, 234, 352]},
}


def digest(data):
    return hashlib.sha256(data).hexdigest()


def read_image(path, selection):
    pe = dnfile.dnPE(str(path), clr_lazy_load=True)
    tables = pe.net.mdtables
    nested = {id(n.NestedClass.row): n.EnclosingClass.row for n in tables.NestedClass}
    def type_name(row):
        if not hasattr(row, "TypeName"):
            return "<TypeSpec>"
        if id(row) in nested:
            return type_name(nested[id(row)]) + "+" + str(row.TypeName)
        return ".".join(filter(None, (str(row.TypeNamespace), str(row.TypeName))))

    owners = {id(method.row): type_name(t) for t in tables.TypeDef for method in t.MethodList}

    def member(token):
        row = tables.tables[token >> 24].rows[(token & 0xffffff) - 1]
        if hasattr(row, "Instantiation"):
            row = row.Method.row
        owner = owners.get(id(row))
        if owner is None and hasattr(row, "Class"):
            owner = type_name(row.Class.row)
        return "::".join(filter(None, [owner, str(row.Name)]))

    result = []
    for rid, offsets in selection.items():
        method = tables.MethodDef.rows[rid - 1]
        body = read_method_body_from_bytes(pe.get_data(method.Rva, 65536))
        instructions = {i.offset - body.header_size: i for i in body.instructions}
        selected = []
        for offset in offsets:
            ins = instructions[offset]
            op = ins.opcode.name
            site = {"il_offset": offset, "op": op}
            token = getattr(ins.operand, "value", None)
            if token is not None:
                site["operand_token"] = f"0x{token:08x}"
                site["operand"] = (pe.net.user_strings.get(token & 0xffffff).value
                                   if op == "ldstr" else member(token))
            elif isinstance(ins.operand, int):
                site["operand"] = (ins.operand - body.header_size
                                   if op.startswith(("br", "beq", "bne", "leave"))
                                   else ins.operand)
            selected.append(site)
        code = body.raw_bytes[body.header_size:body.header_size + body.code_size]
        result.append({"method": owners[id(method)] + "::" + str(method.Name),
                       "token": f"0x{0x06000000 + rid:08x}", "il_sha256": digest(code),
                       "eh_count": len(body.exception_handlers), "sites": selected})
    return {"sha256": digest(path.read_bytes()), "mvid": str(tables.Module.rows[0].Mvid),
            "selected_methods": result}


def database_records(research):
    manifest = research / "evidence/manifest.jsonl.gz"
    extensions = (".sql", ".dump", ".backup", ".sql.gz", ".pgdump")
    candidates = []
    with gzip.open(manifest, "rt") as stream:
        for line in stream:
            row = json.loads(line)
            path = row["path"].lower()
            if path.endswith(extensions) or Path(path).name == "pg_version":
                candidates.append(row["path"])
    folder = research / "validation/admin-storage"
    protocol = json.loads((folder / "runs.json").read_text())
    verified = {}
    for run in protocol["runs"]:
        name = run["record"]
        actual = digest((folder / name).read_bytes())
        if actual != run["catalog_and_probes_sha256"]:
            raise ValueError(f"Catalog hash mismatch: {name}")
        verified[name] = actual
    return {"filename_suffixes": extensions, "exact_basename": "PG_VERSION",
            "distribution_manifest_sha256": digest(manifest.read_bytes()), "candidates": candidates,
            "fixture_catalog_sha256": verified, "isolation": protocol["isolation"]}


if __name__ == "__main__":
    root = Path(sys.argv[1])
    research = Path(__file__).resolve().parents[2]
    record = {"images": {p: read_image(root / p, sites) for p, sites in SITES.items()},
              "db": database_records(research)}
    print(json.dumps(record, indent=2))
