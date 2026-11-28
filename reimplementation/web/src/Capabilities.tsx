import { useEffect, useState } from "react";
import { useAuth } from "./auth";
import type { CapabilityDefinition } from "./types";

export function CapabilitiesScreen() {
  const { request } = useAuth();
  const [values, setValues] = useState<CapabilityDefinition[]>([]);
  const [error, setError] = useState("");
  useEffect(() => { request<CapabilityDefinition[]>("/api/capabilities").then(setValues).catch(cause => setError(message(cause))); }, [request]);
  const complete = values.filter(value => value.state === "runtime-verified").length;
  return <section className="panel"><div className="panel-title"><div><h2>Compatibility capability matrix</h2><p>Runtime state generated from the same registry used by blocked routes.</p></div><span className="badge success">{complete} / {values.length} runtime verified</span></div>
    {error && <div className="state error" role="alert">{error}</div>}
    <div className="table-wrap"><table><thead><tr><th>Group</th><th>Capability</th><th>State</th><th>Routes</th><th>Evidence</th></tr></thead><tbody>
      {values.map(value => <tr key={value.id}><td>{value.group}</td><td><strong>{value.title}</strong><br/><small>{value.id}</small></td><td><span className={`badge ${value.state === "runtime-verified" ? "success" : value.state === "evidence-blocked" ? "failure" : ""}`}>{value.state}</span></td><td><code>{value.routes.join(" · ")}</code></td><td>{value.evidence || "—"}</td></tr>)}
      {!values.length && !error && <tr><td colSpan={5} className="empty">Loading capability registry…</td></tr>}
    </tbody></table></div>
  </section>;
}

function message(cause: unknown) { return cause instanceof Error ? cause.message : "Request failed"; }
