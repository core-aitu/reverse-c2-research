import test from 'node:test';
import assert from 'node:assert/strict';
import { buildDocumentationTree, findDocument, firstDocument, selectDocument } from '../src/documentationModel.ts';

const entry = (name, contents = '', children = []) => ({ name, contents, children });

test('flat API paths share a folder, preserving their contents', () => {
  const tree = buildDocumentationTree([entry('Guide/A.md', 'A'), entry('Guide/B.md', 'B'), entry('Overview.md', 'O')]);
  assert.equal(tree.length, 2);
  assert.equal(tree[0].contents, null);
  assert.deepEqual(tree[0].children.map(n => n.name), ['A.md', 'B.md']);
  assert.equal(findDocument(tree, 'Guide/B.md').contents, 'B');
  assert.equal(findDocument(tree, 'Guide'), undefined);
});

test('nested legacy children merge with flat paths without duplicate nodes', () => {
  const tree = buildDocumentationTree([entry('Guide', '', [entry('Nested', '', [entry('A.md', 'A')])]), entry('Guide/Nested/B.md', 'B')]);
  assert.equal(tree[0].children.length, 1);
  assert.equal(findDocument(tree, 'Guide/Nested/A.md').contents, 'A');
  assert.equal(findDocument(tree, 'Guide/Nested/B.md').contents, 'B');
  assert.equal(firstDocument(tree).path, 'Guide/Nested/A.md');
  assert.equal(findDocument(tree, 'Guide'), undefined);
});

test('selection is stable across reload, with a fallback for deleted documents', () => {
  const tree = buildDocumentationTree([entry('A.md', 'A'), entry('B.md', 'new B')]);
  assert.equal(selectDocument(tree, 'B.md').contents, 'new B');
  assert.equal(selectDocument(tree, 'missing.md').path, 'A.md');
  assert.equal(selectDocument([], 'B.md'), undefined);
});

test('empty documents remain valid and empty catalog has no selection', () => {
  const tree = buildDocumentationTree([entry('Empty.md')]);
  assert.equal(firstDocument(tree).contents, '');
  assert.equal(firstDocument([]), undefined);
  assert.deepEqual(buildDocumentationTree([]), []);
});

test('folder entries with their own contents remain selectable', () => {
  const tree = buildDocumentationTree([entry('Guide', 'Intro', [entry('Page.md', 'Body')])]);
  assert.equal(firstDocument(tree).path, 'Guide');
  assert.equal(findDocument(tree, 'Guide/Page.md').contents, 'Body');
});
