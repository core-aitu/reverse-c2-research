import copy
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'tools'))
import research_reports as reports
import evidence_integrity as evidence


def sample():
    return {"schema": "lab-telemetry-audit/v1", "eventCount": 3,
            "sessions": [{"session": "lab-a", "state": "closed", "firstEvent": "2026-01-01T00:00:00Z", "lastEvent": "2026-01-01T00:01:00Z", "observations": 1, "validObservations": 1}], "issues": []}


class ReportTests(unittest.TestCase):
    def test_summary(self):
        result = reports.summary(sample())
        self.assertEqual(3, result['events'])
        self.assertEqual(1, result['states']['closed'])
        self.assertEqual(1, result['validObservations'])

    def test_empty(self):
        data = dict(sample(), eventCount=0, sessions=[])
        self.assertEqual(0, reports.summary(data)['sessions'])

    def test_comparison(self):
        old = sample()
        new = copy.deepcopy(old)
        new['eventCount'] = 2
        new['sessions'][0]['state'] = 'open'
        result = reports.compare(old, new)
        self.assertEqual(-1, result['delta']['events'])
        self.assertEqual('open', result['changedSessions'][0]['after']['state'])
        self.assertEqual([], reports.compare(old, old)['changedSessions'])

    def test_issue_comparison(self):
        new = sample()
        new['eventCount'] += 1
        new['issues'].append({'eventId': 'x', 'session': 'lab-a', 'code': 'unexpected_open'})
        self.assertEqual(new['issues'], reports.compare(sample(), new)['addedIssues'])

    def test_unknown_session_issues(self):
        data = dict(sample(), eventCount=1, sessions=[dict(sample()['sessions'][0], state='unknown', observations=0, validObservations=0)], issues=[{'eventId':'x', 'session':'lab-a','code':'unexpected_close'}])
        self.assertEqual(1, reports.summary(data)['issues'])

    def test_unknown_session_requires_own_event(self):
        data = sample()
        data['sessions'].append(dict(data['sessions'][0], session='other', state='unknown', observations=0, validObservations=0))
        data['eventCount'] += 1
        data['issues'].append({'eventId':'x','session':'lab-a','code':'unexpected_close'})
        with self.assertRaises(ValueError):
            reports.validate(data)

    def test_invalid_values(self):
        for key, value in [('eventCount', True), ('eventCount', -1), ('eventCount', 4), ('schema', 'other'), ('extra', 1)]:
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                reports.validate(dict(sample(), **{key: value}))

    def test_invalid_observation_count(self):
        data = sample()
        data['sessions'][0]['validObservations'] = 0
        with self.assertRaises(ValueError):
            reports.validate(data)

    def test_unknown_issue_session(self):
        data = sample()
        data['issues'] = [{'eventId':'x','session':'missing','code':'unexpected_close'}]
        with self.assertRaises(ValueError):
            reports.validate(data)

    def test_timestamp_precision(self):
        data = sample()
        data['sessions'][0].update(firstEvent='2026-01-01T00:00:00.000000002Z', lastEvent='2026-01-01T00:00:00.000000001Z')
        with self.assertRaises(ValueError):
            reports.validate(data)
        self.assertEqual(reports.timestamp('2026-01-01T01:00:00+01:00'), reports.timestamp('2026-01-01T00:00:00Z'))

    def test_duplicate_json_keys_and_limits(self):
        with tempfile.TemporaryDirectory() as folder:
            p = Path(folder) / 'input.json'
            p.write_text('{"x":1,"x":2}')
            with self.assertRaises(ValueError):
                reports.read_json(p)
            with self.assertRaises(ValueError):
                reports.read_json(p, 3)

    def test_html(self):
        html = reports.render_html(sample())
        self.assertIn('Content-Security-Policy', html)
        self.assertIn('lab-a', html)
        self.assertNotIn('<script', html)
        data = sample()
        data['sessions'][0]['session'] = '<script>'
        with self.assertRaises(ValueError):
            reports.render_html(data)


class EvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        for folder in evidence.DIRECTORIES:
            (self.root / folder).mkdir(parents=True)
        self.doc = self.root / evidence.DIRECTORIES[0] / 'example.md'
        self.doc.write_text('test')

    def test_repository_baseline(self):
        expected = reports.read_json(evidence.ROOT / 'reimplementation/docs/research-document-inventory.json')
        self.assertTrue(evidence.verify(expected, evidence.inventory(evidence.ROOT))['ok'])

    def test_snapshot_and_verify(self):
        snapshot = evidence.inventory(self.root)
        self.assertEqual(1, len(snapshot['files']))
        self.assertTrue(evidence.verify(snapshot, evidence.inventory(self.root))['ok'])

    def test_changes(self):
        before = evidence.inventory(self.root)
        self.doc.write_text('modified')
        self.assertEqual([before['files'][0]['path']], evidence.verify(before, evidence.inventory(self.root))['changed'])
        self.doc.unlink()
        (self.doc.parent / 'new.md').write_text('new')
        result = evidence.verify(before, evidence.inventory(self.root))
        self.assertEqual(1, len(result['missing']))
        self.assertEqual(1, len(result['added']))
        self.assertFalse(result['ok'])

    def test_path_scope_and_duplicates(self):
        before = evidence.inventory(self.root)
        bad = copy.deepcopy(before)
        bad['files'][0]['path'] = '../outside.md'
        with self.assertRaises(ValueError):
            evidence.verify(bad, before)
        before['files'] *= 2
        with self.assertRaises(ValueError):
            evidence.verify(before, before)

    def test_symlinks_rejected(self):
        (self.doc.parent / 'alias.md').symlink_to(self.doc)
        with self.assertRaises(ValueError):
            evidence.inventory(self.root)


if __name__ == '__main__':
    unittest.main()
