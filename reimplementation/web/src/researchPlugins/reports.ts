import { recordings, replay } from '../measurementReplay.ts';
import { pluginCatalog, type PluginId } from './catalog.ts';

export type ReportRow = Readonly<{ label: string; value: number | null; unit: string }>;
export type PluginReport = Readonly<{ id: PluginId; title: string; version: string; rows: readonly ReportRow[] }>;
export type ReportBundle = Readonly<{ recordingId: string; recordingTitle: string; reports: readonly PluginReport[] }>;

const count = (label: string, value: number): ReportRow => ({ label, value, unit: 'records' });
const celsius = (label: string, value: number | null): ReportRow => ({ label, value, unit: '°C' });

// Only the two compiled-in classroom fixtures and three compiled-in reports
// can be selected. The dispatcher accepts neither code nor user-defined rules.
export function buildReports(recordingId: string, selected: readonly PluginId[]): ReportBundle {
  const recording = recordings().find(item => item.id === recordingId);
  if (!recording) throw new RangeError('Unknown classroom recording');
  const enabled = new Set(selected);
  if (selected.length > pluginCatalog.length || enabled.size !== selected.length || selected.some(id => !pluginCatalog.some(p => p.id === id))) {
    throw new RangeError('Invalid report selection');
  }
  const frame = replay(recording, recording.events.length);
  const reports: PluginReport[] = [];
  for (const plugin of pluginCatalog) {
    if (!enabled.has(plugin.id)) continue;
    let rows: ReportRow[];
    switch (plugin.id) {
      case 'quality':
        rows = [count('Total observations', frame.events.length), count('Accepted', frame.valid), count('Missing', frame.missing), count('Rejected', frame.rejected), count('Duplicates', frame.duplicates)];
        break;
      case 'coverage':
        rows = frame.stations.flatMap(station => {
          const total = station.valid + station.missing + station.rejected;
          return [count(`${station.station}: accepted`, station.valid), count(`${station.station}: unique observations`, total), { label: `${station.station}: acceptance`, value: total ? 100 * station.valid / total : null, unit: '%' }];
        });
        break;
      case 'temperature':
        rows = frame.stations.flatMap(station => [celsius(`${station.station}: average`, station.average), celsius(`${station.station}: last accepted`, station.last)]);
        break;
    }
    reports.push({ id: plugin.id, title: plugin.title, version: plugin.version, rows });
  }
  return { recordingId: recording.id, recordingTitle: recording.title, reports };
}
