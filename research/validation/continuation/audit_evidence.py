"""Audit retained research records without loading or executing product assemblies."""
import argparse
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
from semantic_cards import validate_ui_cards


ROOT = Path(__file__).resolve().parents[2]


def sha256(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def read_rows(path):
    return [json.loads(line) for line in path.read_text().splitlines() if line.strip()]


def audit(root):
    observations = read_rows(root / 'evidence/observations.jsonl')
    findings = read_rows(root / 'validation/findings.jsonl')
    errors = []
    for name, rows in [('observations', observations), ('findings', findings)]:
        counts = Counter(row['id'] for row in rows)
        errors.extend(f'duplicate {name}: {key}' for key, count in counts.items() if count != 1)
    observation_ids = {row['id'] for row in observations}
    record_refs = set()
    for row in observations:
        for record in row['records']:
            record_refs.add(record)
            if not (root / record).is_file():
                errors.append(f"{row['id']}: missing record {record}")
    for row in findings:
        if row['status'] not in {'Verified', 'Inferred', 'Unverified', 'Contradicted'}:
            errors.append(f"{row['id']}: unknown status")
        if not row['observations']:
            errors.append(f"{row['id']}: no observation reference")
        for ref in row['observations']:
            if ref not in observation_ids:
                errors.append(f"{row['id']}: missing observation {ref}")
    pairs = []
    method_sources = {}
    for source in ['analysis/method-evidence.json', 'validation/continuation/diagnostic-variants.json']:
        groups = json.loads((root / source).read_text())
        count = 0
        for group in groups:
            for method in group['methods']:
                pairs.append((group['sha256'], group['mvid'], int(method['token'], 16)))
                count += 1
        method_sources[source] = {'individual_il_records': count, 'sha256': sha256(root / source)}
    if len(set(pairs)) != len(pairs):
        errors.append('duplicate assembly/MVID/method identity')
    legacy_count = len(set(pairs))
    semantics = validate_ui_cards(root)
    ui_source = 'analysis/semantic-cards/documentation-ui.json'
    ui_cards = json.loads((root / ui_source).read_text())['cards']
    all_pairs = set(pairs) | {(c['assembly_sha256'], c['mvid'], int(c['token'], 16)) for c in ui_cards}
    method_sources[ui_source] = {'individual_cards': len(ui_cards),
                               'additional_individual_il_records': semantics['additional_individual_il_records'],
                               'sha256': sha256(root / ui_source)}
    archives = []
    for folder in [root.parent / 'local-archive', root.parent / 'local-archive/continuation']:
        sums = folder / 'SHA256SUMS'
        if not sums.is_file():
            archives.append({'path': str(sums.relative_to(root.parent)), 'available': False})
            continue
        for line in sums.read_text().splitlines():
            expected, name = line.split(maxsplit=1)
            path = folder / name.lstrip('*')
            actual = sha256(path) if path.is_file() else None
            archives.append({'path': str(path.relative_to(root.parent)), 'available': actual is not None,
                             'expected_sha256': expected, 'actual_sha256': actual,
                             'matches': actual == expected})
            if actual is not None and actual != expected:
                errors.append(f'archive hash mismatch: {path.name}')
    sources = ['evidence/observations.jsonl', 'validation/findings.jsonl',
               'validation/continuation/matrix.json', 'validation/continuation/verify.py',
               'validation/continuation/audit_evidence.py',
               'validation/continuation/test_retained_evidence.py',
               'validation/continuation/semantic_cards.py']
    return {
        'checked_utc': datetime.now(timezone.utc).isoformat(),
        'scope': 'Retained record integrity and local archive availability. No new runtime experiment.',
        'observations': len(observations), 'findings': len(findings),
        'finding_status_counts': dict(Counter(row['status'] for row in findings)),
        'distinct_referenced_record_paths': len(record_refs),
        'method_sources': method_sources, 'prior_individual_il_records': legacy_count,
        'distinct_individual_il_records': len(all_pairs),
        'complete_semantic_card_count': semantics['complete_within_method'],
        'partial_semantic_card_count': semantics['partial'],
        'semantic_card_limit': semantics['scope'],
        'archives': archives,
        'historical_temp_workspace_available': Path('/tmp/specter-study-8ULlfzgo').is_dir(),
        'source_sha256': {source: sha256(root / source) for source in sources},
        'errors': errors,
        'limits': [
            'Existing record paths and OBS references do not prove that claims are supported by their content.',
            'Archive hashes do not establish fresh buildability or execution equivalence.',
            'Distribution-wide hashes and artifact IDs are not revalidated by this audit.',
            'Historical runtime logs are retained evidence, not newly executed scenarios.',
        ],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path)
    args = parser.parse_args()
    result = audit(ROOT)
    text = json.dumps(result, ensure_ascii=False, indent=2) + '\n'
    if args.output:
        args.output.write_text(text)
    else:
        print(text, end='')
    if result['errors']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
