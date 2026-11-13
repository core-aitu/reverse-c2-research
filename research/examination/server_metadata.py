"""Read administrative attribute declarations and aggregate server model counts."""
import collections
import json
from pathlib import Path
import sys
import dnfile

path=Path(sys.argv[1])/'server/SpecterInsight.Server.dll'
p=dnfile.dnPE(str(path),clr_lazy_load=True)
t=p.net.mdtables
owners={m.row_index:(idx,typ) for idx,typ in enumerate(t.TypeDef,1) for m in typ.MethodList}
ctl={i:typ for i,typ in enumerate(t.TypeDef,1) if str(typ.TypeName).endswith('Controller') and str(typ.TypeNamespace).startswith('SpecterInsight.Server')}
attrs=collections.Counter()
documentation=[]
markers=[]
entity_types=collections.Counter()
for idx,ty in enumerate(t.TypeDef,1):
    n,ns=str(ty.TypeName),str(ty.TypeNamespace)
    if ns.startswith('SpecterInsight.Server.Services.Database.Entities'):
        entity_types[ns]+=1
    if n in {'EventLoggingService','SpecterEventHub','DocumentationController','DocumentationService','DatabaseService'}:
        markers.append({'namespace':ns,'type':n,'token':hex(0x02000000|idx)})
for attr in t.CustomAttribute:
    target=attr.Parent
    if target.table.number==2:
        typ=ctl.get(target.row_index)
        method=None
    elif target.table.number==6:
        idx,typ=owners[target.row_index]
        if idx not in ctl:continue
        method=target.row
    else:continue
    if typ is None:continue
    ctor=attr.Type
    if ctor.table.number==6:
        atype=owners[ctor.row_index][1]
    elif ctor.table.number==10:
        atype=ctor.row.Class.row
    else:continue
    name=str(getattr(atype,'TypeName',''))
    attrs[name]+=1
    if str(typ.TypeName)=='DocumentationController' and name in {'RouteAttribute','ApiControllerAttribute','HttpGetAttribute','AuthorizeAttribute','HttpPostAttribute'}:
        blob=attr.Value.value
        # Only attributes of the documentation controller are included.
        text=None
        if len(blob)>4 and blob[:2]==b'\x01\x00' and blob[2]<128 and len(blob)>=5+blob[2]:
            text=blob[3:3+blob[2]].decode('utf-8',errors='replace')
        documentation.append({'scope':str(method.Name) if method else 'type','attribute':name,'argument':text})
data={'candidate_controller_types':len(ctl),'criterion':'SpecterInsight.Server namespace and type suffix Controller',
      'entity_namespace_type_counts':entity_types,'audit_and_storage_type_markers':markers,
      'selected_attribute_counts':{k:v for k,v in attrs.items() if k in {'RouteAttribute','ApiControllerAttribute','HttpGetAttribute','HttpPostAttribute','AuthorizeAttribute'}},
      'documentation_attributes':documentation}
out=Path(sys.argv[2])/'analysis/server-metadata.json'
out.write_text(json.dumps(data,indent=2)+'\n')
