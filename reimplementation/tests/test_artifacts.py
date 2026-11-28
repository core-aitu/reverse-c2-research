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

    def test_content_package_migration_enforces_bounds_and_digests(self) -> None:
        migration = (ROOT / "migrations/008_content_packages.sql").read_text()
        self.assertIn("CREATE TABLE app_artifact_contents", migration)
        self.assertIn("CREATE TABLE app_update_packages", migration)
        self.assertIn("octet_length(content) BETWEEN 1 AND 16777216", migration)
        self.assertIn("octet_length(content) BETWEEN 1 AND 33554432", migration)
        self.assertEqual(2, migration.count("sha256 ~ '^[0-9a-f]{64}$'"))
        self.assertIn("app_update_packages_one_active", migration)

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

    def test_agent_p1_migration_pins_lifecycle_replay_and_rotation(self) -> None:
        migration = (ROOT / "migrations/010_agent_gateway_p1.sql").read_text()
        for state in ("queued", "leased", "running", "completed", "failed", "cancelled"):
            self.assertIn(f"'{state}'", migration)
        self.assertIn("UNIQUE (agent_id,idempotency_key)", migration)
        self.assertIn("CREATE TABLE app_agent_request_nonces", migration)
        self.assertIn("previous_identity_valid_until", migration)
        self.assertIn("submission_digest", migration)

    def test_beacon_release_targets_are_reproducible_and_versioned(self) -> None:
        makefile = (ROOT / "Makefile").read_text()
        self.assertIn("-trimpath -buildvcs=false", makefile)
        self.assertIn("-buildid= -X main.buildVersion=$(VERSION)", makefile)
        self.assertIn("verify-reproducible", makefile)
        for target in ("linux-amd64", "linux-arm64", "windows-amd64.exe", "darwin-arm64"):
            self.assertIn(target, makefile)
        self.assertIn("specter-beacon.spdx.json", makefile)


if __name__ == "__main__":
    unittest.main()
