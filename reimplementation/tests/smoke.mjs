const baseURL = process.env.SPECTER_BASE_URL ?? "http://localhost:3000";
const username = process.env.SPECTER_ADMIN_USERNAME ?? "admin";
const password = process.env.SPECTER_ADMIN_PASSWORD;

if (!password) throw new Error("SPECTER_ADMIN_PASSWORD is required");

async function json(path, init = {}) {
  const response = await fetch(`${baseURL}${path}`, init);
  if (!response.ok) throw new Error(`${init.method ?? "GET"} ${path}: ${response.status}`);
  return response.json();
}

const auth = await json("/api/Accounts/authenticate", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ username, password }),
});
const headers = { Authorization: `Bearer ${auth.jwtToken}`, "Content-Type": "application/json" };
const identity = await json("/api/Identity", { headers });
if (identity.username !== username) throw new Error("identity mismatch");
const search = await json("/api/Documentation/search", {
  method: "POST",
  headers,
  body: JSON.stringify({ query: "vertical slice", topK: 10, provider: "postgres", model: "trigram" }),
});
if (!search.some((result) => result.path === "Guide/Sample.md")) {
  throw new Error("documentation search mismatch");
}

const before = await json("/api/events/latest", { headers });
const cursor = before.reduce((maximum, event) => Math.max(maximum, event.id), 0);
const { ticket } = await json("/api/events/ticket", { method: "POST", headers });
const socketURL = new URL("/api/events", baseURL);
socketURL.protocol = socketURL.protocol === "https:" ? "wss:" : "ws:";
socketURL.searchParams.set("ticket", ticket);
socketURL.searchParams.set("after", String(cursor));

const observed = await new Promise((resolve, reject) => {
  const timeout = setTimeout(() => reject(new Error("WebSocket event timeout")), 10000);
  const socket = new WebSocket(socketURL);
  socket.onerror = () => reject(new Error("WebSocket connection failed"));
  socket.onopen = async () => {
    try {
      await json("/api/fixtures/event", { method: "POST", headers });
    } catch (error) {
      reject(error);
    }
  };
  socket.onmessage = (message) => {
    const envelope = JSON.parse(String(message.data));
    if (envelope.payload?.identifier === "fixture-live-window") {
      clearTimeout(timeout);
      socket.close();
      resolve(envelope);
    }
  };
});

await json("/api/Scripts", {
  method: "PUT", headers,
  body: JSON.stringify({ identifier: "smoke-script", name: "Smoke script metadata", description: "No executable body", documentation: "fixture", scriptBlock: "", version: 1, isPremium: false, labels: ["fixture", "smoke"], enabled: true }),
});
const scripts = await json("/api/Scripts", { headers });
if (!scripts.some((item) => item.identifier === "smoke-script" && item.labels.includes("fixture"))) throw new Error("typed script workflow mismatch");

const wordlist = await json("/api/Wordlists", {
  method: "POST", headers,
  body: JSON.stringify({ name: "smoke-wordlist", description: "fixture", tags: ["smoke"], content: "alpha\n\nbeta\ngamma\n" }),
});
if (wordlist.entryCount !== 3) throw new Error("wordlist entryCount mismatch");
const fetchedWordlist = await json(`/api/Wordlists/${wordlist.id}`, { headers });
if (fetchedWordlist.name !== "smoke-wordlist") throw new Error("wordlist GET mismatch");

await json("/api/Certificates", {
  method: "PUT", headers,
  body: JSON.stringify({ name: "smoke-cert", country: "US", state: "", locality: "", organization: "Research", organizationalUnit: "Fixture", commonName: "localhost", start: "2026-01-01T00:00:00Z", end: "2027-01-01T00:00:00Z" }),
});
const certificates = await json("/api/Certificates", { headers });
if (!certificates.some((item) => item.name === "smoke-cert")) throw new Error("certificate metadata mismatch");

await json("/api/Listeners", {
  method: "POST", headers,
  body: JSON.stringify({ prefix: "http://127.0.0.1:8080/", certificateName: "", webDavPath: "" }),
});
const listeners = await json("/api/Listeners", { headers });
if (!listeners.some((item) => item.prefix === "http://127.0.0.1:8080/" && item.enabled)) throw new Error("listener config mismatch");
const traversalListener = await fetch(`${baseURL}/api/Listeners`, {
  method: "POST", headers,
  body: JSON.stringify({ prefix: "http://127.0.0.1:8080/../admin/", certificateName: "", webDavPath: "" }),
});
if (traversalListener.status !== 400) throw new Error("listener traversal was not rejected");
const anonymousScripts = await fetch(`${baseURL}/api/Scripts`);
if (anonymousScripts.status !== 401) throw new Error("typed catalog anonymous access was not rejected");

const adapters = await json("/api/adapters", { headers });
if (adapters.length !== 10) throw new Error("adapter registry mismatch");
const adapter = await json("/api/adapters/elasticsearch/probe", { method: "POST", headers });
if (adapter.status !== "healthy") throw new Error("fixture adapter probe failed");

const pipelines = await json("/api/pipelines", { headers });
const pipelineJob = await json(`/api/pipelines/${pipelines[0].id}/run`, {
  method: "POST", headers, body: JSON.stringify({ input: "specter" }),
});
let completedJob;
for (let attempt = 0; attempt < 20; attempt += 1) {
  const jobs = await json("/api/jobs", { headers });
  completedJob = jobs.find((job) => job.id === pipelineJob.id && job.status === "succeeded");
  if (completedJob) break;
  await new Promise((resolve) => setTimeout(resolve, 100));
}
if (!completedJob?.output?.outputs?.hash) throw new Error("pipeline worker mismatch");

const simulator = await json("/api/gateway/simulate/register", {
  method: "POST", headers, body: JSON.stringify({ name: "smoke-simulator", metadata: { protocolVersion: 1 } }),
});
const queuedTask = await json(`/api/gateway/sessions/${simulator.id}/tasks`, {
  method: "POST", headers, body: JSON.stringify({ kind: "echo", payload: { text: "fixture-live-window" } }),
});
const dispatched = await json(`/api/gateway/simulate/${simulator.id}/checkin`, { method: "POST", headers });
if (!dispatched.some((task) => task.id === queuedTask.id && task.status === "dispatched")) throw new Error("simulator dispatch mismatch");
await json(`/api/gateway/simulate/${simulator.id}/tasks/${queuedTask.id}/result`, {
  method: "POST", headers, body: JSON.stringify({ result: { echoed: "fixture-live-window" }, error: "" }),
});

console.log(JSON.stringify({
  identity: identity.username,
  documentationSearch: search.length,
  event: observed.payload.identifier,
  envelopeVersion: observed.version,
  cursor: observed.id,
  workflows: scripts.length,
  wordlistEntries: wordlist.entryCount,
  certificates: certificates.length,
  listeners: listeners.length,
  adapters: adapters.length,
  pipelineJob: completedJob.status,
  simulator: simulator.id,
}));
