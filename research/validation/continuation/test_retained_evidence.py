"""Regression checks for incomplete or duplicated retained log records."""
import copy
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

from verify import jsonl, validate_retained_rows
from semantic_cards import validate_ui_cards


HERE = Path(__file__).resolve().parent


class RetainedLogChecks(unittest.TestCase):
    def setUp(self):
        self.logs = {
            name: jsonl(HERE / name / f'{prefix}-1.jsonl')
            for name, prefix in [('diagnostics', 'logentry'), ('loopback', 'loop'), ('desktop', 'desktop')]
        }

    def test_original_logs(self):
        for name, rows in self.logs.items():
            validate_retained_rows(name, rows)

    def test_missing_pass_is_rejected(self):
        for name in ['diagnostics', 'loopback']:
            with self.subTest(name=name):
                rows = copy.deepcopy(self.logs[name])
                del rows[0]['passed']
                with self.assertRaises(AssertionError):
                    validate_retained_rows(name, rows)

    def test_false_and_nonboolean_pass_are_rejected(self):
        for value in [False, 1, 'true', None]:
            with self.subTest(value=value):
                rows = copy.deepcopy(self.logs['loopback'])
                rows[0]['passed'] = value
                with self.assertRaises(AssertionError):
                    validate_retained_rows('loopback', rows)

    def test_duplicate_records_are_rejected(self):
        for name, rows in self.logs.items():
            with self.subTest(name=name):
                with self.assertRaises(AssertionError):
                    validate_retained_rows(name, rows + [rows[0]])

    def test_empty_logs_are_rejected(self):
        for name in self.logs:
            with self.assertRaises(AssertionError):
                validate_retained_rows(name, [])

    def test_optimized_interpreter_is_rejected(self):
        result = subprocess.run([sys.executable, '-O', str(HERE / 'verify.py')],
                                capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Run without -O', result.stderr)


class SemanticCardChecks(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='semantic-card-check-')
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.card_path = self.root / 'analysis/semantic-cards/documentation-ui.json'
        for name in ['analysis/semantic-cards/documentation-ui.json',
                     'analysis/method-evidence.json',
                     'validation/continuation/diagnostic-variants.json',
                     'validation/continuation/administrative-call-sites.json',
                     'examination/documentation-ui/rea-top-methods.json',
                     'examination/documentation-ui/rea-callback-method.json',
                     'examination/documentation-ui/callback-body-check.json',
                     'examination/documentation-ui/rea-async-methods.json']:
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(HERE.parents[1] / name, target)
        self.document = json.loads(self.card_path.read_text())

    def reject_modified(self):
        self.card_path.write_text(json.dumps(self.document))
        with self.assertRaises(ValueError):
            validate_ui_cards(self.root)

    def test_original_cards_and_union(self):
        result = validate_ui_cards(self.root)
        self.assertEqual((result['cards'], result['complete_within_method'], result['partial']), (7, 7, 0))
        self.assertEqual(result['additional_individual_il_records'], 6)
        self.assertEqual(result['distinct_individual_il_records'], 127)

    def test_duplicate_identity(self):
        self.document['cards'].append(copy.deepcopy(self.document['cards'][0]))
        self.reject_modified()

    def test_missing_error_analysis(self):
        del self.document['cards'][0]['error_branches']
        self.reject_modified()

    def test_wrong_assembly_identity(self):
        self.document['cards'][0]['assembly_sha256'] = '0' * 64
        self.reject_modified()

    def test_wrong_il_digest(self):
        self.document['cards'][0]['il_sha256'] = '0' * 64
        self.reject_modified()

    def test_invalid_il_coordinate(self):
        self.document['cards'][0]['il_offsets'].append(999999)
        self.reject_modified()

    def test_empty_state_description(self):
        self.document['cards'][0]['state_changes'] = []
        self.reject_modified()


if __name__ == '__main__':
    unittest.main()
