import { type FormEvent, useCallback, useEffect, useState } from "react";
import { useAuth } from "./auth";
import type { AiAgentDefinition, AiFunctionDefinition, RunAiFunctionResult } from "./types";

export function AiFunctionsScreen() {
  const { request } = useAuth();
  const [values, setValues] = useState<AiFunctionDefinition[]>([]);
  const [outputs, setOutputs] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const load = useCallback(() => request<AiFunctionDefinition[]>("/api/AiFunctionDefinitions").then(setValues), [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const data = new FormData(event.currentTarget); const argument = String(data.get("argument") || "text");
    try {
      await request("/api/AiFunctionDefinitions", { method: "POST", body: JSON.stringify({
        functionId: String(data.get("functionId")), name: String(data.get("name")), description: String(data.get("description")),
        documentation: "Deterministic clean-room fixture function.", approvalLevel: String(data.get("approvalLevel")),
        definition: JSON.stringify({ version: 1, operation: String(data.get("operation")), argument }),
      }) }); event.currentTarget.reset(); await load();
    } catch (cause) { setError(message(cause)); }
  }
  async function run(value: AiFunctionDefinition) {
    const definition = JSON.parse(value.definition) as { argument?: string };
    try {
      const result = await request<RunAiFunctionResult>(`/api/AiFunctionDefinitions/${encodeURIComponent(value.functionId)}/run`, { method: "POST", body: JSON.stringify({ [definition.argument ?? "text"]: "fixture-live-window" }) });
      setOutputs((current) => ({ ...current, [value.functionId]: result.output }));
    } catch (cause) { setError(message(cause)); }
  }
  async function remove(id: string) { await request(`/api/AiFunctionDefinitions/${encodeURIComponent(id)}`, { method: "DELETE" }); await load(); }
  return <section className="panel"><div className="panel-title"><div><h2>AI fixture functions</h2><p>Typed legacy DTO with a deterministic, non-executable fixture DSL.</p></div><span className="badge success">no code execution</span></div>
    <form className="inline-form catalog-form" onSubmit={save}><label>Function ID<input name="functionId" required pattern="[a-zA-Z0-9._-]+" /></label><label>Name<input name="name" required /></label><label>Description<input name="description" /></label><label>Operation<select name="operation"><option value="echo">echo</option><option value="sha256">sha256</option></select></label><label>Argument key<input name="argument" defaultValue="text" required /></label><label>Approval<select name="approvalLevel"><option>Low</option><option>Medium</option><option>High</option></select></label><button className="primary">Save fixture</button></form>
    {error && <div className="state error" role="alert">{error}</div>}{!values.length ? <div className="state empty">No fixture functions yet.</div> : <div className="resource-grid">{values.map((value) => <article className="resource-card" key={value.functionId}><div><span className="badge success">{value.approvalLevel}</span><small>{value.functionId}</small></div><h3>{value.name}</h3><p>{value.description}</p><code>{value.definition}</code>{outputs[value.functionId] && <pre>{outputs[value.functionId]}</pre>}<footer><button className="secondary" onClick={() => run(value)}>Run fixture</button><button className="danger-link" onClick={() => remove(value.functionId)}>Delete</button></footer></article>)}</div>}
  </section>;
}

export function AiProfilesScreen() {
  const { request } = useAuth();
  const [values, setValues] = useState<AiAgentDefinition[]>([]);
  const [error, setError] = useState("");
  const load = useCallback(() => request<AiAgentDefinition[]>("/api/AiAgentDefinitions").then(setValues), [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const data = new FormData(event.currentTarget);
    try {
      await request("/api/AiAgentDefinitions", { method: "POST", body: JSON.stringify({ agentId: String(data.get("agentId")), name: String(data.get("name")), description: String(data.get("description")), prompt: String(data.get("prompt")), includeSpecterScripts: false, accessibleFunctions: splitList(String(data.get("functions"))) }) });
      event.currentTarget.reset(); await load();
    } catch (cause) { setError(message(cause)); }
  }
  async function remove(id: string) { await request(`/api/AiAgentDefinitions/${encodeURIComponent(id)}`, { method: "DELETE" }); await load(); }
  return <section className="panel"><div className="panel-title"><div><h2>AI profiles</h2><p>Agent metadata and an explicit allowlist of deterministic fixture functions.</p></div><span className="badge success">typed legacy DTO</span></div>
    <form className="inline-form catalog-form" onSubmit={save}><label>Agent ID<input name="agentId" required pattern="[a-zA-Z0-9._-]+" /></label><label>Name<input name="name" required /></label><label>Description<input name="description" /></label><label>Allowed function IDs<input name="functions" placeholder="fixture-echo" /></label><label className="wide">Prompt<textarea name="prompt" rows={3} required /></label><button className="primary">Save profile</button></form>
    {error && <div className="state error" role="alert">{error}</div>}{!values.length ? <div className="state empty">No AI profiles yet.</div> : <div className="resource-grid">{values.map((value) => <article className="resource-card" key={value.agentId}><div><span className="badge">profile</span><small>{value.agentId}</small></div><h3>{value.name}</h3><p>{value.description}</p><code>{value.accessibleFunctions.join(" · ") || "no functions"}</code><footer><span>{value.includeSpecterScripts ? "scripts included" : "scripts excluded"}</span><button className="danger-link" onClick={() => remove(value.agentId)}>Delete</button></footer></article>)}</div>}
  </section>;
}

function splitList(value: string) { return value.split(",").map((item) => item.trim()).filter(Boolean); }
function message(cause: unknown) { return cause instanceof Error ? cause.message : "Request failed"; }
