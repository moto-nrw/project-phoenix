#!/usr/bin/env node
// Static guidance checks: root/backend/frontend entry points, .claude/.agents/.codex
// Markdown (except worktrees), and docs/agents. Markdown links are doc-relative;
// Inline checks cover repo-root prefixes and explicit ./ or ../ document paths.
// Checks standalone @imports, ATX heading slugs (including duplicates) and HTML ids.
// Also checks that no two ADR filenames in docs/adr claim the same number.
// Skips fences, remote/external paths, pointer-local optional markers and templates.
// Bare inline names may be output templates; use Markdown links to check them.
// Not a full Markdown parser: excludes setext headings and arbitrary code/prose.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const areas = ['', 'backend', 'frontend'];
const budgetFile = 'scripts/check-agent-context.budget.json';
const codexCap = 32768;
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

// An ADR number is the identifier code and prose cite it by ("ADR 0020"), so
// two files may not share one. Only the numeric prefix is compared, without
// its padding; the slug after it is free. Files without one are not ADRs.
function checkAdrNumbers(root, errors) {
  const directory = path.join(root, 'docs/adr');
  if (!fs.existsSync(directory)) return;
  const taken = new Map();
  for (const name of fs.readdirSync(directory).sort()) {
    const prefix = /^(\d+)-.*\.md$/.exec(name)?.[1];
    if (prefix === undefined) continue;
    const number = Number(prefix);
    if (taken.has(number)) {
      errors.push(`docs/adr/${name}: ADR number ${prefix} already taken by ${taken.get(number)}`);
      continue;
    }
    taken.set(number, name);
  }
}

// Keep logical paths for surface keys, but resolve symlinks for bytes and
// canonical skill identity. Ancestors stop directory cycles without hiding mirrors.
function* contextFiles(target, ancestors = new Set()) {
  if (!fs.existsSync(target)) return;
  const real = fs.realpathSync(target);
  if (ancestors.has(real)) return;
  if (fs.statSync(real).isDirectory()) {
    ancestors.add(real);
    for (const name of fs.readdirSync(target).sort()) {
      if (!ignored.has(name)) yield* contextFiles(path.join(target, name), ancestors);
    }
    ancestors.delete(real);
  } else yield target;
}

// Close enough for the 1024-character cap: joins a plain, quoted or block
// scalar with its indented continuation lines and collapses whitespace.
function skillDescription(frontmatter) {
  const field = frontmatter.match(/^description:[ \t]*(.*)((?:\r?\n[ \t]+.*)*)/m);
  if (!field) return '';
  const text = `${field[1].replace(/^[>|][+-]?\s*$/, '')} ${field[2]}`.replace(/\s+/g, ' ').trim();
  return /^(["']).*\1$/.test(text) ? text.slice(1, -1) : text;
}

function measureContext(root, errors) {
  const sizes = {};
  for (const name of ['CLAUDE.md', 'backend/CLAUDE.md', 'frontend/CLAUDE.md', 'frontend/AGENTS.md']) {
    const target = path.join(root, name);
    if (fs.existsSync(target)) sizes[name] = fs.statSync(fs.realpathSync(target)).size;
  }
  for (const target of contextFiles(path.join(root, '.claude/rules'))) {
    if (target.endsWith('.md')) sizes[path.relative(root, target)] = fs.statSync(fs.realpathSync(target)).size;
  }
  const skills = new Set();
  sizes['skill-catalog'] = 0;
  for (const area of areas) {
    for (const directory of ['.agents/skills', '.claude/skills']) {
      for (const target of contextFiles(path.join(root, area, directory))) {
        if (path.basename(target) !== 'SKILL.md') continue;
        const real = fs.realpathSync(target);
        if (skills.has(real)) continue;
        skills.add(real);
        const frontmatter = fs.readFileSync(real, 'utf8').match(/^---\r?\n([\s\S]*?)\r?\n---[ \t]*(?:\r?\n|$)/);
        if (!frontmatter) continue;
        sizes['skill-catalog'] += Buffer.byteLength(frontmatter[0]);
        const length = [...skillDescription(frontmatter[1])].length;
        if (length > 1024) errors.push(`${path.relative(root, target)}: skill description is ${length} characters; spec limit is 1024 characters`);
      }
    }
  }
  const codex = {};
  const agents = path.join(root, 'AGENTS.md');
  if (fs.existsSync(agents)) {
    const rootBytes = fs.statSync(fs.realpathSync(agents)).size;
    for (const area of areas) {
      const target = path.join(root, area, 'AGENTS.md');
      if (!fs.existsSync(target)) continue;
      const total = rootBytes + (area ? fs.statSync(fs.realpathSync(target)).size : 0);
      codex[area || 'root'] = total;
      if (total > codexCap) errors.push(`${area || 'root'} AGENTS.md chain: ${total} bytes exceeds the ${codexCap}-byte cap; Codex would silently drop the rest`);
    }
  }
  return { sizes, codex };
}

function checkBudgets(root, sizes, errors, updateBudget) {
  const target = path.join(root, budgetFile);
  let budgets = {};
  if (fs.existsSync(target)) {
    try {
      budgets = JSON.parse(fs.readFileSync(target, 'utf8'));
      if (!budgets || Array.isArray(budgets) || typeof budgets !== 'object'
        || Object.values(budgets).some(value => !Number.isSafeInteger(value) || value < 0)) throw new Error();
    } catch {
      errors.push(`${budgetFile}: expected an object of non-negative integer byte budgets`);
      return [];
    }
  } else if (!updateBudget) {
    errors.push(`${budgetFile}: missing budget file; run node scripts/check-agent-context.mjs --update-budget`);
    return [];
  }
  if (updateBudget) {
    const updated = Object.fromEntries(Object.entries(sizes).sort(([a], [b]) => a.localeCompare(b))
      .map(([surface, current]) => [surface, Object.hasOwn(budgets, surface) ? Math.min(current, budgets[surface]) : current]));
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, `${JSON.stringify(updated, null, 2)}\n`);
    budgets = updated;
  }
  const hints = [];
  for (const [surface, current] of Object.entries(sizes)) {
    if (!Object.hasOwn(budgets, surface)) errors.push(`${surface}: new surface has no budget; run node scripts/check-agent-context.mjs --update-budget`);
    else if (current > budgets[surface]) errors.push(`${surface}: current ${current} bytes, budget ${budgets[surface]} bytes, delta +${current - budgets[surface]} bytes; cut it, or raise the budget in this PR with a reason in the PR body`);
    else if (current < budgets[surface]) hints.push(`${surface}: current ${current} bytes is below budget ${budgets[surface]}; run node scripts/check-agent-context.mjs --update-budget to lock in the cut`);
  }
  for (const surface of Object.keys(budgets)) {
    if (!Object.hasOwn(sizes, surface)) errors.push(`${surface}: budgeted surface no longer exists; run node scripts/check-agent-context.mjs --update-budget`);
  }
  return hints;
}

export function checkContext(root, { updateBudget = false } = {}) {
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
  checkAdrNumbers(root, state.errors);
  const { sizes, codex } = measureContext(root, state.errors);
  const hints = checkBudgets(root, sizes, state.errors, updateBudget);
  return { documents: state.documents.length, errors: [...new Set(state.errors)].sort(), sizes, codex, hints };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  const result = checkContext(args.find(arg => arg !== '--update-budget') ?? process.cwd(), { updateBudget: args.includes('--update-budget') });
  for (const error of result.errors) console.error(error);
  for (const hint of result.hints) console.error(`Hint: ${hint}`);
  if (!result.errors.length) {
    const entries = Object.entries(result.sizes).filter(([surface]) => !surface.startsWith('.claude/rules/') && surface !== 'skill-catalog');
    const rules = Object.entries(result.sizes).filter(([surface]) => surface.startsWith('.claude/rules/')).reduce((total, [, bytes]) => total + bytes, 0);
    console.log(`Agent context: ${result.documents} documents, 0 errors; ${entries.map(([surface, bytes]) => `${surface} ${bytes} B`).join(', ')}; rules ${rules} B; skill-catalog ${result.sizes['skill-catalog']} B; Codex ${Object.entries(result.codex).map(([area, bytes]) => `${area} ${bytes}/${codexCap} B`).join(', ')}.`);
  }
  process.exitCode = result.errors.length ? 1 : 0;
}
