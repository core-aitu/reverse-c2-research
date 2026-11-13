"""Snapshot a distribution and verify it against an independent working copy."""
import argparse
import collections
import gzip
import hashlib
import json
import os
from pathlib import Path
import stat
import struct
from datetime import datetime, timezone


def file_format(header, name):
    if header.startswith(b'MZ'):
        if len(header) >= 64:
            off = struct.unpack_from('<I', header, 60)[0]
            if off + 26 < len(header) and header[off:off+4] == b'PE\0\0':
                opt = off + 24
                magic = struct.unpack_from('<H', header, opt)[0]
                directory = {0x10b: 96, 0x20b: 112}.get(magic)
                if directory:
                    clr = opt + directory + 14 * 8
                    if clr + 8 <= len(header):
                        rva, size = struct.unpack_from('<II', header, clr)
                        return 'PE/CLI' if rva and size else 'PE'
        return 'MZ (header unresolved)'
    for sig, label in [(b'\x7fELF','ELF'), (b'BSJB','portable PDB / CLI metadata'),
                       (b'Microsoft C/C++ MSF','MSF/PDB'), (b'PK\x03\x04','ZIP container'),
                       (b'SQLite format 3\0','SQLite'), (b'\x0a\x0d\x0d\x0a','PCAPNG'),
                       (b'\x1f\x8b','gzip'), (b'#!','script'), (b'!<arch>','archive')]:
        if header.startswith(sig):
            return label
    if header[:4].hex() in ('a1b2c3d4','d4c3b2a1','a1b23c4d','4d3cb2a1'):
        return 'PCAP'
    if header[:4].hex() in ('feedface','cefaedfe','feedfacf','cffaedfe','cafebabf','bfbafeca'):
        return 'Mach-O'
    if header[:4] in (b'\xca\xfe\xba\xbe', b'\xbe\xba\xfe\xca') and name.endswith('.dylib'):
        return 'Mach-O universal'
    if header.startswith(b'\0asm'):
        return 'WebAssembly'
    if not Path(name).suffix and header and b'\x00' not in header:
        try:
            header.decode('utf-8-sig')
            return 'text sample (no extension)'
        except UnicodeDecodeError:
            pass
    return 'extension:' + (Path(name).suffix.lower() or '(none)')


def walk(root):
    def visit(directory):
        for path in sorted(directory.iterdir()):
            rel = path.relative_to(root).as_posix()
            if rel == 'research':
                continue
            item = {'path': rel, 'id': 'ART-' + hashlib.sha256(os.fsencode(rel)).hexdigest()[:20]}
            try:
                mode = path.lstat().st_mode
                item["mode"] = oct(stat.S_IMODE(mode))
                if stat.S_ISDIR(mode):
                    item.update(kind='directory', empty=next(path.iterdir(), None) is None)
                elif stat.S_ISREG(mode):
                    h = hashlib.sha256()
                    with path.open('rb') as stream:
                        header = stream.read(8192)
                        h.update(header)
                        size = len(header)
                        while block := stream.read(1024*1024):
                            h.update(block)
                            size += len(block)
                    item.update(kind='file', size=size, sha256=h.hexdigest(), format=file_format(header,path.name))
                elif stat.S_ISLNK(mode):
                    item.update(kind='symlink', target=os.readlink(path))
                else:
                    item['kind'] = 'special'
            except OSError as exc:
                item['error'] = str(exc)
            yield item
            if item.get('kind') == 'directory':
                yield from visit(path)
    return visit(root)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=Path)
    parser.add_argument('copy', type=Path)
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    started = datetime.now(timezone.utc).isoformat()
    original = list(walk(args.source.resolve()))
    duplicate = list(walk(args.copy.resolve()))
    if original != duplicate:
        raise SystemExit('Independent copy differs from source')
    if len({x['id'] for x in original}) != len(original):
        raise SystemExit('Path ID collision')
    args.output.mkdir(parents=True, exist_ok=True)
    with gzip.open(args.output/'manifest.jsonl.gz', 'wt', encoding='utf-8') as f:
        for row in original:
            f.write(json.dumps(row, ensure_ascii=False)+'\n')
    by_hash = collections.defaultdict(list)
    for row in original:
        if 'sha256' in row:
            by_hash[row['sha256']].append(row['id'])
    with gzip.open(args.output/'duplicate-content.jsonl.gz', 'wt', encoding='utf-8') as f:
        for h, ids in sorted(by_hash.items()):
            if len(ids)>1:
                f.write(json.dumps({'sha256':h,'paths':ids})+'\n')
    summary = {
        'started_utc':started, 'finished_utc':datetime.now(timezone.utc).isoformat(),
        'source':str(args.source.resolve()), 'verified_copy':str(args.copy.resolve()),
        'counts':dict(collections.Counter(x.get('kind') for x in original)),
        'regular_bytes':sum(x.get('size',0) for x in original),
        'verified_path_count':len(duplicate),
        'empty_directories':sum(x.get('empty',False) for x in original),
        'unique_file_sha256':len(by_hash), 'duplicate_groups':sum(len(v)>1 for v in by_hash.values()),
        'errors':[x for x in original if 'error' in x],
        'formats':dict(collections.Counter(x['format'] for x in original if 'format' in x)),
        'files_by_root':dict(collections.Counter(x['path'].split('/')[0] for x in original if x.get('kind')=='file'))
    }
    (args.output/'snapshot.json').write_text(json.dumps(summary,indent=2,ensure_ascii=False)+'\n')

if __name__ == '__main__':
    main()
