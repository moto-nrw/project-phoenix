#!/usr/bin/env node
// Static guidance checks: root/backend/frontend entry points, .claude/.agents/.codex
// Markdown (except worktrees), and docs/agents. Markdown links are doc-relative;
// Inline checks cover repo-root prefixes and explicit ./ or ../ document paths.
// Checks standalone @imports, ATX heading slugs (including duplicates) and HTML ids.
// Skips fences, remote/external paths, pointer-local optional markers and templates.
// Bare inline names may be output templates; use Markdown links to check them.
// Not a full Markdown parser: excludes setext headings and arbitrary code/prose.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const areas = ['', 'backend', 'frontend'];
const ignored = new Set(['worktrees', 'node_modules', '.git']);
const placeholders = /[<>*{}$]|\.{3}|\[[^\]]*\]/;
const rootPrefix = /^(?:\.agents|\.claude|\.codex|backend|frontend|docs|scripts)\//i;

function exactPath(target, links = new Set()) {
  let current = path.parse(target).root;
  for (const part of target.slice(current.length).split(path.sep).filter(Boolean)) {
    let entries;
    try { entries = fs.readdirSync(current); } catch { return `missing target: ${target}`; }
    if (!entries.includes(part)) {
      const match = entries.find(entry => entry.toLowerCase() === part.toLowerCase());
      return match ? `wrong case: ${part} (expected ${match})` : `missing target: ${target}`;
    }
    current = path.join(current, part);
    if (fs.lstatSync(current).isSymbolicLink()) {
      if (links.has(current)) return `broken or cyclic symlink: ${current}`;
      links.add(current);
      const issue = exactPath(path.resolve(path.dirname(current), fs.readlinkSync(current)), links);
      if (issue) return issue;
    }
  }
  try { fs.realpathSync(target); } catch { return `broken or cyclic symlink: ${target}`; }
}

function walk(target, state) {
  if (!fs.existsSync(target) && !fs.lstatSync(target, { throwIfNoEntry: false })) return;
  if (fs.lstatSync(target).isSymbolicLink()) {
    const destination = path.resolve(path.dirname(target), fs.readlinkSync(target));
    const issue = exactPath(destination);
    if (issue) { state.errors.push(`${path.relative(state.root, target)}: ${issue}`); return; }
  }
  const real = fs.realpathSync(target);
  if (!inside(state.root, real) || state.visited.has(real)) return;
  state.visited.add(real);
  if (fs.statSync(target).isDirectory()) {
    for (const name of fs.readdirSync(target).sort()) {
      if (!ignored.has(name)) walk(path.join(target, name), state);
    }
  } else if (target.endsWith('.md')) state.documents.push(real);
}

function inside(root, target) {
  const relative = path.relative(root, target);
  return relative !== '..' && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

function visibleLines(text) {
  let fence;
  return text.split('\n').map(line => {
    const marker = line.match(/^\s*(`{3,}|~{3,})/);
    if (marker) {
      if (!fence) fence = marker[1];
      else if (marker[1][0] === fence[0] && marker[1].length >= fence.length) fence = undefined;
      return '';
    }
    return fence ? '' : line;
  });
}

function pointers(line) {
  const result = [];
  for (const match of line.matchAll(/\]\(\s*(<[^>]+>|[^\s)]+)(?:\s+"[^"]*")?\s*\)/g)) {
    result.push({ value: match[1].replace(/^<|>$/g, ''), end: match.index + match[0].length });
  }
  const definition = line.match(/^\s*\[[^\]]+\]:\s*(\S+)/);
  if (definition) result.push({ value: definition[1].replace(/^<|>$/g, '') });
  const imported = line.match(/^\s*@([^\s]+)\s*$/);
  if (imported) result.push({ value: imported[1] });
  for (const match of line.matchAll(/`([^`\n]+)`/g)) {
    if (/^(?:[\w./-]+\/)?[\w.-]+\.md(?::\d+)?(?:#[\w-]+)?$/.test(match[1])) {
      result.push({ value: match[1], inline: true, end: match.index + match[0].length });
    }
  }
  return result;
}

function pointerTarget(pointer, document, root, line) {
  let value = pointer.value;
  if (/^(?:[a-z][\w+.-]*:|\/|~)/i.test(value) || placeholders.test(value)) return;
  const suffix = pointer.end === undefined ? '' : line.slice(pointer.end);
  if (/^\s*(?:is\s+)?\(?(?:optional\b|if present\b|if (?:it )?exists\b|when present\b)/i.test(suffix)) return;
  value = decodeURIComponent(value.split(/[?#]/)[0]).replace(/:\d+$/, '');
  if (!value) return document;
  if (pointer.inline && !rootPrefix.test(value) && !/^\.\.?\//.test(value)) return;
  let base = path.dirname(document);
  if (pointer.inline && rootPrefix.test(value)) base = root;
  const target = path.resolve(base, value);
  return inside(root, target) ? target : undefined;
}

function headingSlug(text) {
  // Consume code spans first so their underscores stay literal; word-internal
  // underscores are identifiers, not emphasis delimiters.
  text = text.replace(/(`+)(.*?)\1|(?<![\p{L}\p{M}\p{N}_])(__?)(?=\S)(.*?\S)\3(?![\p{L}\p{M}\p{N}_])/gu,
    (_match, ticks, code, _delimiter, content) => ticks ? code : content);
  return text.replace(/<[^>]*>/g, '').replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1')
    .trim().toLowerCase().replace(/[^\p{L}\p{M}\p{N}\s_-]/gu, '').replace(/\s/g, '-');
}

function documentAnchors(document, cache) {
  if (cache.has(document)) return cache.get(document);
  const anchors = new Set();
  const slugs = new Set();
  for (const line of visibleLines(fs.readFileSync(document, 'utf8'))) {
    for (const match of line.matchAll(/<(?:[a-z][\w-]*)\b[^>]*\b(?:id|name)=["']([^"']+)["'][^>]*>/gi)) anchors.add(match[1]);
    const heading = line.match(/^ {0,3}#{1,6}(?:\s+(.*?)\s*|$)$/);
    if (!heading) continue;
    const base = headingSlug((heading[1] ?? '').replace(/\s+#+\s*$/, ''));
    let slug = base;
    for (let suffix = 1; slugs.has(slug); suffix++) slug = `${base}-${suffix}`;
    slugs.add(slug);
    anchors.add(slug);
  }
  cache.set(document, anchors);
  return anchors;
}

function anchorIssue(pointer, target, state) {
  const fragment = pointer.value.split('#').slice(1).join('#');
  if (!fragment || !target.endsWith('.md')) return;
  const anchor = decodeURIComponent(fragment);
  if (!documentAnchors(target, state.anchors).has(anchor)) return `missing anchor: #${anchor}`;
}

function checkDocument(document, state) {
  visibleLines(fs.readFileSync(document, 'utf8')).forEach((line, index) => {
    for (const pointer of pointers(line)) {
      let issue;
      try {
        const target = pointerTarget(pointer, document, state.root, line);
        if (target) issue = exactPath(target) ?? anchorIssue(pointer, target, state);
      } catch (error) { issue = error.message; }
      if (issue) state.errors.push(`${path.relative(state.root, document)}:${index + 1}: ${pointer.value}: ${issue}`);
    }
  });
}

function checkMirrors(root, errors) {
  for (const area of areas) {
    const agents = path.join(root, area, '.agents/skills');
    const claude = path.join(root, area, '.claude/skills');
    if (!fs.existsSync(agents) || !fs.existsSync(claude)) continue;
    for (const name of fs.readdirSync(agents).sort()) {
      const left = path.join(agents, name, 'SKILL.md');
      const right = path.join(claude, name, 'SKILL.md');
      if (!fs.existsSync(left) || !fs.existsSync(right)) continue;
      if (fs.realpathSync(path.dirname(left)) === fs.realpathSync(path.dirname(right))) continue;
      const differs = fs.readFileSync(left, 'utf8') !== fs.readFileSync(right, 'utf8');
      errors.push(`${path.relative(root, right)}: ${differs ? 'divergent' : 'duplicated'} skill; mirror must resolve to one canonical source`);
    }
  }
}

export function checkContext(root) {
  root = fs.realpathSync(root);
  const state = { root, errors: [], documents: [], visited: new Set(), anchors: new Map() };
  for (const area of areas) {
    for (const name of ['AGENTS.md', 'CLAUDE.md', '.agents', '.claude', '.codex']) {
      walk(path.join(root, area, name), state);
    }
  }
  walk(path.join(root, 'docs/agents'), state);
  for (const document of state.documents) checkDocument(document, state);
  checkMirrors(root, state.errors);
  return { documents: state.documents.length, errors: [...new Set(state.errors)].sort() };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const result = checkContext(process.argv[2] ?? process.cwd());
  for (const error of result.errors) console.error(error);
  console.log(`Agent context: ${result.documents} documents, ${result.errors.length} errors.`);
  process.exitCode = result.errors.length ? 1 : 0;
}
