import { useCallback, useEffect, useState } from "react";
import { useAuth } from "./auth";
import type { BeaconBuild, BeaconRelease, GatewayListener } from "./types";

export function BuildsScreen() {
  const { request, download, identity } = useAuth();
  const [releases, setReleases] = useState<BeaconRelease[]>([]);
  const [builds, setBuilds] = useState<BeaconBuild[]>([]);
  const [selected, setSelected] = useState("");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [listener, setListener] = useState<GatewayListener | null>(null);
  const canAdminister = identity?.roles.includes("admin") ?? false;

  const load = useCallback(async () => {
    const [values, listenerValue] = await Promise.all([
      request<BeaconRelease[]>("/api/beacon-releases"),
      request<GatewayListener>("/api/gateway/listener"),
    ]);
    setReleases(values);
    setListener(listenerValue);
    setSelected((current) => current || values[0]?.id || "");
  }, [request]);

  useEffect(() => { void load().catch((cause) => setError(message(cause))); }, [load]);

  async function createBundle() {
    if (!selected) return;
    setBusy(selected); setError("");
    try {
      const build = await request<BeaconBuild>("/api/beacon-builds", {
        method: "POST",
        body: JSON.stringify({ releaseId: selected }),
      });
      setBuilds((current) => [build, ...current.filter((value) => value.id !== build.id)]);
      const blob = await download(`/api/beacon-builds/${encodeURIComponent(build.id)}/bundle`, { method: "POST" });
      saveBlob(blob, `specter-beacon-${build.version}-${build.os}-${build.architecture}.zip`);
    } catch (cause) {
      setError(message(cause));
    } finally {
      setBusy("");
    }
  }

  return <div className="stack">
    <section className="panel listener-status">
      <div className="panel-title"><div><p className="eyebrow">HTTPS gateway listener</p><h2>{listener?.endpoint || "Listener endpoint unavailable"}</h2><p>Bind address {listener?.address || "—"} · TLS {listener?.tlsConfigured ? "configured" : "not configured"}</p></div><span className={`badge ${listener?.status === "online" ? "success" : listener?.status === "offline" ? "failure" : ""}`}>{listener?.status || "loading"}</span></div>
    </section>
    <section className="panel" data-testid="beacon-builds">
      <div className="panel-title"><div><p className="eyebrow">Release matrix</p><h2>Beacon builds</h2><p>Download a short-lived launch bundle assembled from an S3 release binary. Enrollment credentials are never retained by the browser.</p></div><span className="badge success">S3-backed</span></div>
      {error && <div className="state error" role="alert">{error}</div>}
      {!releases.length ? <div className="state empty">No beacon releases have been published.</div> : <>
        <div className="release-grid">{releases.map((release) => <button type="button" className={selected === release.id ? "release-card selected" : "release-card"} onClick={() => setSelected(release.id)} key={release.id}>
          <span><strong>{release.version}</strong><i className={`badge ${release.signed ? "success" : "failure"}`}>{release.signed ? `signature ${release.signatureStatus}` : release.development ? "development" : "signature missing"}</i></span>
          <b>{release.os} / {release.architecture}</b>
          <code>{release.object?.sha256 ?? release.sha256 ?? "digest unavailable"}</code>
          <small>{formatBytes(release.object?.size ?? release.size ?? 0)} · SBOM {release.sbom ? "yes" : "no"} · checksums {release.checksums ? "yes" : "no"} · {formatTime(release.createdAt)}</small>
        </button>)}</div>
        {canAdminister ? <div className="button-row build-actions"><button className="primary" disabled={!selected || Boolean(busy)} onClick={createBundle}>{busy ? "Preparing one-time bundle…" : "Build and download launch bundle"}</button></div> : <div className="state empty">Administrator role is required to generate launch bundles.</div>}
      </>}
    </section>
    {!!builds.length && <section className="panel"><div className="panel-title"><div><h2>This browser session</h2><p>Generated bundles are not persisted in browser storage and should be used before their enrollment token expires.</p></div></div><div className="table-wrap"><table><thead><tr><th>Build</th><th>Target</th><th>Status</th><th>Created</th></tr></thead><tbody>{builds.map((build) => <tr key={build.id}><td><code>{build.id}</code></td><td>{build.os} / {build.architecture}<br/><small>{build.version}</small></td><td><span className="badge success">bundle ready</span></td><td>{formatTime(build.createdAt)}</td></tr>)}</tbody></table></div></section>}
  </div>;
}

function formatTime(value: string) { return value ? new Date(value).toLocaleString() : "—"; }
function formatBytes(value: number) { return value >= 1024 * 1024 ? `${(value / 1024 / 1024).toFixed(1)} MiB` : `${Math.ceil(value / 1024)} KiB`; }
function message(cause: unknown) { return cause instanceof Error ? cause.message : "Request failed"; }
function saveBlob(blob: Blob, name: string) { const url = URL.createObjectURL(blob); const anchor = document.createElement("a"); anchor.href = url; anchor.download = name; anchor.click(); URL.revokeObjectURL(url); }
