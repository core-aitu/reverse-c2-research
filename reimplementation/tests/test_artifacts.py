import json
import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
RESEARCH = ROOT.parent / "research"


class GeneratedArtifactTests(unittest.TestCase):
    def test_contract_manifest_is_complete(self) -> None:
        manifest = json.loads((ROOT / "contracts/manifest.json").read_text())
        self.assertEqual(202, manifest["operationCount"])
        self.assertEqual(202, len(manifest["operations"]))
        identities = {(item["method"], item["path"]) for item in manifest["operations"]}
        self.assertEqual(202, len(identities))

    def test_openapi_has_every_operation(self) -> None:
        document = json.loads((ROOT / "contracts/openapi.json").read_text())
        count = sum(len(methods) for methods in document["paths"].values())
        self.assertEqual(202, count)

    def test_baseline_matches_observed_catalog(self) -> None:
        catalog = json.loads(
            (RESEARCH / "validation/stage5/evidence/postgres-catalog.json").read_text()
        )["catalog"]
        migration = (ROOT / "migrations/001_legacy_baseline.sql").read_text()
        self.assertEqual(len(catalog["tables"]), len(re.findall(r"^CREATE TABLE ", migration, re.M)))
        self.assertEqual(len(catalog["enums"]), len(re.findall(r"^CREATE TYPE ", migration, re.M)))
        observed_indexes = sum(len(table["indexes"]) for table in catalog["tables"])
        constraint_indexes = sum(
            1
            for table in catalog["tables"]
            for constraint in table["constraints"]
            if constraint["type"] in {"p", "u"}
        )
        explicit_indexes = len(re.findall(r"^CREATE (?:UNIQUE )?INDEX ", migration, re.M))
        self.assertEqual(observed_indexes, constraint_indexes + explicit_indexes)

    def test_management_plane_migration_has_isolated_runtime_tables(self) -> None:
        migration = (ROOT / "migrations/003_management_plane.sql").read_text()
        for table in (
            "app_resources", "app_adapters", "app_jobs", "app_pipeline_definitions",
            "app_agent_sessions", "app_agent_tasks",
        ):
            self.assertIn(f"CREATE TABLE {table}", migration)
        self.assertIn("fixture-safe-transform", migration)

    def test_typed_catalog_dtos_are_pinned_to_managed_evidence(self) -> None:
        catalog = json.loads((ROOT / "contracts/dto-catalog.json").read_text())
        self.assertEqual(
            "5c658f03289e5804a692ee66ca7beff3883631f4a3d81b301bdade0e7fa45422",
            catalog["artifact"]["sha256"],
        )
        self.assertFalse(catalog["provenance"]["executed"])
        self.assertEqual(
            ["identifier", "name", "description", "documentation", "scriptBlock", "version", "isPremium", "labels", "enabled"],
            catalog["types"]["SpecterScript"],
        )
        self.assertEqual(8, len(catalog["types"]["WordlistResponse"]))
        self.assertEqual(9, len(catalog["types"]["CertificateInfo"]))


if __name__ == "__main__":
    unittest.main()
