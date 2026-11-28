import { type FormEvent, useCallback, useEffect, useState } from "react";
import { useAuth } from "./auth";
import type { Adapter, Job, PipelineDefinition, Resource } from "./types";

export function ResourceScreen({ kind, title }: { kind: string; title: string }) {
  const { request, identity } = useAuth();
  const [resources, setResources] = useState<Resource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const canWrite = identity?.roles.includes("admin") ?? false;
  const load = useCallback(async () => {
    setLoading(true); setError("");
    try { setResources(await request<Resource[]>(`/api/workflows/${kind}`)); }
    catch (cause) { setError(message(cause)); }
    finally { setLoading(false); }
  }, [kind, request]);
  useEffect(() => { void load(); }, [load]);

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setError("");
    try {
      await request(`/api/workflows/${kind}`, { method: "PUT", body: JSON.stringify({
        key: String(data.get("key")), name: String(data.get("name")),
        description: String(data.get("description")), status: "ready",
        spec: { provenance: "clean-room", fixture: true },
      }) });
      event.currentTarget.reset(); setShowForm(false); await load();
    } catch (cause) { setError(message(cause)); }
  }

  async function remove(key: string) {
    try { await request(`/api/workflows/${kind}/${encodeURIComponent(key)}`, { method: "DELETE" }); await load(); }
    catch (cause) { setError(message(cause)); }
  }

  return <div className="stack">
    <section className="panel">
      <div className="panel-title"><div><h2>{title}</h2><p>Versioned resources with provenance, audit events and permission checks.</p></div>
        {canWrite ? <button className="primary" onClick={() => setShowForm((value) => !value)}>{showForm ? "Close" : "New resource"}</button> : <span className="badge">read only</span>}
      </div>
      {showForm && <form className="inline-form" onSubmit={save}>
        <label>Stable key<input name="key" required pattern="[a-zA-Z0-9._-]+" /></label>
        <label>Name<input name="name" required /></label>
        <label className="wide">Description<input name="description" /></label>
        <button className="primary">Save</button>
      </form>}
      {error && <State kind="error" text={error} />}
      {loading ? <State kind="loading" text="Loading resources…" /> : !resources.length ? <State kind="empty" text="No resources yet. The API and empty state are operational." /> :
        <div className="resource-grid">{resources.map((value) => <article className="resource-card" key={value.key}>
          <div><span className={`badge ${value.status === "ready" ? "success" : ""}`}>{value.status}</span><small>{value.key}</small></div>
          <h3>{value.name}</h3><p>{value.description || "No description"}</p>
          <footer><time>{new Date(value.updatedAt).toLocaleString()}</time>{canWrite && <button className="danger-link" onClick={() => remove(value.key)}>Delete</button>}</footer>
        </article>)}</div>}
    </section>
  </div>;
}

export function AdaptersScreen() {
  const { request } = useAuth();
  const [values, setValues] = useState<Adapter[]>([]);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const load = useCallback(() => request<Adapter[]>("/api/adapters").then(setValues).catch((cause) => setError(message(cause))), [request]);
  useEffect(() => { void load(); }, [load]);
  async function probe(name: string) {
    setBusy(name); setError("");
    try { await request(`/api/adapters/${name}/probe`, { method: "POST" }); await load(); }
    catch (cause) { setError(message(cause)); }
    finally { setBusy(""); }
  }
  return <section className="panel"><div className="panel-title"><div><h2>External adapter ports</h2><p>Fixture-first probes. Real network probes require explicit server configuration.</p></div><span className="badge success">{values.filter((value) => value.status === "healthy").length} healthy</span></div>
    {error && <State kind="error" text={error} />}
    <div className="adapter-grid">{values.map((value) => <article className="adapter-card" key={value.name}>
      <div><h3>{value.name}</h3><span className={`badge ${value.status === "healthy" ? "success" : value.status === "unhealthy" ? "failure" : ""}`}>{value.status}</span></div>
      <p>{value.category} · {value.mode}</p><code>{value.endpoint}</code>
      {value.lastError && <small>{value.lastError}</small>}
      <button className="secondary" disabled={busy === value.name} onClick={() => probe(value.name)}>{busy === value.name ? "Probing…" : "Probe"}</button>
    </article>)}</div>
  </section>;
}

export function PipelinesScreen() {
  const { request } = useAuth();
  const [pipelines, setPipelines] = useState<PipelineDefinition[]>([]);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [input, setInput] = useState("fixture-live-window");
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    const [definitions, jobList] = await Promise.all([request<PipelineDefinition[]>("/api/pipelines"), request<Job[]>("/api/jobs")]);
    setPipelines(definitions); setJobs(jobList.filter((job) => job.kind === "pipeline"));
  }, [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  async function run(id: string) {
    setError("");
    try {
      await request(`/api/pipelines/${id}/run`, { method: "POST", body: JSON.stringify({ input }) });
      await new Promise((resolve) => window.setTimeout(resolve, 250)); await load();
    } catch (cause) { setError(message(cause)); }
  }
  return <div className="stack"><section className="panel"><div className="panel-title"><div><h2>Typed safe DAGs</h2><p>Bounded transformations only; executable and network nodes are rejected.</p></div><span className="badge success">worker ready</span></div>
    <label>Fixture input<input value={input} onChange={(event) => setInput(event.target.value)} maxLength={1048576} /></label>
    {error && <State kind="error" text={error} />}
    <div className="resource-grid">{pipelines.map((value) => <article className="resource-card" key={value.id}><div><span className="badge success">v{value.definition.version}</span><small>{value.id}</small></div><h3>{value.name}</h3><p>{value.description}</p><code>{value.definition.nodes.map((node) => node.type).join(" · ")}</code><footer><span>{value.definition.nodes.length} nodes</span><button className="primary" onClick={() => run(value.id)}>Run fixture</button></footer></article>)}</div>
  </section><JobsTable jobs={jobs} refresh={load} /></div>;
}

export function JobsScreen() {
  const { request } = useAuth();
  const [jobs, setJobs] = useState<Job[]>([]);
  const load = useCallback(() => request<Job[]>("/api/jobs").then(setJobs), [request]);
  useEffect(() => { void load(); const timer = window.setInterval(() => void load(), 3000); return () => window.clearInterval(timer); }, [load]);
  return <JobsTable jobs={jobs} refresh={load} />;
}

function JobsTable({ jobs, refresh }: { jobs: Job[]; refresh: () => Promise<unknown> }) {
  const { request } = useAuth();
  async function cancel(id: string) { await request(`/api/jobs/${id}/cancel`, { method: "POST" }); await refresh(); }
  return <section className="panel"><div className="panel-title"><div><h2>Persistent jobs</h2><p>Lifecycle, cancellation, bounded attempts and retained output.</p></div><span className="badge">{jobs.length} jobs</span></div>
    <div className="table-wrap"><table><thead><tr><th>Job</th><th>Kind</th><th>Status</th><th>Attempt</th><th>Output</th><th /></tr></thead><tbody>{jobs.map((job) => <tr key={job.id}><td><code>{job.id}</code></td><td>{job.kind}</td><td><span className={`badge ${job.status === "succeeded" ? "success" : job.status === "failed" ? "failure" : ""}`}>{job.status}</span></td><td>{job.attempt}/{job.maxAttempts}</td><td><code>{job.error || JSON.stringify(job.output ?? {}).slice(0, 90)}</code></td><td>{["queued", "running"].includes(job.status) && <button className="danger-link" onClick={() => cancel(job.id)}>Cancel</button>}</td></tr>)}{!jobs.length && <tr><td className="empty" colSpan={6}>No jobs have run.</td></tr>}</tbody></table></div>
  </section>;
}

function State({ kind, text }: { kind: "loading" | "empty" | "error"; text: string }) {
  return <div className={`state ${kind}`} role={kind === "error" ? "alert" : undefined}>{text}</div>;
}

function message(cause: unknown) { return cause instanceof Error ? cause.message : "Request failed"; }
