import { type FormEvent, type ReactNode, useCallback, useEffect, useState } from "react";
import { useAuth } from "./auth";
import type { CertificateInfo, Listener, SpecterScript, Wordlist } from "./types";

export function ScriptsScreen() {
  const { request } = useAuth();
  const [values, setValues] = useState<SpecterScript[]>([]);
  const [error, setError] = useState("");
  const load = useCallback(() => request<SpecterScript[]>("/api/Scripts").then(setValues), [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const data = new FormData(event.currentTarget);
    try {
      await request("/api/Scripts", { method: "PUT", body: JSON.stringify({
        identifier: String(data.get("identifier")), name: String(data.get("name")),
        description: String(data.get("description")), documentation: "Clean-room metadata fixture.",
        scriptBlock: "", version: 1, isPremium: false,
        labels: splitList(String(data.get("labels"))), enabled: true,
      }) });
      event.currentTarget.reset(); await load();
    } catch (cause) { setError(message(cause)); }
  }
  async function remove(identifier: string) {
    await request(`/api/Scripts?identifier=${encodeURIComponent(identifier)}`, { method: "DELETE" }); await load();
  }
  return <CatalogLayout title="Scripts" subtitle="Typed SpecterScript DTO backed by the legacy script and label tables" form={<form className="inline-form" onSubmit={save}>
    <label>Identifier<input name="identifier" required pattern="[a-zA-Z0-9._-]+" /></label><label>Name<input name="name" required /></label>
    <label>Description<input name="description" /></label><label>Labels<input name="labels" placeholder="fixture, research" /></label><button className="primary">Save</button>
  </form>} error={error} empty={!values.length}>
    {values.map((value) => <article className="resource-card" key={value.identifier}><div><span className={`badge ${value.enabled ? "success" : ""}`}>{value.enabled ? "enabled" : "disabled"}</span><small>{value.identifier} · v{value.version}</small></div><h3>{value.name}</h3><p>{value.description || "No description"}</p><code>{value.labels.join(" · ") || "no labels"}</code><footer><span>{value.isPremium ? "premium" : "standard"}</span><button className="danger-link" onClick={() => remove(value.identifier)}>Delete</button></footer></article>)}
  </CatalogLayout>;
}

export function WordlistsScreen() {
  const { request } = useAuth();
  const [values, setValues] = useState<Wordlist[]>([]);
  const [error, setError] = useState("");
  const load = useCallback(() => request<Wordlist[]>("/api/Wordlists").then(setValues), [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const data = new FormData(event.currentTarget);
    try {
      await request("/api/Wordlists", { method: "POST", body: JSON.stringify({ name: String(data.get("name")), description: String(data.get("description")), tags: splitList(String(data.get("tags"))), content: String(data.get("content")) }) });
      event.currentTarget.reset(); await load();
    } catch (cause) { setError(message(cause)); }
  }
  async function remove(id: number) { await request(`/api/Wordlists/${id}`, { method: "DELETE" }); await load(); }
  return <CatalogLayout title="Wordlists" subtitle="Legacy WordlistResponse with tag filtering and derived entryCount" form={<form className="inline-form catalog-form" onSubmit={save}>
    <label>Name<input name="name" required /></label><label>Description<input name="description" /></label><label>Tags<input name="tags" placeholder="fixture, sample" /></label><label className="wide">Entries<textarea name="content" required rows={3} placeholder={"alpha\nbeta\ngamma"} /></label><button className="primary">Import entries</button>
  </form>} error={error} empty={!values.length}>
    {values.map((value) => <article className="resource-card" key={value.id}><div><span className="badge success">{value.entryCount} entries</span><small>#{value.id}</small></div><h3>{value.name}</h3><p>{value.description || "No description"}</p><code>{value.tags.join(" · ") || "no tags"}</code><footer><time>{new Date(value.updatedAt).toLocaleString()}</time><button className="danger-link" onClick={() => remove(value.id)}>Delete</button></footer></article>)}
  </CatalogLayout>;
}

export function ListenersScreen() {
  const { request } = useAuth();
  const [values, setValues] = useState<Listener[]>([]);
  const [error, setError] = useState("");
  const load = useCallback(() => request<Listener[]>("/api/Listeners").then(setValues), [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const data = new FormData(event.currentTarget);
    try { await request("/api/Listeners", { method: "POST", body: JSON.stringify({ prefix: String(data.get("prefix")), certificateName: String(data.get("certificateName")), webDavPath: String(data.get("webDavPath")) }) }); event.currentTarget.reset(); await load(); }
    catch (cause) { setError(message(cause)); }
  }
  async function toggle(value: Listener) { await request("/api/Listeners", { method: "PUT", body: JSON.stringify({ prefix: value.prefix, enabled: !value.enabled }) }); await load(); }
  async function remove(prefix: string) { await request("/api/Listeners", { method: "DELETE", body: JSON.stringify(prefix) }); await load(); }
  return <CatalogLayout title="Listener configurations" subtitle="Configuration registry only; this service never opens the declared socket" form={<form className="inline-form" onSubmit={save}>
    <label>Absolute prefix<input name="prefix" type="url" required placeholder="http://127.0.0.1:8080/" /></label><label>Certificate<input name="certificateName" /></label><label>WebDAV path<input name="webDavPath" placeholder="/dav" /></label><button className="primary">Add config</button>
  </form>} error={error} empty={!values.length}>
    {values.map((value) => <article className="resource-card" key={value.prefix}><div><span className={`badge ${value.enabled ? "success" : ""}`}>{value.enabled ? "enabled" : "disabled"}</span><small>no socket opened</small></div><h3>{value.prefix}</h3><p>{value.certificateName || "HTTP without certificate"}</p><code>{value.webDavPath || "no WebDAV mount"}</code><footer><button className="secondary" onClick={() => toggle(value)}>{value.enabled ? "Disable" : "Enable"}</button><button className="danger-link" onClick={() => remove(value.prefix)}>Delete</button></footer></article>)}
  </CatalogLayout>;
}

export function CertificatesScreen() {
  const { request } = useAuth();
  const [values, setValues] = useState<CertificateInfo[]>([]);
  const [error, setError] = useState("");
  const load = useCallback(() => request<CertificateInfo[]>("/api/Certificates").then(setValues), [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const data = new FormData(event.currentTarget); const start = new Date(); const end = new Date(start); end.setUTCFullYear(end.getUTCFullYear() + 1);
    try { await request("/api/Certificates", { method: "PUT", body: JSON.stringify({ name: String(data.get("name")), commonName: String(data.get("commonName")), country: String(data.get("country")), state: "", locality: "", organization: "Specter Research", organizationalUnit: "Clean Room", start: start.toISOString(), end: end.toISOString() }) }); event.currentTarget.reset(); await load(); }
    catch (cause) { setError(message(cause)); }
  }
  async function remove(name: string) { await request(`/api/Certificates/Certificates/${encodeURIComponent(name)}`, { method: "DELETE" }); await load(); }
  return <CatalogLayout title="Certificate metadata" subtitle="Typed CertificateInfo registry; private keys are never accepted by this endpoint" form={<form className="inline-form" onSubmit={save}>
    <label>Name<input name="name" required pattern="[a-zA-Z0-9._-]+" /></label><label>Common name<input name="commonName" required /></label><label>Country<input name="country" maxLength={2} /></label><button className="primary">Save metadata</button>
  </form>} error={error} empty={!values.length}>
    {values.map((value) => <article className="resource-card" key={value.name}><div><span className="badge success">metadata</span><small>{value.country || "--"}</small></div><h3>{value.name}</h3><p>{value.commonName}</p><code>{new Date(value.start).toLocaleDateString()} – {new Date(value.end).toLocaleDateString()}</code><footer><span>{value.organization}</span><button className="danger-link" onClick={() => remove(value.name)}>Delete</button></footer></article>)}
  </CatalogLayout>;
}

function CatalogLayout({ title, subtitle, form, error, empty, children }: { title: string; subtitle: string; form: ReactNode; error: string; empty: boolean; children: ReactNode }) {
  return <section className="panel"><div className="panel-title"><div><h2>{title}</h2><p>{subtitle}</p></div><span className="badge success">typed legacy DTO</span></div>{form}{error && <div className="state error" role="alert">{error}</div>}{empty ? <div className="state empty">No records yet.</div> : <div className="resource-grid">{children}</div>}</section>;
}

function splitList(value: string) { return value.split(",").map((item) => item.trim()).filter(Boolean); }
function message(cause: unknown) { return cause instanceof Error ? cause.message : "Request failed"; }
