import { useEffect, useState } from "react";
import { recordings, replay } from "./measurementReplay";

const catalog = recordings();
const format = (value: number | null) => value === null ? "—" : `${value.toFixed(1)} °C`;

export function MeasurementReplay() {
  const [id, setId] = useState(catalog[0].id);
  const [position, setPosition] = useState(0);
  const [playing, setPlaying] = useState(false);
  const recording = catalog.find(item => item.id === id) ?? catalog[0];
  const count = recording.events.length;
  const frame = replay(recording, position);
  useEffect(() => {
    if (!playing) return;
    if (position >= count) { setPlaying(false); return; }
    const timer = window.setTimeout(() => setPosition(previous => Math.min(previous + 1, count)), 700);
    return () => window.clearTimeout(timer);
  }, [playing, position, count, id]);

  function seek(next: number) { setPlaying(false); setPosition(next); }
  const last = frame.events.at(-1);
  return <div className="stack">
    <section className="panel">
      <div className="panel-title"><div><h2>Measurement replay</h2><p>Offline fictional classroom recordings. No live data or device connections.</p></div></div>
      <div className="form-grid"><label>Recording<select value={id} onChange={event => { setPlaying(false); setPosition(0); setId(event.target.value); }}>
        {catalog.map(item => <option key={item.id} value={item.id}>{item.title}</option>)}
      </select></label></div>
      <p>{recording.description}</p>
      <label>Timeline: {position} / {count} records<input type="range" min={0} max={count} step={1} value={position} onChange={event => seek(Number(event.target.value))}/></label>
      <p role="status">{last ? `Recorded minute ${last.minute} · ${last.station}` : "Before the first observation"}</p>
      <div className="button-row">
        <button className="secondary" disabled={position === 0} onClick={() => seek(0)}>Reset</button>
        <button className="secondary" disabled={position === 0} onClick={() => seek(position - 1)}>Previous</button>
        <button className="primary" disabled={!playing && position === count} onClick={() => setPlaying(value => !value)}>{playing ? "Pause" : "Play"}</button>
        <button className="secondary" disabled={position === count} onClick={() => seek(position + 1)}>Next</button>
      </div>
    </section>
    <section className="panel"><div className="panel-title"><div><h2>Frame statistics</h2><p>Only the displayed prefix contributes to the results.</p></div></div>
      <div className="metric-grid">{[["Accepted", frame.valid], ["Missing", frame.missing], ["Rejected", frame.rejected], ["Duplicates", frame.duplicates]].map(([label, value]) => <div className="replay-stat" key={label}><strong>{value}</strong><span>{label}</span></div>)}</div>
      <div className="table-wrap"><table><thead><tr><th>Station</th><th>Accepted</th><th>Missing</th><th>Rejected</th><th>Mean</th><th>Last accepted</th></tr></thead><tbody>
        {frame.stations.map(item => <tr key={item.station}><td>{item.station}</td><td>{item.valid}</td><td>{item.missing}</td><td>{item.rejected}</td><td>{format(item.average)}</td><td>{format(item.last)}</td></tr>)}
        {!frame.stations.length && <tr><td colSpan={6} className="empty">No observations at this position.</td></tr>}
      </tbody></table></div>
    </section>
    <section className="panel"><div className="panel-title"><div><h2>Recorded observations</h2><p>Original arrival order, including duplicates and rejected readings.</p></div></div>
      <div className="table-wrap"><table><thead><tr><th>Position</th><th>Minute</th><th>Record</th><th>Station</th><th>Temperature</th></tr></thead><tbody>
        {frame.events.map((event, index) => <tr key={index}><td>{index + 1}</td><td>{event.minute}</td><td>{event.id}</td><td>{event.station}</td><td>{format(event.temperature)}</td></tr>)}
        {!frame.events.length && <tr><td colSpan={5} className="empty">Start playback to view the recording.</td></tr>}
      </tbody></table></div>
    </section>
  </div>;
}
