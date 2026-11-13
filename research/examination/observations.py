"""Extract bounded, read-only static observations for the documentation and deployment study."""
import argparse
import collections
import gzip
import hashlib
import json
import zipfile
from pathlib import Path

import dnfile
import pefile
from dncil.cil.body.reader import read_method_body_from_bytes


def save(path, data):
    path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + '\n')


def read_json(path):
    return json.loads(path.read_text(encoding='utf-8-sig'))


# Tokens are bound to this snapshot. System-wide authentication evidence is limited
# to framework lifecycle markers. Documentation IL records contain no method bodies.
METHODS = {
    'server/SpecterInsight.Server.Host.dll': [1, 3, 4],
    'server/SpecterInsight.Server.dll': [0x3b, 0x3c, 0x199b, 0x199d, 0x199e, 0x2725, 0x2727, 0x2729, 0x2294, 0xfad, 0xfb1, 0xfb2, 0xfb4, 0xc7e, 0xcd2, 0xcd4, 0xcd5, 0xcd6, 0xb56, 0x21d1, 0x229a, 0x230e, 0x236e, 0x2390, 0x23e2, 0x2446, 0xc59, 0xc5a, 0xc5b],
    'client/SpecterInsight.UI.dll': [0x4b, 0x4c, 0x919, 0x122a, 0x120d, 0x1207, 0x17f3, 0x17f4, 0x17fa, 0x17fb, 0x17fe, 0x17ff, 0x1801, 0x1802, 0x1804, 0x1805, 0x1807, 0x1808, 0x936, 0x93a, 0x93b],
    'client/SpecterInsight.ApiClient.dll': [0x860, 0x385, 0x386],
}
MARKERS = {'ConfigureWebHostDefaults','UseStartup','Build','RunAsync','StartAsync',
           'AddAuthentication','AddAuthorization','AddJwtBearer','AddControllers',
           'AddSignalR','AddSingleton','AddScoped','AddTransient','AddHostedService',
           'UseRouting','UseAuthentication','UseAuthorization','UseEndpoints','MapControllers',
           'StartWithClassicDesktopLifetime','BuildAvaloniaApp','Configure','UsePlatformDetect','UseReactiveUI',
           'Run','Main','GetAwaiter','GetResult','SetException','SetResult','get_Task','Start','Create'}
DOC_STRINGS = {'*.md','Overview.md','GET','Documentation','application/json','DocumentationChunks'}


def full_type(row):
    ns = str(row.TypeNamespace)
    return (ns + '.' if ns else '') + str(row.TypeName)


def semantic_methods(image, path):
    tables = image.net.mdtables
    declaring = {m.row_index: typ for typ in tables.TypeDef for m in typ.MethodList}
    nested = {x.NestedClass.row_index: x.EnclosingClass.row for x in (tables.NestedClass or [])}
    typedef_rows = {id(r):i for i,r in enumerate(tables.TypeDef,1)}
    def qual(row):
        parent = nested.get(typedef_rows.get(id(row)))
        return qual(parent)+'+'+str(row.TypeName) if parent else full_type(row)

    result = []
    for index in METHODS[path]:
        method = tables.MethodDef.rows[index-1]
        body = read_method_body_from_bytes(image.get_data(method.Rva, 2**16))
        calls, strings, fields = [], [], []
        filtered = index in {1,3,4,0x3b,0x3c,0x4b,0x4c}
        for ins in body.instructions:
            v = getattr(ins.operand, 'value', 0)
            op = ins.opcode.name
            loc = ins.offset-body.header_size
            if op in {'call','callvirt','newobj','ldftn'}:
                token = v
                row = tables.tables[v>>24].rows[(v&0xffffff)-1]
                if v>>24 == 43:
                    ref = row.Method
                    row = ref.row
                    v=(ref.table.number<<24)|ref.row_index
                dest = str(row.Name)
                if filtered and dest not in MARKERS:
                    continue
                if v>>24 == 6:
                    owner=qual(declaring[v&0xffffff])
                else:
                    parent=row.Class.row
                    owner=full_type(parent) if hasattr(parent,'TypeName') else '<TypeSpec/other>'
                calls.append({'il':loc,'op':op,'token':hex(token),'callee':owner+'::'+dest})
            elif op=='ldstr':
                value=image.net.user_strings.get(v&0xffffff).value
                if (value in DOC_STRINGS or (index in {0x2294,0x229a} and value.startswith('CREATE TABLE'))
                        or (path.startswith('client/') and index in {0x17fe,0x1801,0x1804,0x1807})):
                    strings.append({'il':loc,'value':value})
            elif op in {'stfld','ldfld'} and index in {0x385,0x386}:
                fields.append({'il':loc,'op':op,'field':str(tables.Field.rows[(v&0xffffff)-1].Name)})
        result.append({'declaring_type':qual(declaring[index]), 'name':str(method.Name),
                       'token':f'0x{0x06000000|index:08x}','signature_ecma335':method.Signature.value.hex(),
                       'parameters':[str(item.row.Name) for item in method.ParamList],
                       'rva':method.Rva,'body_size':body.size, 'il_code_size':body.code_size,
                       'il_sha256':hashlib.sha256(body.get_bytes()[body.header_size:body.header_size+body.code_size]).hexdigest(),
                       'exception_regions':len(body.exception_handlers),'calls':calls,
                       'call_selection':'framework-markers' if filtered else 'all-direct-operands',
                       'strings_selected':strings,'fields_selected':fields})
    return result


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('copy',type=Path)
    parser.add_argument('research',type=Path)
    args=parser.parse_args()
    rows=list(map(json.loads,gzip.open(args.research/'evidence/manifest.jsonl.gz','rt')))
    by_path={r['path']:r for r in rows}
    assemblies=list(map(json.loads,(args.research/'analysis/assemblies.jsonl').read_text().splitlines()))
    methods,versions=[],[]
    for r in assemblies:
        if not r.get('own_candidate'):
            continue
        path=r['path']; file=args.copy/path
        if path in METHODS:
            image=dnfile.dnPE(str(file),clr_lazy_load=True)
            methods.append({'id':r['id'],'path':path,'sha256':r['hash'],'mvid':r['mvid'],'methods':semantic_methods(image,path)})
            image.close()
        pe=pefile.PE(str(file),fast_load=True)
        pe.parse_data_directories(directories=[pefile.DIRECTORY_ENTRY['IMAGE_DIRECTORY_ENTRY_RESOURCE']])
        for group in getattr(pe,'FileInfo',[]):
            for attr in group:
                for st in getattr(attr,'StringTable',[]):
                    values={k.decode():v.decode(errors='replace') for k,v in st.entries.items()
                            if k in {b'CompanyName',b'ProductName',b'FileVersion',b'ProductVersion'}}
                    versions.append({'id':r['id'],'path':path,**values})
        pe.close()
    save(args.research/'analysis/method-evidence.json',methods)
    save(args.research/'evidence/pe-versions.json',versions)
    graphs=[]
    for path in sorted(args.copy.glob('*/*.deps.json')):
        rel=path.relative_to(args.copy).as_posix()
        data=read_json(path)
        for target,libs in data['targets'].items():
            graphs.append({'id':by_path[rel]['id'],'manifest':rel,'target':target,
                           'nodes':{name:{'dependencies':val.get('dependencies',{}),
                                          'runtime_assets':list(val.get('runtime',{})),
                                          'native_assets':list(val.get('native',{})),
                                          'rid_assets':list(val.get('runtimeTargets',{}))}
                                    for name,val in libs.items()}})
    save(args.research/'architecture/package-graphs.json',graphs)
    containers=[]
    for row in rows:
        if row.get('format')!='ZIP container':
            continue
        with zipfile.ZipFile(args.copy/row['path']) as archive:
            entries=[]
            for info in archive.infolist():
                if info.flag_bits & 1:
                    entries.append({'name':info.filename,'size':info.file_size,'status':'encrypted','sha256':None})
                    continue
                with archive.open(info) as f:
                    entries.append({'name':info.filename,'size':info.file_size,'sha256':hashlib.file_digest(f,'sha256').hexdigest()})
            containers.append({'id':row['id'],'path':row['path'],'members':entries})
    save(args.research/'evidence/archive-members.json',containers)

if __name__=='__main__':
    main()
