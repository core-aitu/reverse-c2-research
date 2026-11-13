"""Verify retained evidence, repeated runs, the bounded capture search and the six-level matrix."""
import argparse
from collections import Counter
import gzip
import hashlib
import json
from pathlib import Path

here=Path(__file__).resolve().parent
r=here.parents[1]
def jsonl(p):return [json.loads(v) for v in p.read_text().splitlines()]
def load(p):return json.loads(p.read_text())
def digest(data):return hashlib.sha256(data).hexdigest()

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('distribution',type=Path)
    parser.add_argument('--write',action='store_true')
    args=parser.parse_args()
    with gzip.open(r/'evidence/manifest.jsonl.gz','rt') as f:
        manifest={row['path']:row for row in map(json.loads,f)}
    groups=load(here/'diagnostic-variants.json')
    variants=load(here/'variants-and-modules.json')
    kernels=load(here/'administrative-call-sites.json')
    old=load(r/'analysis/method-evidence.json')
    assert sum(len(a['methods']) for a in old)==56
    for a in old:
        assert manifest[a['path']]['sha256']==a['sha256']
    result={}
    for name,pat in [('diagnostics','logentry'),('loopback','loop'),('desktop','desktop')]:
        f=here/name/(pat+'-1.jsonl');second=here/name/(pat+'-2.jsonl')
        assert f.read_bytes()==second.read_bytes(),name
        rows=jsonl(f);assert all(o.get('passed',True) for o in rows)
        result[name]={'records':len(rows),'runs':2,'log_sha256':digest(f.read_bytes())}
    diag=jsonl(here/'diagnostics/logentry-1.jsonl')
    assert len(groups)==5 and all(len(g['methods'])==13 for g in groups)
    assert sum(row['id']=='sequence-point' for row in diag)==13
    for g in groups:
        assert {v['id'] for v in diag if v['sha256']==g['sha256'] and v['id']!='sequence-point'}=={'default','explicit-null','explicit-empty','explicit-text','setter-roundtrip'}
        for path in g['paths']:assert digest((args.distribution/path).read_bytes())==g['sha256']
    assert len(variants['native'])==8 and len(variants['modules'])==22
    for v in variants['native']+variants['modules']:
        for path in v.get('paths',[v.get('path')]):assert digest((args.distribution/path).read_bytes())==v['sha256']
    for path,a in kernels.items():assert digest((args.distribution/path).read_bytes())==a['sha256']
    net={v['id']:v for v in jsonl(here/'loopback/loop-1.jsonl')};screen={v['id']:v['value'] for v in jsonl(here/'desktop/desktop-1.jsonl')}
    doc=next(v for v in jsonl(r/'validation/documentation/run-1.jsonl') if v['scenario']=='tree')
    assert net['http-pair']['body']==json.loads(doc['value'])
    assert net['live-stock-window']['eventId']=='fixture-live-window' and net['same-id-in-database']['passed']
    assert net['csv-consumer']['value']=='Example' and net['search-api-to-stored-fixture']['passed']
    assert screen['doc-success']['text']==['Fixture Page'] and screen['doc-empty']['documents']==0 and screen['doc-error']['documents']==-1
    assert screen['doc-retention-error']['visible'] and not screen['doc-retention-empty']['visible']
    assert screen['events-success']['events']==screen['events-error']['events']==1 and screen['events-empty']['events']==0
    assert screen['events-reopened']['visible'] and screen['events-reopened']['events']==1
    # Search supplied tree by filename, augment with the previously recorded header search.
    source=args.distribution.resolve()
    exts={'.pcap','.pcapng','.cap','.har','.saz','.qlog','.etl','.nettrace'}
    by_name=sorted(p.relative_to(source).as_posix() for p in source.rglob('*') if p.suffix.lower() in exts and p.is_file())
    header_hits=load(r/'examination/distribution.json')['capture_candidates']
    assert not by_name and not header_hits
    aux={'assets':[9],'client':[5,6,7],'dependencies':[3,9],'documentation':[5,6,9],'elastic':[9],
         'modules':[3],'notebooks':[9],'resources':[9],'server':[2,4,5,7,9],'settings':[4,9],
         'templates':[2,3,9],'vm':[4,7,9],'webdav':[9]}
    assert set(aux)=={path.split('/')[0] for path in manifest}
    baseline={a['sha256']:{int(m['token'],16) for m in a['methods']} for a in old}
    coverage=load(r/'validation/coverage.json')
    identities=[dict(sha256=a['sha256'],paths=a['paths'],assembly_discovered=True,type_indexed=a['indexed_types'],
        method_identified=a['indexed_methods'],prior_individual_il=len(baseline.get(a['sha256'],set())),
        additional_diagnostic_semantics=next((len(g['methods']) for g in groups if g['sha256']==a['sha256']),0),
        tasks=sorted({t for path in a['paths'] for t in aux[path.split('/')[0]]})) for a in coverage]
    for a in identities:
        for p in a['paths']:
            if p.endswith('Obfuscation.dll'):a['tasks']=[3,9]
    stats={'Assembly Discovered':len(identities),'Type Indexed':sum(a['type_indexed'] for a in identities),
           'Method Identified':sum(a['method_identified'] for a in identities),
           'Method Semantically Analyzed':{'prior_individual_records':56,'new_LogEntry_method_image_pairs':65,
           'distinct_individual_records_lower_bound':121,'automatic_direct_accessors_separate':13514},
           'Cross-Component Relationship Established':{'selected_static_method_cards':sum(len(v['methods']) for v in kernels.values()),
           'confirmed_lab_edges':['Host -> Server Build (IL)','SearchAsync -> fixture HTTP adapter -> SearchDocumentation -> PostgreSQL -> ApiClient DTO',
              'EventLoggingService -> PostgreSQL','IHubContext bridge -> local SignalR -> stock ApiClient subscription',
              'fixture callback -> EventLogViewModel.OnEventNotification -> visible text']},
           'Behavior Dynamically Verified':result}
    matrix={'scope':'32 selected distinct managed images; native, module, and auxiliary records use their own denominators',
       'levels':stats,'images':identities,'roots':aux,'capture_search':{'filename_candidates':by_name,'previous_signature_candidates':header_hits},
       'baseline_il_sha256':digest((r/'analysis/method-evidence.json').read_bytes()),
       'resource_gaps':['3 encrypted members','8 missing-basename and 19 ambiguous standalone Server file declarations','historical DB dump unavailable'],
       'references':'Individual IL records, inferred use, classification and dynamic cases have different units. No global percentage.'}
    p=here/'matrix.json'
    if args.write:p.write_text(json.dumps(matrix,ensure_ascii=False,indent=2)+'\n')
    else:assert matrix==load(p)
    print('Verified',len(identities),'managed image rows;',result)

if __name__=='__main__':main()
