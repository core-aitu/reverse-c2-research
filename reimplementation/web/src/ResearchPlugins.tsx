import { useState } from 'react';
import { recordings } from './measurementReplay';
import { pluginCatalog, type PluginId } from './researchPlugins/catalog';
import { buildReports, type ReportBundle } from './researchPlugins/reports';

const format = (value: number | null) => value === null ? '—' : new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value);

export function ResearchPlugins() {
  const [recording, setRecording] = useState('baseline');
  const [enabled, setEnabled] = useState<PluginId[]>(pluginCatalog.map(p => p.id));
  const [result, setResult] = useState<ReportBundle | null>(null);
  function toggle(id: PluginId, checked: boolean) {
    setEnabled(previous => checked ? [...previous.filter(value => value !== id), id] : previous.filter(value => value !== id));
    setResult(null);
  }
  return <div className="stack">
    <section className="panel">
      <div className="panel-title"><div><h2>Research plugins</h2><p>Built-in, local reports for fictional classroom measurement records. No external plugins or operational connections.</p></div></div>
      <div className="form-grid"><label>Recording<select value={recording} onChange={event => { setRecording(event.target.value); setResult(null); }}>
        {recordings().map(item => <option key={item.id} value={item.id}>{item.title}</option>)}
      </select></label></div>
      <h3>Report catalog</h3>
      <div className="table-wrap"><table><thead><tr><th>Enabled</th><th>Report</th><th>Version</th><th>Description</th></tr></thead><tbody>
        {pluginCatalog.map(plugin => <tr key={plugin.id}><td><input type="checkbox" aria-label={`Enable ${plugin.title}`} checked={enabled.includes(plugin.id)} onChange={event => toggle(plugin.id, event.target.checked)}/></td><td>{plugin.title}</td><td>{plugin.version}</td><td>{plugin.description}</td></tr>)}
      </tbody></table></div>
      <div className="button-row"><button className="primary" disabled={!enabled.length} onClick={() => setResult(buildReports(recording, enabled))}>Run selected reports</button><button className="secondary" onClick={() => { setEnabled(pluginCatalog.map(p => p.id)); setResult(null); }}>Reset selection</button></div>
    </section>
    <section className="panel"><h2>Results</h2><p role="status">{result ? `${result.recordingTitle}: ${result.reports.length} report(s) generated.` : 'Choose reports and run them to see results.'}</p>
      {result?.reports.map(report => <div key={report.id}><h3>{report.title} <small>v{report.version}</small></h3><div className="table-wrap"><table><thead><tr><th>Metric</th><th>Value</th><th>Unit</th></tr></thead><tbody>{report.rows.map(row => <tr key={row.label}><td>{row.label}</td><td>{format(row.value)}</td><td>{row.unit}</td></tr>)}</tbody></table></div></div>)}
      <p>Results are computed in memory. No data is transmitted or persisted.</p>
    </section>
  </div>;
}
