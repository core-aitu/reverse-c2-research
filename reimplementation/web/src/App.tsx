import { useEffect, useState } from "react";
import { AiFunctionsScreen, AiProfilesScreen, AiSessionsScreen } from "./AI";
import { ArtifactsScreen, LicenseScreen, UpdatesScreen } from "./AdminCatalog";
import { NetworkScreen } from "./Network";
import { Login, useAuth } from "./auth";
import { CertificatesScreen, ListenersScreen, ModulesScreen, NotebooksScreen, ScriptsScreen, WordlistsScreen } from "./Catalog";
import { AdaptersScreen, AgentSimulatorScreen, JobsScreen, PipelinesScreen, ResourceScreen } from "./Management";
import { useEvents } from "./useEvents";
import type { Account, DocumentationEntry, SearchResult, Setting } from "./types";

type NavItem = { category: string; label: string; id: string; live?: boolean };

const navigation: NavItem[] = [
  { category: "Overview", label: "Research status", id: "home", live: true },
  { category: "Operations", label: "Listeners", id: "listeners", live: true },
  { category: "Operations", label: "Certificates", id: "certificates", live: true },
  { category: "Operations", label: "Implants", id: "implants", live: true },
  { category: "Operations", label: "Payload Pipelines", id: "pipelines", live: true },
  { category: "Operations", label: "Sessions", id: "sessions", live: true },
  { category: "Operations", label: "Scripts", id: "scripts", live: true },
  { category: "Operations", label: "Wordlists", id: "wordlists", live: true },
  { category: "Operations", label: "Modules", id: "modules", live: true },
  { category: "Operations", label: "PowerShell Notebooks", id: "notebooks", live: true },
  { category: "Operations", label: "Events", id: "events", live: true },
  { category: "Operations", label: "Workflows & Jobs", id: "workflows", live: true },
  { category: "Operations", label: "WinRM Sessions", id: "winrm", live: true },
  { category: "Infrastructure", label: "Tunnels", id: "tunnels", live: true },
  { category: "Infrastructure", label: "Adapters", id: "adapters", live: true },
  { category: "AI", label: "Profiles", id: "ai-profiles", live: true },
  { category: "AI", label: "Functions", id: "ai-functions", live: true },
  { category: "AI", label: "Sessions", id: "ai-sessions", live: true },
  { category: "Documentation", label: "Home", id: "docs-home", live: true },
  { category: "Documentation", label: "Docs", id: "docs", live: true },
  { category: "Documentation", label: "Index", id: "docs-index", live: true },
  { category: "Intel", label: "Network Map", id: "network-map", live: true },
  { category: "Intel", label: "Files", id: "files", live: true },
  { category: "Administration", label: "Users", id: "users", live: true },
  { category: "Administration", label: "Updates", id: "updates", live: true },
  { category: "Administration", label: "License", id: "license", live: true },
  { category: "Administration", label: "Settings", id: "settings", live: true },
];

export default function App() {
  const { identity, loading, logout } = useAuth();
  const [screen, setScreen] = useState("home");
  if (loading) return <div className="loading-screen">Restoring secure session…</div>;
  if (!identity) return <Login />;

  const selected = navigation.find((item) => item.id === screen) ?? navigation[0];
  const categories = [...new Set(navigation.map((item) => item.category))];
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand"><div className="brand-mark small">SI</div><div><strong>Specter</strong><span>research console</span></div></div>
        <nav aria-label="Primary navigation">
          {categories.map((category) => (
            <section key={category}>
              <h2>{category}</h2>
              {navigation.filter((item) => item.category === category).map((item) => (
                <button key={item.id} className={screen === item.id ? "active" : ""} onClick={() => setScreen(item.id)}>
                  <span>{item.label}</span>{item.live && <i title="Implemented" />}
                </button>
              ))}
            </section>
          ))}
        </nav>
        <div className="profile"><span>{identity.username}</span><small>{identity.roles.join(", ")}</small><button onClick={logout}>Sign out</button></div>
      </aside>
      <main className="workspace">
        <header><div><p className="eyebrow">{selected.category}</p><h1>{selected.label}</h1></div><span className="environment">isolated / linux</span></header>
        <Screen id={screen} item={selected} />
      </main>
    </div>
  );
}

function Screen({ id, item }: { id: string; item: NavItem }) {
  if (id === "home") return <Overview />;
  if (id === "events") return <Events />;
  if (id === "docs" || id === "docs-home") return <Documentation />;
  if (id === "docs-index") return <DocumentationIndex />;
  if (id === "settings") return <Settings />;
  if (id === "users") return <Users />;
  if (id === "pipelines") return <PipelinesScreen />;
  if (id === "sessions") return <AgentSimulatorScreen />;
  if (id === "workflows") return <JobsScreen />;
  if (id === "adapters") return <AdaptersScreen />;
  if (id === "scripts") return <ScriptsScreen />;
  if (id === "wordlists") return <WordlistsScreen />;
  if (id === "listeners") return <ListenersScreen />;
  if (id === "certificates") return <CertificatesScreen />;
  if (id === "modules") return <ModulesScreen />;
  if (id === "notebooks") return <NotebooksScreen />;
  if (id === "ai-functions") return <AiFunctionsScreen />;
  if (id === "ai-profiles") return <AiProfilesScreen />;
  if (id === "ai-sessions") return <AiSessionsScreen />;
  if (id === "files") return <ArtifactsScreen />;
  if (id === "updates") return <UpdatesScreen />;
  if (id === "license") return <LicenseScreen />;
  if (id === "network-map") return <NetworkScreen />;
  return <ResourceScreen kind={id} title={item.label} />;
}

function Overview() {
  const { events, status } = useEvents();
  return <div className="stack">
    <section className="hero-panel"><div><p className="eyebrow">SpecterInsight 7.0.0</p><h2>Clean-room management plane</h2><p>Contract-indexed Go services and a responsive web client, grounded in reproducible reverse-engineering evidence.</p></div><div className={`connection ${status}`}>{status}</div></section>
    <div className="metric-grid">
      <Metric value="202 / 202" label="REST contracts indexed" />
      <Metric value="36" label="legacy tables migrated" />
      <Metric value="24 / 24" label="workflow surfaces active" />
      <Metric value={String(events.length)} label="events in browser cache" />
    </div>
    <section className="panel"><div className="panel-title"><h2>First vertical slice</h2><span className="badge success">operational</span></div><div className="pipeline"><span>PostgreSQL</span><b>event row</b><span>Go service</span><b>v1 envelope</b><span>WebSocket</span><b>cursor replay</b><span>Browser</span></div></section>
  </div>;
}

function Metric({ value, label }: { value: string; label: string }) {
  return <section className="metric"><strong>{value}</strong><span>{label}</span></section>;
}

function Events() {
  const { request } = useAuth();
  const { events, status } = useEvents();
  const [error, setError] = useState("");
  async function inject() {
    setError("");
    try { await request("/api/fixtures/event", { method: "POST" }); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "Fixture failed"); }
  }
  return <section className="panel">
    <div className="panel-title"><div><h2>Audit stream</h2><p>{events.length} retained in this view · connection {status}</p></div><button className="secondary" onClick={inject}>Emit fixture-live-window</button></div>
    {error && <p className="error">{error}</p>}
    <div className="table-wrap"><table><thead><tr><th>Time</th><th>Outcome</th><th>Source</th><th>Message</th><th>Details</th></tr></thead><tbody>
      {events.map((event) => <tr key={event.id}><td>{new Date(event.timestamp).toLocaleString()}</td><td><span className={`badge ${event.outcome.toLowerCase()}`}>{event.outcome}</span></td><td>{event.source}</td><td>{event.message}</td><td>{event.details}</td></tr>)}
      {!events.length && <tr><td colSpan={5} className="empty">No events yet.</td></tr>}
    </tbody></table></div>
  </section>;
}

function Documentation() {
  const { request } = useAuth();
  const [entries, setEntries] = useState<DocumentationEntry[]>([]);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<DocumentationEntry | null>(null);
  useEffect(() => { request<DocumentationEntry[]>("/api/Documentation").then((data) => { setEntries(data); setSelected(data[0] ?? null); }); }, [request]);
  async function search() {
    if (!query.trim()) { setResults([]); return; }
    setResults(await request<SearchResult[]>("/api/Documentation/search", { method: "POST", body: JSON.stringify({ query, topK: 10, provider: "postgres", model: "trigram" }) }));
  }
  return <div className="docs-layout">
    <section className="panel doc-list"><div className="search"><input value={query} onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => e.key === "Enter" && search()} placeholder="Search evidence-backed docs"/><button onClick={search}>Search</button></div>
      {(results.length ? results.map((result) => ({ name: result.path, contents: result.content, children: [] })) : entries).map((entry) => <button className={selected?.name === entry.name ? "selected" : ""} key={entry.name} onClick={() => setSelected(entry)}><strong>{entry.name}</strong><span>{entry.contents.slice(0, 88)}</span></button>)}
    </section>
    <article className="panel document"><p className="eyebrow">Documentation entry</p><h2>{selected?.name ?? "Select a document"}</h2><p>{selected?.contents}</p></article>
  </div>;
}

function DocumentationIndex() {
  const { request } = useAuth();
  const [status, setStatus] = useState<Record<string, unknown>>({});
  const load = () => request<Record<string, unknown>>("/api/Documentation/index-status").then(setStatus);
  useEffect(() => { void load(); }, [request]);
  return <section className="panel narrow"><div className="panel-title"><div><h2>Documentation index</h2><p>PostgreSQL trigram search for the first vertical slice.</p></div><span className="badge success">available</span></div><pre>{JSON.stringify(status, null, 2)}</pre><button className="primary" onClick={async () => { await request("/api/Documentation/index", { method: "POST" }); load(); }}>Rebuild index</button></section>;
}

function Settings() {
  const { request } = useAuth();
  const [settings, setSettings] = useState<Setting[]>([]);
  const [saved, setSaved] = useState(false);
  useEffect(() => { request<Setting[]>("/api/Settings").then(setSettings); }, [request]);
  return <section className="panel narrow"><div className="panel-title"><div><h2>Runtime settings</h2><p>Versioned management-plane configuration.</p></div>{saved && <span className="badge success">saved</span>}</div>
    <div className="form-grid">{settings.map((setting, index) => <label key={setting.key}>{setting.key}<input value={setting.value} onChange={(event) => setSettings((current) => current.map((item, currentIndex) => currentIndex === index ? { ...item, value: event.target.value } : item))}/></label>)}</div>
    <button className="primary" onClick={async () => { await request("/api/Settings", { method: "PUT", body: JSON.stringify(settings) }); setSaved(true); window.setTimeout(() => setSaved(false), 1800); }}>Save settings</button>
  </section>;
}

function Users() {
  const { request } = useAuth();
  const [accounts, setAccounts] = useState<Account[]>([]);
  useEffect(() => { request<Account[]>("/api/Accounts").then(setAccounts); }, [request]);
  return <section className="panel"><div className="panel-title"><div><h2>Accounts</h2><p>Role-aware identities stored in the compatible account schema.</p></div><span className="badge">{accounts.length} users</span></div><div className="card-grid">{accounts.map((account) => <article className="account-card" key={account.username}><div className="avatar">{account.username.slice(0,2).toUpperCase()}</div><div><strong>{account.username}</strong><span>{account.roles.join(", ")}</span></div><i className={account.enabled ? "enabled" : "disabled"}/></article>)}</div></section>;
}
