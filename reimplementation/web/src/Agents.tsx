import { type FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { useAuth } from "./auth";
import type { AgentSession, AgentTask, EnrollmentTokenCreated, EnrollmentTokenInfo, GatewayTaskDetail, GatewayTaskView, ObjectInfo, RealAgent, TaskOutputChunk, TaskOutputEnvelope } from "./types";

export function AgentsScreen() {
  const { request, identity } = useAuth();
  const [agents, setAgents] = useState<RealAgent[]>([]);
  const [simulators, setSimulators] = useState<AgentSession[]>([]);
  const [tasks, setTasks] = useState<GatewayTaskView[]>([]);
  const [simulatorTasks, setSimulatorTasks] = useState<AgentTask[]>([]);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [selectedSimulator, setSelectedSimulator] = useState("");
  const [error, setError] = useState("");
  const canAdminister = identity?.roles.includes("admin") ?? false;

  const load = useCallback(async () => {
    const [real, simulated] = await Promise.all([
      request<RealAgent[]>("/api/gateway/agents"),
      request<AgentSession[]>("/api/gateway/sessions"),
    ]);
    setAgents(real);
    setSimulators(simulated);
    setSelectedAgent((current) => current || real[0]?.id || "");
    setSelectedSimulator((current) => current || simulated[0]?.id || "");
  }, [request]);

  const loadRealTasks = useCallback(async () => {
    if (selectedAgent) setTasks(await request<GatewayTaskView[]>(`/api/gateway/agents/${encodeURIComponent(selectedAgent)}/tasks`));
    else setTasks([]);
  }, [request, selectedAgent]);

  const loadSimulatorTasks = useCallback(async () => {
    if (selectedSimulator) setSimulatorTasks(await request<AgentTask[]>(`/api/gateway/sessions/${encodeURIComponent(selectedSimulator)}/tasks`));
    else setSimulatorTasks([]);
  }, [request, selectedSimulator]);

  useEffect(() => {
    void load().catch((cause) => setError(message(cause)));
    const timer = window.setInterval(() => void load().catch(() => undefined), 5000);
    return () => window.clearInterval(timer);
  }, [load]);
  useEffect(() => {
    void loadRealTasks().catch((cause) => setError(message(cause)));
    const timer = window.setInterval(() => void loadRealTasks().catch(() => undefined), 2000);
    return () => window.clearInterval(timer);
  }, [loadRealTasks]);
  useEffect(() => { void loadSimulatorTasks().catch((cause) => setError(message(cause))); }, [loadSimulatorTasks]);

  async function identityAction(agent: RealAgent, action: "disable" | "revoke") {
    const warning = action === "revoke" ? "Revoke this beacon identity permanently?" : "Disable this beacon identity?";
    if (!window.confirm(`${warning}\n\n${agent.id}`)) return;
    setError("");
    try {
      await request(`/api/gateway/agents/${encodeURIComponent(agent.id)}/${action}`, { method: "POST" });
      await load();
    } catch (cause) { setError(message(cause)); }
  }

  async function queueEcho() {
    if (!selectedAgent) return;
    const now = Date.now();
    try {
      await request(`/api/gateway/agents/${encodeURIComponent(selectedAgent)}/tasks`, {
        method: "POST",
        body: JSON.stringify({ version: 1, idempotencyKey: `ui-${now}`, kind: "echo", payload: { value: "ui-health-check" }, deadline: new Date(now + 60_000).toISOString(), timeoutMs: 5000 }),
      });
      await loadRealTasks();
    } catch (cause) { setError(message(cause)); }
  }

  async function cancelTask(task: GatewayTaskView) {
    if (!window.confirm(`Request cancellation for task ${task.id}?`)) return;
    try {
      await request(`/api/gateway/agents/${encodeURIComponent(task.agentId)}/tasks/${encodeURIComponent(task.id)}/cancel`, { method: "POST" });
      await loadRealTasks();
    } catch (cause) { setError(message(cause)); }
  }

  async function registerSimulator() {
    try {
      const value = await request<AgentSession>("/api/gateway/simulate/register", { method: "POST", body: JSON.stringify({ name: `browser-simulator-${simulators.length + 1}`, metadata: { platform: "fixture", protocolVersion: 1 } }) });
      setSelectedSimulator(value.id);
      await load();
    } catch (cause) { setError(message(cause)); }
  }

  async function simulatorCheckin() {
    try {
      setSimulatorTasks(await request<AgentTask[]>(`/api/gateway/simulate/${encodeURIComponent(selectedSimulator)}/checkin`, { method: "POST" }));
    } catch (cause) { setError(message(cause)); }
  }

  const selected = agents.find((agent) => agent.id === selectedAgent);
  return <div className="stack">
    {error && <div className="state error" role="alert">{error}</div>}
    <section className="panel" data-testid="real-beacons">
      <div className="panel-title"><div><p className="eyebrow">Real agent plane</p><h2>Beacon sessions</h2><p>Live identities are kept separate from simulator fixtures. Operational responses never include credentials or task payloads.</p></div><div className="button-row"><span className="badge">{agents.length} real</span>{canAdminister && <button className="primary" disabled={!selectedAgent} onClick={queueEcho}>Queue safe echo</button>}</div></div>
      <div className="session-tabs">{agents.map((agent) => <button className={selectedAgent === agent.id ? "selected" : ""} onClick={() => setSelectedAgent(agent.id)} key={agent.id}><strong>{agent.name}</strong><span><i className={`health-dot ${agent.health}`} /> {agent.health} · real</span></button>)}</div>
      {!agents.length && <div className="state empty">No enrolled beacons. Create a one-use token on the Enrollment page.</div>}
      {selected && <div className="agent-detail-grid">
        <article><small>Protocol / build</small><strong>v{selected.protocolVersion} · {selected.buildVersion || "unknown"}</strong></article>
        <article><small>Platform</small><strong>{selected.os || "unknown"} / {selected.architecture || "unknown"}</strong></article>
        <article><small>First seen</small><strong>{formatTime(selected.firstSeen)}</strong></article>
        <article><small>Last check-in</small><strong>{formatTime(selected.lastCheckin)}</strong></article>
        <article><small>Next expected</small><strong>{formatTime(selected.nextExpectedCheckin)}</strong></article>
        <article><small>Capabilities</small><strong>{selected.capabilities.join(", ") || "none"}</strong></article>
        <article><small>Replay state</small><strong>seq {selected.lastSequence} · cursor {selected.lastCursor}</strong></article>
        <article><small>Identity</small><strong>generation {selected.generation} · {selected.enabled ? "enabled" : "disabled"}</strong></article>
      </div>}
      {selected && canAdminister && <div className="button-row destructive-actions"><button className="secondary" disabled={!selected.enabled} onClick={() => identityAction(selected, "disable")}>Disable…</button><button className="danger" disabled={Boolean(selected.revokedAt)} onClick={() => identityAction(selected, "revoke")}>Revoke…</button></div>}
      {selected && canAdminister && <CommandConsole agent={selected} tasks={tasks} reloadTasks={loadRealTasks} reportError={setError} />}
      {!!selected && <div className="table-wrap"><table><thead><tr><th>Task / correlation</th><th>Type</th><th>Status</th><th>Lease / retries</th><th>Structured error</th><th /></tr></thead><tbody>
        {tasks.map((task) => <tr key={task.id}><td><code>{task.correlationId}</code><br/><small>v{task.version} · {formatTime(task.createdAt)}</small></td><td>{task.kind}</td><td><span className={`badge ${task.status === "completed" ? "success" : task.status === "failed" ? "failure" : ""}`}>{task.status}</span></td><td>{task.leaseUntil ? formatTime(task.leaseUntil) : "—"}<br/><small>{task.dispatchCount} dispatch · {task.retries} retries</small></td><td>{task.error ? <><code>{task.error.code}</code><br/><small>{task.error.message} · retryable {String(task.error.retryable)}</small></> : "—"}</td><td>{canAdminister && ["queued", "leased", "running"].includes(task.status) && <button className="danger-link" onClick={() => cancelTask(task)}>Cancel…</button>}</td></tr>)}
        {!tasks.length && <tr><td colSpan={6} className="empty">No tasks for this beacon.</td></tr>}
      </tbody></table></div>}
    </section>

    <section className="panel" data-testid="simulator-sessions">
      <div className="panel-title"><div><p className="eyebrow">Simulator plane</p><h2>Simulator sessions</h2><p>Fixture-only lifecycle; never confused with enrolled beacons.</p></div><div className="button-row"><span className="badge">{simulators.length} simulator</span>{canAdminister && <button className="secondary" onClick={registerSimulator}>Register simulator</button>}{canAdminister && <button className="primary" disabled={!selectedSimulator} onClick={simulatorCheckin}>Check in</button>}</div></div>
      <div className="session-tabs">{simulators.map((session) => <button className={selectedSimulator === session.id ? "selected" : ""} onClick={() => setSelectedSimulator(session.id)} key={session.id}><strong>{session.name}</strong><span>{session.status} · simulator</span></button>)}</div>
      {!simulators.length && <div className="state empty">No simulator fixtures.</div>}
      {!!selectedSimulator && <div className="table-wrap"><table><thead><tr><th>Task</th><th>Kind</th><th>Status</th><th>Created</th></tr></thead><tbody>{simulatorTasks.map((task) => <tr key={task.id}><td><code>{task.id}</code></td><td>{task.kind}</td><td>{task.status}</td><td>{formatTime(task.createdAt)}</td></tr>)}{!simulatorTasks.length && <tr><td colSpan={4} className="empty">No simulator tasks.</td></tr>}</tbody></table></div>}
    </section>
  </div>;
}

type CommandConsoleProps = {
  agent: RealAgent;
  tasks: GatewayTaskView[];
  reloadTasks: () => Promise<void>;
  reportError: (value: string) => void;
};

function CommandConsole({ agent, tasks, reloadTasks, reportError }: CommandConsoleProps) {
  const { request, download } = useAuth();
  const [mode, setMode] = useState<"exec" | "sh" | "cmd" | "powershell">("exec");
  const [selectedTask, setSelectedTask] = useState("");
  const [detail, setDetail] = useState<GatewayTaskDetail | null>(null);
  const [chunks, setChunks] = useState<TaskOutputChunk[]>([]);
  const [archivedOutput, setArchivedOutput] = useState({ stdout: "", stderr: "" });
  const [streamState, setStreamState] = useState("idle");
  const [busy, setBusy] = useState(false);
  const lastChunk = useRef(-1);
  const supports = (value: string) => agent.capabilities.includes(value);

  const loadDetail = useCallback(async (taskID: string) => {
    if (!taskID) { setDetail(null); return; }
    const value = await request<GatewayTaskDetail>(`/api/gateway/agents/${encodeURIComponent(agent.id)}/tasks/${encodeURIComponent(taskID)}`);
    setDetail(value);
  }, [agent.id, request]);

  useEffect(() => {
    setSelectedTask(""); setDetail(null); setChunks([]); setArchivedOutput({ stdout: "", stderr: "" }); lastChunk.current = -1;
  }, [agent.id]);

  const archivedOutputKey = (detail?.outputs ?? []).map((value) => `${value.stream}:${value.object.id}`).sort().join("|");
  useEffect(() => {
    let stale = false;
    if (!archivedOutputKey || !detail?.outputs?.length) {
      setArchivedOutput({ stdout: "", stderr: "" });
      return () => { stale = true; };
    }
    const outputs = detail.outputs;
    void Promise.all(outputs.map(async (value) => {
      const blob = await download(`/api/objects/${encodeURIComponent(value.object.id)}/content`);
      const content = new TextDecoder().decode(await blob.arrayBuffer());
      return [value.stream, content] as const;
    })).then((values) => {
      if (stale) return;
      const next = { stdout: "", stderr: "" };
      for (const [stream, content] of values) {
        if (stream === "stdout" || stream === "stderr") next[stream] = content;
      }
      setArchivedOutput(next);
    }).catch((cause) => { if (!stale) reportError(message(cause)); });
    return () => { stale = true; };
  }, [archivedOutputKey, download, reportError]);

  useEffect(() => {
    if (!selectedTask) return;
    void loadDetail(selectedTask).catch((cause) => reportError(message(cause)));
    const timer = window.setInterval(() => void loadDetail(selectedTask).catch(() => undefined), 2000);
    return () => window.clearInterval(timer);
  }, [loadDetail, reportError, selectedTask]);

  useEffect(() => {
    if (!selectedTask) { setStreamState("idle"); return; }
    let closed = false;
    let socket: WebSocket | undefined;
    let reconnect: number | undefined;
    let attempts = 0;
    async function connect() {
      try {
        setStreamState(attempts ? "reconnecting" : "connecting");
        const response = await request<{ ticket: string }>(`/api/gateway/agents/${encodeURIComponent(agent.id)}/tasks/${encodeURIComponent(selectedTask)}/output-ticket`, { method: "POST", body: "{}" });
        if (closed) return;
        const scheme = location.protocol === "https:" ? "wss" : "ws";
        socket = new WebSocket(`${scheme}://${location.host}/api/gateway/task-output?ticket=${encodeURIComponent(response.ticket)}&taskId=${encodeURIComponent(selectedTask)}&after=${lastChunk.current}`);
        socket.onopen = () => { attempts = 0; setStreamState("live"); };
        socket.onmessage = (event) => {
          const envelope = JSON.parse(String(event.data)) as TaskOutputEnvelope;
          const chunk = normalizeChunk(envelope);
          if (!chunk || chunk.taskId !== selectedTask) return;
          lastChunk.current = Math.max(lastChunk.current, chunk.sequence ?? chunk.index);
          setChunks((current) => {
            const key = `${chunk.stream}:${chunk.index}`;
            const byKey = new Map(current.map((value) => [`${value.stream}:${value.index}`, value]));
            byKey.set(key, chunk);
            return [...byKey.values()].sort((left, right) => left.index - right.index || left.stream.localeCompare(right.stream));
          });
        };
        socket.onclose = () => {
          if (closed) return;
          setStreamState("reconnecting"); attempts += 1;
          reconnect = window.setTimeout(connect, Math.min(1000 * 2 ** attempts, 10000));
        };
      } catch {
        if (!closed) { setStreamState("reconnecting"); attempts += 1; reconnect = window.setTimeout(connect, Math.min(1000 * 2 ** attempts, 10000)); }
      }
    }
    setChunks([]); lastChunk.current = -1; void connect();
    return () => { closed = true; if (reconnect) window.clearTimeout(reconnect); socket?.close(); };
  }, [agent.id, request, selectedTask]);

  async function queue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); reportError("");
    const form = event.currentTarget;
    const data = new FormData(form);
    const timeoutSeconds = Math.max(1, Math.min(300, Number(data.get("timeout")) || 60));
    const env = parseEnvironment(String(data.get("environment") ?? ""));
    const cwd = String(data.get("cwd") ?? "").trim();
    const value = String(data.get("command") ?? "").trim();
    const payload = mode === "exec"
      ? { executable: value, args: lines(String(data.get("arguments") ?? "")), ...(cwd && { cwd }), ...(Object.keys(env).length && { env }) }
      : { interpreter: mode, command: value, ...(cwd && { cwd }), ...(Object.keys(env).length && { env }) };
    const now = Date.now();
    try {
      const task = await request<GatewayTaskView>(`/api/gateway/agents/${encodeURIComponent(agent.id)}/tasks`, {
        method: "POST",
        body: JSON.stringify({ version: 2, idempotencyKey: `ui-command-${now}`, kind: mode === "exec" ? "exec" : "shell", payload, deadline: new Date(now + timeoutSeconds * 1000 + 60_000).toISOString(), timeoutMs: timeoutSeconds * 1000 }),
      });
      setSelectedTask(task.id); setChunks([]); setArchivedOutput({ stdout: "", stderr: "" }); lastChunk.current = -1;
      await reloadTasks(); form.reset(); setMode("exec");
    } catch (cause) { reportError(message(cause)); }
    finally { setBusy(false); }
  }

  async function requestFile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); reportError("");
    const form = event.currentTarget;
    const path = String(new FormData(form).get("path") ?? "").trim();
    const now = Date.now();
    try {
      const task = await request<GatewayTaskView>(`/api/gateway/agents/${encodeURIComponent(agent.id)}/tasks`, {
        method: "POST",
        body: JSON.stringify({ version: 2, idempotencyKey: `ui-file-${now}`, kind: "file-download", payload: { path }, deadline: new Date(now + 360_000).toISOString(), timeoutMs: 300_000 }),
      });
      setSelectedTask(task.id); await reloadTasks(); form.reset();
    } catch (cause) { reportError(message(cause)); }
    finally { setBusy(false); }
  }

  async function saveObject(value: ObjectInfo) {
    try { saveBlob(await download(`/api/objects/${encodeURIComponent(value.id)}/content`), value.name || `${value.kind}-${value.id}`); }
    catch (cause) { reportError(message(cause)); }
  }

  const liveStdout = chunks.filter((value) => value.stream === "stdout").map(chunkText).join("");
  const liveStderr = chunks.filter((value) => value.stream === "stderr").map(chunkText).join("");
  const stdout = liveStdout || archivedOutput.stdout;
  const stderr = liveStderr || archivedOutput.stderr;
  const objects = detailObjects(detail);
  const selectedExecutorAvailable = mode === "exec" ? supports("exec") : supports(`shell:${mode}`);
  return <section className="command-console" data-testid="command-console">
    <div className="panel-title"><div><h3>Command console</h3><p>Commands run under the beacon process identity. Output is rendered as plain text.</p></div><span className={`connection ${streamState === "live" ? "live" : "reconnecting"}`}>{streamState}</span></div>
    <form className="command-form" onSubmit={queue}>
      <label>Executor<select value={mode} onChange={(event) => setMode(event.target.value as typeof mode)}><option value="exec" disabled={!supports("exec")}>Direct executable</option><option value="sh" disabled={!supports("shell:sh")}>sh</option><option value="cmd" disabled={!supports("shell:cmd")}>cmd</option><option value="powershell" disabled={!supports("shell:powershell")}>PowerShell</option></select></label>
      <label className="wide">{mode === "exec" ? "Executable" : "Command"}<input name="command" required autoComplete="off" placeholder={mode === "exec" ? "/usr/bin/id" : "Write-Output ready"} /></label>
      {mode === "exec" && <label className="wide">Arguments (one per line)<textarea name="arguments" rows={3} placeholder={"--version\n--verbose"} /></label>}
      <label>Working directory<input name="cwd" autoComplete="off" placeholder="optional" /></label><label>Timeout (seconds)<input name="timeout" type="number" min={1} max={300} defaultValue={60} /></label>
      <label className="wide">Environment overlay (KEY=value)<textarea name="environment" rows={2} placeholder="LANG=C" /></label>
      <button className="primary" disabled={busy || !agent.enabled || !selectedExecutorAvailable}>{busy ? "Queueing…" : "Run command"}</button>
    </form>
    <form className="file-request-form" onSubmit={requestFile}><label>Remote file path<input name="path" required autoComplete="off" placeholder="/var/tmp/report.txt" /></label><button className="secondary" disabled={busy || !agent.enabled || !supports("file-download")}>Retrieve file</button></form>
    <label className="task-history">Task history<select value={selectedTask} onChange={(event) => setSelectedTask(event.target.value)}><option value="">Select a task</option>{tasks.map((task) => <option value={task.id} key={task.id}>{task.kind} · {task.status} · {task.id}</option>)}</select></label>
    {selectedTask && <div className="terminal-grid">
      <section><header><strong>stdout</strong><span>{detail?.result?.stdoutBytes ?? 0} bytes</span></header><pre>{stdout || "No stdout received."}</pre></section>
      <section><header><strong>stderr</strong><span>{detail?.result?.stderrBytes ?? 0} bytes</span></header><pre>{stderr || "No stderr received."}</pre></section>
    </div>}
    {detail?.result && <div className="command-result"><span>exit <strong>{detail.result.exitCode ?? "—"}</strong></span><span>{detail.result.durationMs ?? 0} ms</span>{detail.result.truncated && <span className="badge failure">output truncated</span>}</div>}
    {!!objects.length && <div className="object-results"><h4>Result objects</h4>{objects.map((value) => <button className="secondary" onClick={() => saveObject(value)} key={value.id}>Download {value.name || value.kind} · {formatBytes(value.size)}</button>)}</div>}
  </section>;
}

function normalizeChunk(envelope: TaskOutputEnvelope): TaskOutputChunk | null {
  const value = envelope.payload ?? envelope;
  if (!value.taskId || (value.stream !== "stdout" && value.stream !== "stderr") || typeof value.index !== "number") return null;
  return { ...value, sequence: envelope.id ?? value.sequence } as TaskOutputChunk;
}
function chunkText(value: TaskOutputChunk) {
  const encoded = value.text ?? value.data ?? "";
  if (value.encoding !== "base64") return encoded;
  try { const binary = atob(encoded); return new TextDecoder().decode(Uint8Array.from(binary, (character) => character.charCodeAt(0))); }
  catch { return "[invalid output chunk]"; }
}
function lines(value: string) { return value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean); }
function parseEnvironment(value: string) {
  const result: Record<string, string> = {};
  for (const line of lines(value)) { const separator = line.indexOf("="); if (separator > 0) result[line.slice(0, separator)] = line.slice(separator + 1); }
  return result;
}
function detailObjects(value: GatewayTaskDetail | null) {
  if (!value) return [];
  const objects = (value.outputs ?? []).map((output) => output.object);
  if (value.result?.object && !objects.some((item) => item.id === value.result?.object?.id)) objects.push(value.result.object);
  return objects;
}
function formatBytes(value: number) { return value >= 1024 * 1024 ? `${(value / 1024 / 1024).toFixed(1)} MiB` : `${Math.ceil(value / 1024)} KiB`; }
function saveBlob(blob: Blob, name: string) { const url = URL.createObjectURL(blob); const anchor = document.createElement("a"); anchor.href = url; anchor.download = name; anchor.click(); URL.revokeObjectURL(url); }

export function EnrollmentTokensScreen() {
  const { request } = useAuth();
  const [tokens, setTokens] = useState<EnrollmentTokenInfo[]>([]);
  const [created, setCreated] = useState<EnrollmentTokenCreated | null>(null);
  const [error, setError] = useState("");
  const load = useCallback(() => request<EnrollmentTokenInfo[]>("/api/gateway/enrollment-tokens").then(setTokens), [request]);
  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);

  async function create() {
    setCreated(null); setError("");
    try {
      const value = await request<EnrollmentTokenCreated>("/api/gateway/enrollment-tokens", { method: "POST", body: JSON.stringify({ ttlSeconds: 600 }) });
      setCreated(value);
      await load();
    } catch (cause) { setError(message(cause)); }
  }

  return <section className="panel narrow" data-testid="enrollment-tokens"><div className="panel-title"><div><h2>One-use enrollment tokens</h2><p>The secret is returned once, never persisted by the browser and never returned by the list endpoint.</p></div><button className="primary" onClick={create}>Create 10-minute token</button></div>
    {error && <div className="state error" role="alert">{error}</div>}
    {created && <div className="one-time-secret" role="status"><strong>Copy this secret now</strong><code data-sensitive="true">{created.token}</code><p>It cannot be displayed again.</p><button className="secondary" onClick={() => setCreated(null)}>I stored it — hide secret</button></div>}
    <div className="table-wrap"><table><thead><tr><th>ID</th><th>Created</th><th>Expires</th><th>Status</th><th>Created by</th></tr></thead><tbody>{tokens.map((token) => <tr key={token.id}><td><code>{token.id}</code></td><td>{formatTime(token.createdAt)}</td><td>{formatTime(token.expiresAt)}</td><td><span className={`badge ${token.usedAt ? "" : new Date(token.expiresAt) > new Date() ? "success" : "failure"}`}>{token.usedAt ? "used" : new Date(token.expiresAt) > new Date() ? "available" : "expired"}</span></td><td>{token.createdBy}</td></tr>)}{!tokens.length && <tr><td colSpan={5} className="empty">No enrollment tokens have been created.</td></tr>}</tbody></table></div>
  </section>;
}

function formatTime(value: string) { return new Date(value).toLocaleString(); }
function message(cause: unknown) { return cause instanceof Error ? cause.message : "Request failed"; }
