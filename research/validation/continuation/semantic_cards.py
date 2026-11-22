"""Validate evidence bindings and required fields of the reviewed UI cards.

This checks card structure and byte identity. It does not automate semantic review.
"""
import hashlib
import json


def validate_ui_cards(root, distribution=None):
    def read(path):
        return json.loads((root / path).read_text())

    source = 'analysis/semantic-cards/documentation-ui.json'
    document = read(source)
    cards = document['cards']
    required = {'id', 'assembly_path', 'assembly_sha256', 'mvid', 'token', 'signature',
                'signature_metadata', 'il_sha256', 'il_size', 'inputs', 'result',
                'state_changes', 'error_branches', 'calls', 'exception_regions',
                'il_offsets', 'il_evidence', 'observations', 'card_status',
                'verification_status', 'basis', 'confidence', 'dynamic_verification', 'limits'}
    keys = set()
    complete = partial = 0
    prior = read('validation/continuation/administrative-call-sites.json')
    for card in cards:
        missing = required - card.keys()
        if missing:
            raise ValueError(f"{card.get('id')}: missing {sorted(missing)}")
        key = (card['assembly_sha256'], card['mvid'], int(card['token'], 16))
        if key in keys:
            raise ValueError('duplicate method card')
        keys.add(key)
        page = read(card['il_evidence']['path'])['normalized_result']
        identity = page['identity_scope']
        if (identity['requires_artifact_sha256'], identity['requires_mvid']) != key[:2]:
            raise ValueError('evidence identity mismatch')
        method = next(m for m in page['page']['rows'] if m['token'] == card['token'])
        body = method['body']
        if body['truncated_instructions'] or body['instruction_count'] != body['decoded_instruction_count']:
            raise ValueError('incomplete instruction evidence')
        if (body['il_sha256'], body['il_size'], body['exception_regions'], method['signature']) != (
                card['il_sha256'], card['il_size'], card['exception_regions'], card['signature_metadata']):
            raise ValueError('method evidence mismatch')
        if not set(card['il_offsets']) <= {i['il_offset'] for i in body['anchors']}:
            raise ValueError('unknown IL coordinate')
        if key[2] in {0x06000914, 0x06000915, 0x0600091c}:
            check = read('examination/documentation-ui/callback-body-check.json')
            if (check['assembly_sha256'], check['mvid']) != key[:2]:
                raise ValueError('independent PE identity mismatch')
            peers = check['methods']
        else:
            peers = prior[card['assembly_path']]['methods']
        previous = next(m for m in peers if int(m['token'], 16) == key[2])
        if previous['il_sha256'] != card['il_sha256']:
            raise ValueError('independent IL digest differs')
        for field in ['signature', 'inputs', 'result', 'state_changes', 'error_branches',
                      'observations', 'basis', 'confidence', 'limits']:
            if not card[field]:
                raise ValueError(f'empty required field: {field}')
        if card['card_status'] == 'complete_within_method':
            complete += 1
        elif card['card_status'] == 'partial':
            partial += 1
        else:
            raise ValueError('unknown card status')
        if distribution is not None:
            binary = (distribution / card['assembly_path']).read_bytes()
            if hashlib.sha256(binary).hexdigest() != card['assembly_sha256']:
                raise ValueError('assembly digest mismatch')
            offset = body['file_offset']
            if binary[offset] & 3 == 2:
                header_size = 1
            elif binary[offset] & 3 == 3:
                header_size = (int.from_bytes(binary[offset:offset + 2], 'little') >> 12) * 4
            else:
                raise ValueError('unknown IL header')
            code = binary[offset + header_size:offset + header_size + card['il_size']]
            if hashlib.sha256(code).hexdigest() != card['il_sha256']:
                raise ValueError('file-backed IL digest mismatch')
    old_keys = set()
    for path in ['analysis/method-evidence.json', 'validation/continuation/diagnostic-variants.json']:
        for group in read(path):
            old_keys.update((group['sha256'], group['mvid'], int(m['token'], 16)) for m in group['methods'])
    by_image = {}
    for card in cards:
        sha = card['assembly_sha256']
        counts = by_image.setdefault(sha, {'additional_individual_il_records': 0,
                                         'complete_within_method': 0, 'partial': 0})
        counts[card['card_status']] += 1
        if (sha, card['mvid'], int(card['token'], 16)) not in old_keys:
            counts['additional_individual_il_records'] += 1
    return {'cards': len(cards), 'complete_within_method': complete, 'partial': partial,
            'additional_individual_il_records': len(keys - old_keys),
            'distinct_individual_il_records': len(keys | old_keys),
            'by_image': by_image,
            'card_file_sha256': hashlib.sha256((root / source).read_bytes()).hexdigest(),
            'scope': 'Reviewed local method semantics and declared dependency/error limits. Dynamic evidence has its own counters.'}
