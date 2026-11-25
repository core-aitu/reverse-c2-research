import { type FormEvent, useCallback, useEffect, useState } from "react";
import { useAuth } from "./auth";
import type { ArtifactInfo, LicenseValidation, SerializableVersion, UpdateJobInfo } from "./types";

export function ArtifactsScreen() {
  const { request } = useAuth(); const [values, setValues] = useState<ArtifactInfo[]>([]); const [error, setError] = useState("");
  const load = useCallback(() => request<ArtifactInfo[]>("/api/Artifacts").then(setValues), [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  async function remove(value: ArtifactInfo) { await request(`/api/Artifacts/Artifacts/${encodeURIComponent(value.host)}/${encodeURIComponent(value.name)}`, { method: "DELETE" }); await load(); }
  return <section className="panel"><div className="panel-title"><div><h2>Artifact metadata</h2><p>Legacy ArtifactInfo rows; binary content is unavailable unless independently ingested.</p></div><span className="badge success">metadata only</span></div>{error && <div className="state error">{error}</div>}{!values.length ? <div className="state empty">No artifact metadata has been ingested.</div> : <div className="resource-grid">{values.map((value) => <article className="resource-card" key={`${value.host}/${value.name}`}><div><span className="badge">{value.length} bytes</span><small>{value.host}</small></div><h3>{value.name}</h3><p>{new Date(value.creationDate).toLocaleString()}</p><code>content not present</code><footer><button className="danger-link" onClick={() => remove(value)}>Delete metadata</button></footer></article>)}</div>}</section>;
}

export function UpdatesScreen() {
  const { request } = useAuth(); const [version, setVersion] = useState<SerializableVersion | null>(null); const [status, setStatus] = useState<UpdateJobInfo | null>(null); const [error, setError] = useState("");
  const load = useCallback(async () => { setVersion(await request<SerializableVersion>("/api/Updates")); setStatus(await request<UpdateJobInfo>("/api/Updates/Updates/status")); }, [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  return <section className="panel narrow"><div className="panel-title"><div><h2>Fixture update service</h2><p>Checks state only; no package is downloaded or installed.</p></div><span className="badge success">{version ? `${version.major}.${version.minor}.${version.build}.${version.revision}` : "loading"}</span></div>{error && <div className="state error">{error}</div>}<pre>{JSON.stringify(status, null, 2)}</pre><button className="primary" onClick={async () => { await request("/api/Updates", { method: "POST" }); await load(); }}>Run fixture check</button></section>;
}

export function LicenseScreen() {
  const { request } = useAuth(); const [value, setValue] = useState<LicenseValidation | null>(null); const [error, setError] = useState("");
  const load = useCallback(() => request<LicenseValidation>("/api/License").then(setValue), [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  async function save(event: FormEvent<HTMLFormElement>) { event.preventDefault(); const data = new FormData(event.currentTarget); try { setValue(await request<LicenseValidation>("/api/License", { method: "PUT", body: JSON.stringify({ email: String(data.get("email")), key: String(data.get("key")) }) })); event.currentTarget.reset(); } catch (cause) { setError(message(cause)); } }
  return <section className="panel narrow"><div className="panel-title"><div><h2>License metadata</h2><p>Keys are stored only as SHA-256 plus a masked hint and are never sent to a vendor.</p></div><span className="badge">unvalidated</span></div><form className="form-grid" onSubmit={save}><label>Email<input name="email" type="email" required /></label><label>Key<input name="key" type="password" minLength={8} required /></label><button className="primary">Store digest</button></form>{error && <div className="state error">{error}</div>}<pre>{JSON.stringify(value, null, 2)}</pre></section>;
}

function message(cause: unknown) { return cause instanceof Error ? cause.message : "Request failed"; }
