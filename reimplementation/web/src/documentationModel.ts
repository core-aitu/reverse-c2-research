import type { DocumentationEntry } from "./types";

export type DocumentNode = {
  path: string;
  name: string;
  // null denotes a generated folder; an empty string is a real entry.
  contents: string | null;
  children: DocumentNode[];
};

/** Accept both nested legacy entries and the current API's flat slash paths. */
export function buildDocumentationTree(entries: DocumentationEntry[]): DocumentNode[] {
  const roots: DocumentNode[] = [];
  const byPath = new Map<string, DocumentNode>();

  function insert(entry: DocumentationEntry, parentPath: string) {
    const parts = entry.name.split("/").filter(Boolean);
    if (!parts.length) return;
    let path = parentPath;
    let siblings = path ? byPath.get(path)!.children : roots;
    let node: DocumentNode | undefined;
    for (const name of parts) {
      path = path ? `${path}/${name}` : name;
      node = byPath.get(path);
      if (!node) {
        node = { path, name, contents: null, children: [] };
        byPath.set(path, node);
        siblings.push(node);
      }
      siblings = node.children;
    }
    node!.contents = entry.contents;
    for (const child of entry.children ?? []) insert(child, path);
  }

  for (const entry of entries) insert(entry, "");
  return roots;
}

export function isReadable(node: DocumentNode): boolean {
  return node.contents !== null && (node.contents !== "" || !node.children.length);
}

export function findDocument(nodes: DocumentNode[], path: string): DocumentNode | undefined {
  for (const node of nodes) {
    if (node.path === path && isReadable(node)) return node;
    const child = findDocument(node.children, path);
    if (child) return child;
  }
}

export function firstDocument(nodes: DocumentNode[]): DocumentNode | undefined {
  for (const node of nodes) {
    if (isReadable(node)) return node;
    const child = firstDocument(node.children);
    if (child) return child;
  }
}

export function selectDocument(nodes: DocumentNode[], previousPath: string | null): DocumentNode | undefined {
  return (previousPath === null ? undefined : findDocument(nodes, previousPath)) ?? firstDocument(nodes);
}
