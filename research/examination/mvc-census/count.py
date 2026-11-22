"""Count explicit MVC metadata declarations in a supplied server image."""
from collections import Counter, defaultdict
from pathlib import Path
import hashlib
import json
import sys
import dnfile

MVC = 'Microsoft.AspNetCore.Mvc'
VERBS = {'Http' + verb + 'Attribute' for verb in ('Get', 'Head', 'Post', 'Put', 'Patch', 'Delete', 'Options')}


def census(path):
    image = dnfile.dnPE(str(path), clr_lazy_load=True)
    tables = image.net.mdtables
    types = list(tables.TypeDef)
    owners = {m.row_index: i for i, t in enumerate(types, 1) for m in t.MethodList}
    attrs = defaultdict(list)
    for attr in tables.CustomAttribute:
        if attr.Parent.table.name not in {'TypeDef', 'MethodDef'}:
            continue
        ctor = attr.Type
        owner = ctor.row.Class.row if ctor.table.name == 'MemberRef' else types[owners[ctor.row_index]-1]
        if str(getattr(owner, 'TypeNamespace', '')) == MVC:
            attrs[attr.Parent.table.name, attr.Parent.row_index].append(str(owner.TypeName))

    def inherits_mvc(t):
        seen = set()
        base = t.Extends
        while base is not None and base.row_index:
            if base.table.name == 'TypeRef':
                ref = base.row
                return str(ref.TypeNamespace) == MVC and str(ref.TypeName) in {'ControllerBase','Controller'}
            if base.table.name != 'TypeDef' or base.row_index in seen:
                break
            seen.add(base.row_index)
            base = base.row.Extends
        return False

    reasons = Counter()
    verb_counts = Counter()
    method_total = routes = 0
    for rid, t in enumerate(types, 1):
        base = inherits_mvc(t)
        declared = 'ApiControllerAttribute' in attrs['TypeDef',rid]
        if not (base or declared):
            continue
        reasons['both' if base and declared else 'mvc_base_only' if base else 'api_attribute_only'] += 1
        for m in t.MethodList:
            flags = m.row.Flags
            annotations = attrs['MethodDef',m.row_index]
            if not flags.mdPublic or flags.mdStatic or flags.mdSpecialName or 'NonActionAttribute' in annotations:
                continue
            verbs = VERBS.intersection(annotations)
            if verbs:
                method_total += 1
                verb_counts.update(sorted(verbs))
            routes += 'RouteAttribute' in annotations
    return {'sha256':hashlib.sha256(path.read_bytes()).hexdigest(), 'mvid':str(tables.Module.rows[0].Mvid),
            'typed_candidates':dict(sorted(reasons.items())), 'public_instance_methods_with_direct_verb':method_total,
            'methods_with_separate_route_attribute':routes,'verb_method_counts':dict(sorted(verb_counts.items())),
            'scope':'Direct attributes and locally resolvable ancestry. Metadata counts only. Does not resolve conventions, dynamic endpoints, named routes, or invocation frequency.'}


if __name__ == '__main__':
    print(json.dumps(census(Path(sys.argv[1])), indent=2))
