"""Produce aggregate metadata coverage, configuration and package evidence. No target execution."""
import collections
import gzip
import hashlib
import json
from pathlib import Path
import sys
import zipfile

import dnfile
import pefile
import yaml
from dncil.cil.body.reader import read_method_body_from_bytes

root, out=map(Path,sys.argv[1:])
def save(name,value):
    (out/name).write_text(json.dumps(value,indent=2,ensure_ascii=False)+'\n')
def j(path):
    return json.loads((root/path).read_text(encoding='utf-8-sig'))
def rows(path):
    return [json.loads(l) for l in (out/path).read_text().splitlines()]
manifest={r['path']:r for r in map(json.loads,gzip.open(out/'evidence/manifest.jsonl.gz','rt'))}
def ref(path):
    return {'path':path,'id':manifest[path]['id']}
assemblies=rows('analysis/assemblies.jsonl')
selected=collections.defaultdict(list)
for a in assemblies:
    if a.get('own_candidate'):
        selected[a['hash']].append(a)
method_groups=json.loads((out/'analysis/method-evidence.json').read_text())
reviewed={a['sha256']:len(a['methods']) for a in method_groups}
by_hash=collections.defaultdict(lambda:[0,0,0])
for r in map(json.loads,gzip.open(out/'analysis/type-method-index.jsonl.gz','rt')):
    b=by_hash[r['hash']]
    b[0]+=1
    b[1]+=r['method_count']
    b[2]+=len(r['named_methods'] or [])
coverage=[]
for sha, group in selected.items():
    first=group[0]
    image=dnfile.dnPE(str(root/first['path']),clr_lazy_load=True)
    tables=image.net.mdtables
    sample_ok=0
    for method in tables.MethodDef:
        if not method.Rva or not method.ImplFlags.miIL:
            continue
        try:
            b=read_method_body_from_bytes(image.get_data(method.Rva,65536))
            sample_ok+=int(bool(b.instructions))
        except (ValueError,IndexError): pass
        if sample_ok==3: break
    counts=by_hash[sha]
    assert counts[:2]==[first['types'],first['methods']],first['path']
    coverage.append({'paths':[x['path'] for x in group],'sha256':sha,'mvid':first['mvid'],
                     'metadata_methods':first['methods'],'indexed_methods':counts[1],
                     'named_methods':counts[2],'il_reviewed':reviewed.get(sha,0),
                     'indexed_types':counts[0],'il_readability_sample':sample_ok,
                     'pinvoke_declarations':tables.ImplMap.num_rows if tables.ImplMap else 0})
    image.close()
structure_path=out/'analysis/il-structure.jsonl'
if structure_path.is_file():
    structure={entry['hash']:entry for entry in rows('analysis/il-structure.jsonl')}
    for entry in coverage:
        decoded=structure[entry['sha256']]
        assert decoded['methods']==entry['metadata_methods']
        entry['structural_il_bodies']=decoded['validated']
        entry['without_rva']=decoded['no_rva']
        entry['other_code']=decoded['other_code']
        entry['decode_failures']=sum(decoded['failures'].values())
        entry['direct_field_getters']=decoded['direct_field_getters']
        entry['direct_field_setters']=decoded['direct_field_setters']
save('validation/coverage.json',coverage)
runtime=[]
for path in sorted(x for x in manifest if x.endswith('.runtimeconfig.json')):
    opts=j(path)['runtimeOptions']
    runtime.append(dict(**ref(path),tfm=opts.get('tfm'),frameworks=opts.get('frameworks',[opts.get('framework')]),
                        property_names=list(opts.get('configProperties',{}))))
compose_path='vm/docker-compose.yml'
compose=yaml.safe_load((root/compose_path).read_text())['services']
configs=[]
for path in ['settings/settings.json','vm/settings.json']:
    configs.append(dict(**ref(path),keys={k:sorted(t.keys()) for k,t in j(path).items() if isinstance(t,dict)}))
data={'settings':configs,'runtime':runtime,'deployment':dict(**ref(compose_path),services={
    n:{'image':s.get('image'),'depends_on':list(s.get('depends_on',[]))} for n,s in compose.items()}),
    'launch':[dict(**ref(path),line=i,value=line.strip()) for path,key in [
        ('vm/specter-insight.service','ExecStart='),('vm/SpecterInsight.desktop','Exec=')]
        for i,line in enumerate((root/path).read_text().splitlines(),1) if line.startswith(key)]}
extras={}
for folder in sorted({p.split('/')[0] for p in manifest}):
    files=[r for p,r in manifest.items() if p.split('/')[0]==folder and r['kind']=='file']
    extras[folder]={'files':len(files),'formats':dict(collections.Counter(r['format'] for r in files))}
data['roots']=extras
data['elastic_saved_objects']=dict(**ref('elastic/elastic/dashboards.ndjson'),records=dict(collections.Counter(
    str(json.loads(l).get('type','(export metadata)')) for l in (root/'elastic/elastic/dashboards.ndjson').read_text().splitlines() if l)))
capture_ext={'.pcap','.cap','.pcapng','.har','.saz','.etl','.nettrace','.qlog'}
database_ext={'.db','.sqlite','.sqlite3','.backup','.pgdump','.sql','.dump','.mdf'}
data['capture_candidates']=[r['path'] for r in manifest.values() if r.get('kind')=='file' and
    (r['format'] in {'PCAP','PCAPNG'} or Path(r['path']).suffix.lower() in capture_ext)]
data['database_candidates']=[r['path'] for r in manifest.values() if r.get('kind')=='file' and
    (r['format']=='SQLite' or Path(r['path']).suffix.lower() in database_ext or Path(r['path']).name=='PG_VERSION')]
data['backup_paths']=[p for p,r in manifest.items() if r['kind']=='file' and ('bak' in Path(p).suffix.lower())]
save('examination/distribution.json',data)
