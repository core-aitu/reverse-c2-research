import { useEffect, useState } from "react";
import { useAuth } from "./auth";

type Run = { id: string; createdAt: string; report: { scenarioId: string } };
type Entry = { ordinal: number; kind: "dataset.checked" | "finding.recorded" | "report.completed"; code?: string; sequence?: number; passed?: boolean };
const message = (cause: unknown) => cause instanceof Error ? cause.message : "Request failed";

export function ResearchJournal() {
  const { identity } = useAuth();
  if (!identity?.roles.includes("admin")) return <section className="panel"><p>Administrator access is required.</p></section>;
  return <JournalPanel />;
}

function JournalPanel() {
  const { request } = useAuth();
  const [runs, setRuns] = useState<Run[]>([]);
  const [selected, setSelected] = useState("");
  const [revision, setRevision] = useState(0);
  const [loading, setLoading] = useState(true);
  const [historyError, setHistoryError] = useState("");
  const [result, setResult] = useState<{ runId: string; entries: Entry[]; error: string } | null>(null);
  const [position, setPosition] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setHistoryError("");
    void (async () => {
      try {
        const items = await request<Run[]>("/api/research-scenarios/runs", { signal: controller.signal });
        if (controller.signal.aborted) return;
        setRuns(items);
        setSelected(previous => items.some(item => item.id === previous) ? previous : items[0]?.id ?? "");
      } catch (cause) {
        if (!controller.signal.aborted) setHistoryError(message(cause));
      } finally { if (!controller.signal.aborted) setLoading(false); }
    })();
    return () => controller.abort();
  }, [request, revision]);

  useEffect(() => {
    if (loading || !selected) return;
    const controller = new AbortController();
    setResult(null); setPosition(0);
    void (async () => {
      try {
        const entries = await request<Entry[]>(`/api/research-scenarios/runs/${encodeURIComponent(selected)}/journal`, { signal: controller.signal });
        if (!controller.signal.aborted) setResult({ runId: selected, entries, error: "" });
      } catch (cause) {
        if (!controller.signal.aborted) setResult({ runId: selected, entries: [], error: message(cause) });
      }
    })();
    return () => controller.abort();
  }, [request, selected, loading]);

  const current = !loading && result?.runId === selected ? result : null;
  const entries = current?.entries ?? [];
  const visible = entries.slice(0, position);
  const last = visible.at(-1);
  return <div className="stack">
    <section className="panel"><div className="panel-title"><div><h2>Research journal</h2><p>Persisted records of fictional dataset checks, in logical order. No operational events or live devices.</p></div>
      <button className="secondary" disabled={loading} onClick={() => setRevision(v => v + 1)}>Refresh history</button></div>
      {historyError && <p className="error" role="alert">{historyError}</p>}
      <div className="form-grid"><label>Stored run<select value={selected} disabled={loading || !runs.length} onChange={e => { setResult(null); setPosition(0); setSelected(e.target.value); }}>
        {!runs.length && <option value="">No stored runs</option>}
        {runs.map(run => <option key={run.id} value={run.id}>{run.report.scenarioId} · {new Date(run.createdAt).toLocaleString()} · {run.id}</option>)}
      </select></label></div>
      {!loading && !runs.length && <p>Run a built-in dataset in Scenario Engine first.</p>}
      {loading || (selected && !current) ? <p role="status">Loading journal…</p> : null}
      {current?.error && <p className="error" role="alert">{current.error}</p>}
      {entries.length > 0 && <><label>Recorded position: {position} / {entries.length}<input type="range" min={0} max={entries.length} step={1} value={position} onChange={e => setPosition(Number(e.target.value))}/></label>
      <div className="button-row">
        <button className="secondary" disabled={position === 0} onClick={() => setPosition(0)}>Reset</button>
        <button className="secondary" disabled={position === 0} onClick={() => setPosition(v => Math.max(0, v - 1))}>Previous</button>
        <button className="primary" disabled={position === entries.length} onClick={() => setPosition(v => Math.min(entries.length, v + 1))}>Next</button>
        <button className="secondary" disabled={position === entries.length} onClick={() => setPosition(entries.length)}>Show all</button>
      </div></>}
    </section>
    <section className="panel"><h2>Journal timeline</h2><p role="status">{last ? `Last displayed event: ${last.kind}` : "No events displayed."}</p>
      <div className="table-wrap"><table><thead><tr><th>Order</th><th>Kind</th><th>Finding</th><th>Sequence</th><th>Outcome</th></tr></thead><tbody>
        {visible.map(entry => <tr key={entry.ordinal}><td>{entry.ordinal}</td><td>{entry.kind}</td><td>{entry.code ?? "—"}</td><td>{entry.sequence ?? "—"}</td><td>{entry.passed === undefined ? "—" : entry.passed ? "PASS" : "FAIL"}</td></tr>)}
        {!visible.length && <tr><td colSpan={5} className="empty">Select a stored run and advance the timeline.</td></tr>}
      </tbody></table></div>
      <p>PASS indicates a match to the expected findings, not clean input. Order is logical, not wall-clock timing.</p>
    </section>
  </div>;
}
