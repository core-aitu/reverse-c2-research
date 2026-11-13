"""Static examination of diagnostic DTOs and selected administrative call sites."""
from pathlib import Path
import sys,json,hashlib
import dnfile,pefile
from dncil.cil.body.reader import read_method_body_from_bytes

root,out=map(Path,sys.argv[1:])
r=out.parents[1]
def read(f):return json.loads((r/f).read_text())
def save(f,d):(out/f).write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def generic_args(t,blob):
    data=iter(blob)
    def integer():
        n=next(data)
        if n<128:return n
        if n<192:return ((n&63)<<8)|next(data)
        return ((n&31)<<24)|(next(data)<<16)|(next(data)<<8)|next(data)
    def item():
        kind=integer()
        if kind in (0x11,0x12):
            token=integer()
            table=(t.TypeDef,t.TypeRef,t.TypeSpec)[token&3]
            row=table.rows[(token>>2)-1]
            return name(row) if hasattr(row,'TypeName') else '<type-spec>'
        if kind==0x15:
            base=item();return base+'<'+','.join(item() for _ in range(integer()))+'>'
        if kind in (0x13,0x1e):return '!'+str(integer())
        if kind==0x1d:return item()+'[]'
        return hex(kind)
    try:
        if integer()!=10:return []
        return [item() for _ in range(integer())]
    except (IndexError,StopIteration):return []
def name(t):return '.'.join(filter(None,[str(t.TypeNamespace),str(t.TypeName)]))

class Image:
    def __init__(self,path):
        self.p=dnfile.dnPE(str(path),clr_lazy_load=True)
        self.t=self.p.net.mdtables
        self.types=list(self.t.TypeDef)
        self.owners={m.row_index:t for t in self.types for m in t.MethodList}
        self.nested={id(x.NestedClass.row):x.EnclosingClass.row for x in self.t.NestedClass or []}
    def owner(self,t):
        parent=self.nested.get(id(t))
        return self.owner(parent)+'+'+str(t.TypeName) if parent else name(t)
    def card(self,num,diagnostic=False):
        m=self.t.MethodDef.rows[num-1]
        b=read_method_body_from_bytes(self.p.get_data(m.Rva,65536))
        code=b.raw_bytes[b.header_size:b.header_size+b.code_size]
        ops=[]; literals=[]
        for i in b.instructions:
            v=getattr(i.operand,'value',0)
            op=i.opcode.name; target=None
            if op in ('call','callvirt','newobj','ldftn'):
                row=self.t.tables[v>>24].rows[(v&0xffffff)-1]
                args=[]
                if v>>24==43:
                    args=generic_args(self.t,row.Instantiation.value)
                    ref=row.Method;row=ref.row;v=(ref.table.number<<24)|ref.row_index
                if v>>24==6:owner=self.owner(self.owners[v&0xffffff])
                else:
                    parent=row.Class.row
                    owner=name(parent) if hasattr(parent,'TypeName') else '<generic>'
                target=owner+'::'+str(row.Name)
                if any(('Documentation' in a or 'EventLogging' in a or 'Startup' in a) for a in args):target+='<' + ','.join(args)+'>'
            elif op=='ldstr':
                raw=self.p.net.user_strings.get(v&0xffffff).value
                if raw.lower() in ('documentation','documentation/search','documentation/index','documentation/index-status','documentation/embedding-providers','eventnotification','documentation/'):
                    literals.append(raw)
            elif diagnostic and op in ('stfld','ldfld','ldsfld','stsfld'):
                target=str(self.t.tables[v>>24].rows[(v&0xffffff)-1].Name)
            if diagnostic or target:
                ops.append({'il':i.offset-b.header_size,'op':op,'target':target})
        if not diagnostic:
            keys=('Startup','Documentation','EventLog','EventNotification','Refresh','Configure','Build','GetEvents','GetAllEvents','GetLatestEvents','GetDocumentation','InvokeAsync','GetService','get_Client','::Add','::Clear','ReadObjectResponse','CreateRequest','LoadVendors','LookupVendor','SubstringSearch','GetTreeAsync','::SendAsync','SearchAsync','SemanticSearch','get_DocumentationFolder','::get_Current','TryLookup','AddSingleton','AddScoped','EventLogging','TryGetReleaseFolder','get_Configuration','::Bind','OnEventNotification','get_UIThread','::SendCoreAsync','::set_Events','::NotifyAll','GetAwaiter','get_IsCompleted','::LogError','::SetException')
            ops=[o for o in ops if any(k in o['target'] for k in keys) and not any(u in o['target'].lower() for u in ('payload','implant','listener','tunnel','pipeline','script:'))]
        return dict(type=self.owner(self.owners[num]),name=str(m.Name),token=hex(0x6000000|num),sig=m.Signature.value.hex(),parameters=[str(x.row.Name) for x in m.ParamList],rva=m.Rva,il_sha256=hashlib.sha256(code).hexdigest(),exception_regions=len(b.exception_handlers),operations=ops,selected_literals=literals)
variants=read('evidence/variant-comparison.json')
symbols=read('evidence/debug-bindings.json')['bindings']
assemblies=[json.loads(l) for l in (r/'analysis/assemblies.jsonl').read_text().splitlines()]
diagnostics=[]
for v in variants['PE/CLI']['variants']:
    p=root/v['paths'][0];assert sha(p)==v['sha256']
    im=Image(p);t=next(t for t in im.types if name(t)=='SpecterInsight.Logging.LogEntry')
    diagnostics.append(dict(**v,mvid=str(im.t.Module.rows[0].Mvid),portable_pdb=next((b['matches'] for b in symbols if b['hash']==v['sha256']),[]),methods=[im.card(m.row_index,True) for m in t.MethodList]))
save('diagnostic-variants.json',diagnostics)
native=[]
for v in variants['PE']['variants']:
    path=root/v['paths'][0];assert sha(path)==v['sha256']
    p=pefile.PE(str(path),fast_load=True)
    p.parse_data_directories(directories=[pefile.DIRECTORY_ENTRY['IMAGE_DIRECTORY_ENTRY_IMPORT'],pefile.DIRECTORY_ENTRY['IMAGE_DIRECTORY_ENTRY_DEBUG']])
    native.append(dict(**v,import_libraries=sorted(set(x.dll.decode('ascii','replace') for x in getattr(p,'DIRECTORY_ENTRY_IMPORT',[]))),debug_directory_types=[x.struct.Type for x in getattr(p,'DIRECTORY_ENTRY_DEBUG',[])],use='Unverified'))
mods=[]
for a in assemblies:
    if not a['path'].startswith('modules/'):continue
    refs=[dict(path=b['path'],sha256=b['hash']) for b in assemblies if b['own_candidate'] and b['hash']!=a['hash'] and any(v['name']==a['name'] and v['version']==a['version'] for v in b['refs'])]
    mods.append(dict(path=a['path'],sha256=a['hash'],name=a['name'],version=a['version'],mvid=a['mvid'],direct_assemblyref_candidates=refs,runtime_use='Unverified'))
save('variants-and-modules.json',{'native':native,'modules':mods})
routes={}
selected={
 'server/SpecterInsight.Server.Host.dll':{4},
 'server/SpecterInsight.Server.dll':{0x3b,0x3c,0x3d,0x3e,0x3f,0x1bf1,0x1bf3,0x1bf5,0x199a,0x199b,0x199e,0x199f,0x19a0,0x19a1,0x19a2,0x2725,0x2727,0x272b,0x21d1,0xc55,0xfaf,0xfb0},
 'client/SpecterInsight.UI.dll':{0x157,0x158,0x159,0x917,0x918,0x919,0x938,0x939,0x93e,0x93f,0x940,0x122a},
 'client/SpecterInsight.ApiClient.dll':{0x23,0xa3,0xa4,0xa5,0xa6,0xa7,0xa8,0xa9,0xaa,0xab,0xac,0x1e1}}
for path,nums in selected.items():
    im=Image(root/path)
    for t in im.types:
        own=im.owner(t)
        if '+' not in own:continue
        if any(s in own for s in ('DocumentationController+','EventLogViewModel+','Startup+','KestrelManagementServer+','DocumentationService+<SemanticSearchAsync','SpecterInsightClientGenerated+<ReadObjectResponseAsync','SpecterInsightClientGenerated+<DocumentationAsync','SpecterInsightClientGenerated+<EmbeddingProvidersAsync','SpecterInsightClientGenerated+<IndexStatusAsync','SpecterInsightClientGenerated+<IndexAsync','SpecterInsightClientGenerated+<SearchAsync')):
            for m in t.MethodList:
                if str(m.row.Name)=='MoveNext' or (str(t.TypeName)=='<>c' and ('Build' in str(m.row.Name) or 'Refresh' in str(m.row.Name))):nums.add(m.row_index)
    routes[path]={'sha256':sha(root/path),'mvid':str(im.t.Module.rows[0].Mvid),'methods':[im.card(i) for i in sorted(nums)]}
save('administrative-call-sites.json',routes)
