"""Cross-check packaged .NET assets and saved-object references without starting any service."""
from collections import Counter, defaultdict
import argparse
import gzip
import json
from pathlib import Path


def package_record(root, manifest, paths):
    source = json.loads((root / manifest).read_text(encoding='utf-8-sig'))
    base = str(Path(manifest).parent)
    local = {p[len(base)+1:] for p in paths if p.startswith(base+'/')}
    by_name = defaultdict(list)
    for p in local:
        by_name[Path(p).name].append(p)
    records = []
    for target, packages in source['targets'].items():
        versions = defaultdict(list)
        for key in packages:
            name, version = key.rsplit('/', 1)
            versions[name].append(version)
        edges = Counter()
        assets = defaultdict(Counter)
        exceptions = []
        for package, body in packages.items():
            for name, requested in body.get('dependencies', {}).items():
                classification = 'exact' if requested in versions[name] else ('other-version' if versions[name] else 'absent-name')
                edges[classification] += 1
            for group in ('runtime','native','runtimeTargets','resources'):
                for asset in body.get(group,{}):
                    name = Path(asset).name
                    if name == '_._':
                        result = 'placeholder'
                    elif asset in local:
                        result = 'package-relative'
                    elif group=='resources' and str(Path(asset).parent.name+'/'+name) in local:
                        result='locale-and-basename'
                    elif name in local:
                        result = 'top-level-basename'
                    elif len(by_name[name]) == 1:
                        result = 'unique-nested-basename'
                    elif by_name[name]:
                        result = 'ambiguous-basename'
                    else:
                        result = 'no-filename-match'
                    assets[group][result] += 1
                    if result in ('ambiguous-basename','no-filename-match'):
                        exceptions.append({'package':package,'group':group,'name':name,'classification':result})
        records.append({'target':target,'packages':len(packages),'edges':edges,'assets':assets,'exceptions':exceptions})
    return {'manifest':manifest,'targets':records}


def dashboard_record(root):
    path='elastic/elastic/dashboards.ndjson'
    records=[json.loads(line) for line in (root/path).read_text().splitlines() if line.strip()]
    items=[x for x in records if 'id' in x and 'type' in x]
    index={(x['type'],x['id']) for x in items}
    references=Counter()
    panels=Counter()
    for item in items:
        refs=item.get('references',[])
        for ref in refs:
            references['resolved' if (ref['type'],ref['id']) in index else 'missing']+=1
        if item['type']!='dashboard':
            continue
        for panel in json.loads(item['attributes']['panelsJSON']):
            identifiers={panel.get('panelRefName'),str(panel.get('panelIndex'))+':'+str(panel.get('panelRefName'))}
            matching=[ref for ref in refs if ref['name'] in identifiers]
            if len(matching)!=1:
                panels['ambiguous-or-missing']+=1
            elif (matching[0]['type'],matching[0]['id']) not in index:
                panels['missing-target']+=1
            elif matching[0]['type']!=panel['type']:
                panels['type-mismatch']+=1
            else:
                panels['resolved']+=1
    return {'path':path,'objects':len(items),'unique':len(index),'references':references,'panels':panels,
            'types':Counter(x['type'] for x in items),'metadata_records':len(records)-len(items)}


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('copy',type=Path)
    parser.add_argument('research',type=Path)
    args=parser.parse_args()
    with gzip.open(args.research/'evidence/manifest.jsonl.gz','rt') as src:
        rows=(json.loads(line) for line in src)
        paths={x['path'] for x in rows if x['kind']=='file'}
    manifests=sorted(p for p in paths if p.endswith('.deps.json'))
    out={'dependencies':[package_record(args.copy,p,paths) for p in manifests],
         'elastic':dashboard_record(args.copy)}
    (args.research/'architecture/distribution-integrity.json').write_text(json.dumps(out,indent=2)+'\n')
