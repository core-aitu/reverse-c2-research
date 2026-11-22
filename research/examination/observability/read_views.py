"""Resolve panel data sources from the shipped Kibana saved-object export."""
import argparse
import hashlib
import json
from collections import Counter
from pathlib import Path


def reference(record, name, expected_type, objects):
    matches = [ref for ref in record['references'] if ref['name'] == name]
    if len(matches) != 1 or matches[0]['type'] != expected_type:
        raise ValueError(f"Invalid {expected_type} reference {name} in {record['id']}")
    return objects[expected_type, matches[0]['id']]


def inspect(path):
    payload = path.read_bytes()
    rows = [json.loads(line) for line in payload.splitlines() if line.strip()]
    objects = {(row['type'], row['id']): row for row in rows if 'type' in row}
    if len(objects) != sum('type' in row for row in rows):
        raise ValueError('Duplicate saved-object identity')
    data_views, consumers, dashboards = {}, {}, []
    for (kind, key), row in objects.items():
        attributes = row['attributes']
        if kind == 'index-pattern':
            data_views[key] = {
                field: attributes[field] for field in ('title', 'name', 'timeFieldName')
            }
            for field in ('fields', 'runtimeFieldMap'):
                data_views[key][field + '_count'] = len(json.loads(attributes[field]))
            continue
        sources = []
        if kind in ('visualization', 'search'):
            search = json.loads(attributes['kibanaSavedObjectMeta']['searchSourceJSON'])
            sources.append(('searchSource', search['indexRefName']))
        elif kind == 'map':
            for layer in json.loads(attributes['layerListJSON']):
                source = layer['sourceDescriptor']
                sources.append((source['type'], source.get('indexPatternRefName')))
        else:
            continue
        consumers[kind, key] = []
        for role, name in sources:
            item = {'source': role}
            if name is not None:
                item['data_view'] = reference(row, name, 'index-pattern', objects)['id']
            consumers[kind, key].append(item)
    for (kind, key), row in objects.items():
        if kind != 'dashboard':
            continue
        placements = []
        for panel in json.loads(row['attributes']['panelsJSON']):
            target_type = panel['type']
            name = f"{panel['panelIndex']}:{panel['panelRefName']}"
            target = reference(row, name, target_type, objects)
            placement = {'panel': panel['panelIndex'], 'type': target_type, 'object': target['id']}
            embedded = panel['embeddableConfig'].get('savedVis')
            if embedded:
                search = embedded['data']['searchSource']
                local_name = f"{panel['panelIndex']}:{search['indexRefName']}"
                view = reference(row, local_name, 'index-pattern', objects)
                placement['embedded_data_view'] = view['id']
            placements.append(placement)
        ids = {item['panel'] for item in placements}
        if len(ids) != len(placements):
            raise ValueError(f'Duplicate panel in dashboard {key}')
        counts = Counter()
        for item in placements:
            views = {source['data_view'] for source in consumers[item['type'], item['object']]
                     if 'data_view' in source}
            if not views:
                raise ValueError(f"No data view for panel {item['panel']}")
            counts.update(views)
        dashboards.append({'id': key, 'title': row['attributes']['title'], 'panels': placements,
                           'panel_count_by_primary_view': dict(counts)})
    return {'sha256': hashlib.sha256(payload).hexdigest(), 'data_views': data_views,
            'panel_sources': [{'type': kind, 'id': key, 'sources': sources}
                              for (kind, key), sources in consumers.items()],
            'dashboards': dashboards}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('export', type=Path)
    args = parser.parse_args()
    print(json.dumps(inspect(args.export), indent=2, sort_keys=True, ensure_ascii=False))
