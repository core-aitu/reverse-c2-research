"""Count module metadata without loading the assemblies. Requires dnfile."""
import hashlib
import json
import sys
from pathlib import Path

import dnfile


def inventory(root):
    catalog = Path(__file__).resolve().parents[2] / 'validation/continuation/variants-and-modules.json'
    known = {entry['path']: entry for entry in json.loads(catalog.read_text())['modules']}
    paths = sorted(p for p in (root / 'modules').rglob('*') if p.is_file())
    if {p.relative_to(root).as_posix() for p in paths} != set(known):
        raise ValueError('module directory differs from reference catalog')

    rows = []
    for path in paths:
        rel = path.relative_to(root).as_posix()
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        if digest != known[rel]['sha256']:
            raise ValueError(rel)
        image = dnfile.dnPE(str(path))
        try:
            tables = image.net.mdtables
            flags = [row.Flags for row in tables.TypeDef.rows]
            def count(table):
                return len(table.rows) if table is not None else 0
            rows.append(dict(path=rel, sha256=digest, mvid=known[rel]['mvid'],
                             top_level_public_types=sum(f.tdPublic for f in flags),
                             declared_interfaces=sum(f.tdInterface for f in flags),
                             direct_interface_impl_rows=count(tables.InterfaceImpl),
                             exported_type_table_rows=count(tables.ExportedType),
                             method_definitions=count(tables.MethodDef),
                             observed_runtime_load=None))
        finally:
            image.close()
    return rows


if __name__ == '__main__':
    print(json.dumps(inventory(Path(sys.argv[1])), indent=2))
