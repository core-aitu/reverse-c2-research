import { useEffect, useRef, useState } from "react";
import { useAuth } from "./auth";

type Definition = { id: string; title: string; description: string; version: number };
type Run = {
  id: string; actor: string; createdAt: string;
  report: {
    scenarioId: string; version: number; passed: boolean;
    records: { sequence: number; value: number }[];
    findings: { code: string; sequence: number }[];
    expected: string[];
  };
};

const message = (cause: unknown) => cause instanceof Error ? cause.message : "Request failed";

export function ResearchScenarios() {
  const { identity } = useAuth();
  if (!identity?.roles.includes("admin")) return <section className="panel"><p>Administrator access is required.</p></section>;
  return <ResearchScenarioPanel />;
}

function ResearchScenarioPanel() {
  const { request } = useAuth();
  const [catalog, setCatalog] = useState<Definition[]>([]);
  const [runs, setRuns] = useState<Run[]>([]);
  const [preset, setPreset] = useState("");
  const [selected, setSelected] = useState("");
  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  const mounted = useRef(false);
  const mutation = useRef<AbortController | null>(null);

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; mutation.current?.abort(); };
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    const init = { signal: controller.signal };
    setLoading(true);
    setError("");
    void (async () => {
      try {
        // Sequential calls avoid concurrent refresh-token rotation.
        const definitions = await request<Definition[]>("/api/research-scenarios", init);
        const history = await request<Run[]>("/api/research-scenarios/runs", init);
        if (controller.signal.aborted) return;
        setCatalog(definitions);
        setRuns(history);
        setPreset(previous => definitions.some(item => item.id === previous) ? previous : definitions[0]?.id ?? "");
        setSelected(previous => history.some(item => item.id === previous) ? previous : history[0]?.id ?? "");
      } catch (cause) {
        if (!controller.signal.aborted) setError(message(cause));
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    })();
    return () => controller.abort();
  }, [request, revision]);

  async function execute() {
    if (!preset || loading || mutation.current) return;
    const controller = new AbortController();
    mutation.current = controller;
    setRunning(true);
    setError("");
    try {
      const result = await request<Run>("/api/research-scenarios/runs", {
        method: "POST", signal: controller.signal, body: JSON.stringify({ scenarioId: preset }),
      });
      if (mounted.current && !controller.signal.aborted) {
        setRuns(previous => [result, ...previous.filter(item => item.id !== result.id)].slice(0, 50));
        setSelected(result.id);
        // Reload authoritative ordering and include runs from other administrators.
        setRevision(previous => previous + 1);
      }
    } catch (cause) {
      if (mounted.current && !controller.signal.aborted) setError(message(cause));
    } finally {
      if (mutation.current === controller) mutation.current = null;
      if (mounted.current && !controller.signal.aborted) setRunning(false);
    }
  }

  const definition = catalog.find(item => item.id === preset);
  const run = runs.find(item => item.id === selected);
  return <div className="stack">
    <section className="panel">
      <div className="panel-title"><div><h2>Research scenario engine</h2><p>Built-in fictional datasets only. No network access or remote execution.</p></div>
        <button className="secondary" disabled={loading || running} onClick={() => setRevision(previous => previous + 1)}>Refresh</button></div>
      {error && <p className="error" role="alert">{error}</p>}
      {loading && <p role="status">Loading scenario catalog and history…</p>}
      <div className="form-grid"><label>Scenario<select value={preset} disabled={loading || running} onChange={event => setPreset(event.target.value)}>
        {!catalog.length && <option value="">No scenarios loaded</option>}
        {catalog.map(item => <option value={item.id} key={item.id}>{item.title} (v{item.version})</option>)}
      </select></label></div>
      {definition && <p>{definition.description}</p>}
      <button className="primary" disabled={loading || running || !definition} onClick={() => void execute()}>{running ? "Running…" : "Run scenario"}</button>
    </section>
    <section className="panel"><div className="panel-title"><div><h2>Run history</h2><p>Latest 50 persisted runs across administrators.</p></div></div>
      <div className="table-wrap"><table><thead><tr><th>Time</th><th>Scenario</th><th>Actor</th><th>Check</th><th>Findings</th><th>Report</th></tr></thead>
      <tbody>{runs.map(item => <tr key={item.id}>
        <td>{new Date(item.createdAt).toLocaleString()}</td><td>{item.report.scenarioId} (v{item.report.version})</td><td>{item.actor}</td>
        <td><span className={`badge ${item.report.passed ? "success" : "failure"}`}>{item.report.passed ? "PASS" : "FAIL"}</span></td>
        <td>{item.report.findings.length}</td><td><button className="secondary" aria-pressed={selected === item.id} onClick={() => setSelected(item.id)}>View</button></td>
      </tr>)}{!runs.length && <tr><td colSpan={6} className="empty">{loading ? "Loading…" : "No runs yet."}</td></tr>}</tbody></table></div>
    </section>
    {run && <section className="panel"><div className="panel-title"><div><h2>Run report</h2><p>{run.report.scenarioId} · {run.id}</p></div><span className={`badge ${run.report.passed ? "success" : "failure"}`}>{run.report.passed ? "PASS" : "FAIL"}</span></div>
      <p>PASS means the observed findings match the fixture's expected findings, not that the input was clean.</p>
      <h3>Source observations</h3><div className="table-wrap"><table><thead><tr><th>Sequence</th><th>Measurement</th></tr></thead><tbody>
        {run.report.records.map((record, index) => <tr key={index}><td>{record.sequence}</td><td>{record.value}</td></tr>)}
      </tbody></table></div>
      <h3>Expected findings</h3><p>{run.report.expected.join(", ") || "None"}</p>
      <h3>Observed findings</h3><div className="table-wrap"><table><thead><tr><th>Code</th><th>Sequence</th></tr></thead><tbody>
        {run.report.findings.map((finding, index) => <tr key={index}><td>{finding.code}</td><td>{finding.sequence}</td></tr>)}
        {!run.report.findings.length && <tr><td colSpan={2} className="empty">No findings.</td></tr>}
      </tbody></table></div>
    </section>}
  </div>;
}
