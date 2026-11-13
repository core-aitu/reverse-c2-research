"""Independently count selected IL bodies, decoded instructions, sizes and exception regions."""
import argparse
import json
from pathlib import Path

import dnfile
from dncil.cil.body import CilMethodBody
from dncil.cil.body.reader import CilMethodBodyReaderBase


class FileReader(CilMethodBodyReaderBase):
    def __init__(self, stream):
        self.stream=stream

    def read(self, n):
        return self.stream.read(n)

    def tell(self):
        return self.stream.tell()

    def seek(self, offset):
        return self.stream.seek(offset)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('copy',type=Path)
    parser.add_argument('research',type=Path)
    args=parser.parse_args()
    records=[]
    with (args.research/'analysis/il-structure.jsonl').open() as src:
        for ref in map(json.loads,src):
            if not ref['selected']:
                continue
            file=args.copy/ref['paths'][0]
            pe=dnfile.dnPE(str(file),clr_lazy_load=True)
            counts=dict(validated=0,instructions=0,il_bytes=0,with_exception_regions=0,no_rva=0,other_code=0)
            with file.open('rb') as stream:
                reader=FileReader(stream)
                for method in pe.net.mdtables.MethodDef:
                    if not method.Rva:
                        counts['no_rva']+=1
                    elif not method.ImplFlags.miIL:
                        counts['other_code']+=1
                    else:
                        reader.seek(pe.get_offset_from_rva(method.Rva))
                        b=CilMethodBody(reader)
                        counts['validated']+=1
                        counts['instructions']+=len(b.instructions)
                        counts['il_bytes']+=b.code_size
                        counts['with_exception_regions']+=bool(b.exception_handlers)
            pe.close()
            differences={k:{'dncil':v,'metadata_reader':ref[k]} for k,v in counts.items() if ref[k]!=v}
            records.append({'sha256':ref['hash'],'path':ref['paths'][0],**counts,'differences':differences})
    (args.research/'validation/independent-il.json').write_text(json.dumps(records,indent=2)+'\n')
    if any(r['differences'] for r in records):
        raise SystemExit('Decoder counts disagree. See independent-il.json')


if __name__=='__main__':
    main()
