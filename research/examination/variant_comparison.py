"""Compare content identities of packaged variants without producing instructions, names or offsets."""
from collections import Counter
import gzip
import hashlib
from itertools import combinations
import json
from pathlib import Path
import sys

import dnfile
import pefile
from dncil.cil.body import CilMethodBody
from independent_il import FileReader


def method_bodies(path):
    pe=dnfile.dnPE(str(path),clr_lazy_load=True)
    digests=Counter()
    with path.open('rb') as stream:
        reader=FileReader(stream)
        for method in pe.net.mdtables.MethodDef:
            if not method.Rva or not method.ImplFlags.miIL:
                continue
            reader.seek(pe.get_offset_from_rva(method.Rva))
            body=CilMethodBody(reader)
            code=body.raw_bytes[body.header_size:body.header_size+body.code_size]
            digests[hashlib.sha256(code).digest()]+=1
    pe.close()
    return digests


def native_info(path):
    pe=pefile.PE(str(path),fast_load=True)
    sections={}
    for section in pe.sections:
        name=section.Name.rstrip(b'\0').decode('ascii',errors='replace')
        sections.setdefault(name,[]).append(hashlib.sha256(section.get_data()).digest())
    cert=pe.OPTIONAL_HEADER.DATA_DIRECTORY[pefile.DIRECTORY_ENTRY['IMAGE_DIRECTORY_ENTRY_SECURITY']]
    info={'machine':hex(pe.FILE_HEADER.Machine),'is_dll':pe.is_dll(),'section_count':len(pe.sections),
          'certificate_directory_present':bool(cert.VirtualAddress and cert.Size)}
    pe.close()
    return info,sections


def main():
    root,research=map(Path,sys.argv[1:])
    rows=[json.loads(x) for x in gzip.open(research/'evidence/manifest.jsonl.gz','rt')]
    chosen=[r for r in rows if r.get('kind')=='file' and (r['path']=='server/SpecterInsight.exe' or
            r['path'].startswith(('templates/any/','templates/x64/','templates/x86/')))]
    result={}
    for kind in ('PE/CLI','PE'):
        variants={}
        for r in chosen:
            if r['format']!=kind: continue
            variants.setdefault(r['sha256'],[]).append(r['path'])
        props={}; contents={}
        for sha,paths in variants.items():
            path=root/paths[0]
            if kind=='PE/CLI':
                contents[sha]=method_bodies(path)
                props[sha]={'count_il_bodies':sum(contents[sha].values())}
            else:
                props[sha],contents[sha]=native_info(path)
        pairs=[]
        for a,b in combinations(variants,2):
            if kind=='PE/CLI':
                common=sum((contents[a]&contents[b]).values())
                pairs.append({'a':a,'b':b,'identical_il_multiset_intersection':common})
            elif (props[a]['machine'],props[a]['is_dll'])==(props[b]['machine'],props[b]['is_dll']):
                matches=sum(v==contents[b].get(k) for k,v in contents[a].items())
                names=set(contents[a])|set(contents[b])
                pairs.append({'a':a,'b':b,'equal_section_groups':matches,'total_section_names':len(names)})
        result[kind]={'variants':[dict(sha256=k,paths=p,**props[k]) for k,p in variants.items()],'pairs':pairs}
    (research/'evidence/variant-comparison.json').write_text(json.dumps(result,indent=2)+'\n')


if __name__=='__main__':
    main()
