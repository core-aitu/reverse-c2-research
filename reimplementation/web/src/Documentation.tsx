import { type FormEvent, useEffect, useMemo, useState } from "react";
import { useAuth } from "./auth";
import { buildDocumentationTree, isReadable, selectDocument, type DocumentNode } from "./documentationModel";
import type { DocumentationEntry, SearchResult } from "./types";

type SearchState =
  | { kind: "idle" }
  | { kind: "loading"; query: string }
  | { kind: "ready"; query: string; results: SearchResult[] }
  | { kind: "error"; query: string; message: string };

function errorMessage(cause: unknown): string {
  return cause instanceof Error ? cause.message : "Unexpected request failure";
}

function Tree({ nodes, selectedPath, onSelect }: {
  nodes: DocumentNode[];
  selectedPath: string | null;
  onSelect: (path: string) => void;
}) {
  return <ul className="doc-tree">
    {nodes.map((node) => {
      const canRead = isReadable(node);
      return <li key={node.path}>
        {node.children.length > 0 ? <details open>
          <summary>{node.name}</summary>
          {canRead && <button type="button" className="doc-link" aria-current={selectedPath === node.path ? "page" : undefined} onClick={() => onSelect(node.path)}>Read {node.name}</button>}
          <Tree nodes={node.children} selectedPath={selectedPath} onSelect={onSelect} />
        </details> : <button type="button" className="doc-link" aria-current={selectedPath === node.path ? "page" : undefined} onClick={() => onSelect(node.path)}>{node.name}</button>}
      </li>;
    })}
  </ul>;
}

export function Documentation() {
  const { request } = useAuth();
  const [entries, setEntries] = useState<DocumentationEntry[]>([]);
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [preview, setPreview] = useState<SearchResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [reload, setReload] = useState(0);
  const [query, setQuery] = useState("");
  const [submitted, setSubmitted] = useState<{ query: string } | null>(null);
  const [search, setSearch] = useState<SearchState>({ kind: "idle" });

  const tree = useMemo(() => buildDocumentationTree(entries), [entries]);
  const selected = selectDocument(tree, selectedPath);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setLoadError("");
    request<DocumentationEntry[]>("/api/Documentation", { signal: controller.signal })
      .then((data) => {
        if (controller.signal.aborted) return;
        if (!Array.isArray(data)) throw new Error("Invalid documentation response");
        setEntries(data);
      })
      .catch((cause: unknown) => { if (!controller.signal.aborted) setLoadError(errorMessage(cause)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [request, reload]);

  useEffect(() => {
    if (!submitted) return;
    const controller = new AbortController();
    const term = submitted.query;
    setSearch({ kind: "loading", query: term });
    request<SearchResult[]>("/api/Documentation/search", {
      method: "POST",
      signal: controller.signal,
      body: JSON.stringify({ query: term, topK: 10, provider: "postgres", model: "trigram" }),
    }).then((results) => {
      if (controller.signal.aborted) return;
      if (!Array.isArray(results)) throw new Error("Invalid search response");
      setSearch({ kind: "ready", query: term, results });
    }).catch((cause: unknown) => {
      if (!controller.signal.aborted) setSearch({ kind: "error", query: term, message: errorMessage(cause) });
    });
    return () => controller.abort();
  }, [request, submitted]);

  function clearSearch() {
    setSubmitted(null);
    setSearch({ kind: "idle" });
    setQuery("");
    setPreview(null);
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const term = query.trim();
    if (!term) { clearSearch(); return; }
    setPreview(null);
    setSearch({ kind: "loading", query: term });
    setSubmitted({ query: term });
  }

  return <div className="docs-layout">
    <section className="panel doc-list" aria-label="Documentation navigation">
      <div className="panel-title"><h2>Documents</h2><button type="button" className="secondary" disabled={loading} onClick={() => setReload((n) => n + 1)}>Reload</button></div>
      <form className="search" onSubmit={submit}>
        <input aria-label="Search documentation" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search documentation" />
        <button type="submit">Search</button>
      </form>
      {loadError && <p className="state error" role="alert">Unable to load documents: {loadError}</p>}
      {loading && <p className="state loading" role="status">Loading documentation…</p>}
      {!submitted ? <>
        {!loading && !loadError && !tree.length && <p className="state">No documents available.</p>}
        <Tree nodes={tree} selectedPath={selected?.path ?? null} onSelect={(path) => { setSelectedPath(path); setPreview(null); }} />
      </> : <>
        <div className="doc-search-header"><span>Search: {submitted.query}</span><button type="button" className="secondary" onClick={clearSearch}>Clear</button></div>
        {search.kind === "loading" && <p className="state loading" role="status">Searching…</p>}
        {search.kind === "error" && <p className="state error" role="alert">Search failed: {search.message}</p>}
        {search.kind === "ready" && <>
          <p className="muted" role="status">{search.results.length} results</p>
          {!search.results.length && <p className="state">No matching documents.</p>}
          <ul className="doc-tree">{search.results.map((result, index) => <li key={`${result.path}:${index}`}>
            <button type="button" className="doc-link" aria-current={preview === result ? "page" : undefined} onClick={() => setPreview(result)}>
              <strong>{result.title || result.path}</strong><span>{result.path}</span>
            </button>
          </li>)}</ul>
        </>}
      </>}
    </section>
    <article className="panel document">
      <p className="eyebrow">{preview ? "Search result" : "Documentation entry"}</p>
      <h2>{preview?.title || preview?.path || selected?.name || "Select a document"}</h2>
      {(preview || selected) && <p className="doc-path">{preview?.path ?? selected?.path}</p>}
      <p className="doc-content">{preview?.content ?? selected?.contents ?? ""}</p>
    </article>
  </div>;
}
