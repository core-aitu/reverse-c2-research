#!/usr/bin/env python3
"""Generate a deterministic SPDX 2.3 dependency inventory from go.sum."""
import argparse
import hashlib
import json
from pathlib import Path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    modules = {}
    for line in (root / "go.sum").read_text().splitlines():
        fields = line.split()
        if len(fields) != 3 or fields[1].endswith("/go.mod"):
            continue
        modules[(fields[0], fields[1])] = fields[2]
    packages = [{
        "SPDXID": "SPDXRef-Package-beacon",
        "name": "specter-beacon",
        "versionInfo": args.version,
        "downloadLocation": "NOASSERTION",
        "filesAnalyzed": False,
    }]
    relationships = []
    for index, ((name, version), checksum) in enumerate(sorted(modules.items()), 1):
        spdx_id = f"SPDXRef-Dependency-{index}"
        packages.append({
            "SPDXID": spdx_id,
            "name": name,
            "versionInfo": version,
            "downloadLocation": f"https://proxy.golang.org/{name}/@v/{version}.zip",
            "filesAnalyzed": False,
            "externalRefs": [{"referenceCategory": "PACKAGE-MANAGER", "referenceType": "purl", "referenceLocator": f"pkg:golang/{name}@{version}"}],
            "annotations": [{"annotationType": "OTHER", "annotator": "Tool: generate_sbom.py", "annotationDate": "1970-01-01T00:00:00Z", "comment": f"go.sum={checksum}"}],
        })
        relationships.append({"spdxElementId": "SPDXRef-Package-beacon", "relationshipType": "DEPENDS_ON", "relatedSpdxElement": spdx_id})
    namespace_seed = hashlib.sha256((args.version + "\n" + "\n".join(f"{k[0]} {k[1]} {v}" for k, v in sorted(modules.items()))).encode()).hexdigest()
    document = {
        "spdxVersion": "SPDX-2.3",
        "dataLicense": "CC0-1.0",
        "SPDXID": "SPDXRef-DOCUMENT",
        "name": f"specter-beacon-{args.version}",
        "documentNamespace": f"https://specter.invalid/spdx/{namespace_seed}",
        "creationInfo": {"created": "1970-01-01T00:00:00Z", "creators": ["Tool: generate_sbom.py"]},
        "packages": packages,
        "relationships": relationships,
    }
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(document, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
