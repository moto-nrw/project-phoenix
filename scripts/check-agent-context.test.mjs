import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { checkContext } from './check-agent-context.mjs';
import { fileURLToPath } from 'node:url';

const budgetFile = 'scripts/check-agent-context.budget.json';

function fixture(t, files) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-context-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  for (const [name, body] of Object.entries(files)) {
    const target = path.join(root, name);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, body);
  }
  checkContext(root, { updateBudget: true });
  return root;
}

function budget(root, values) {
  const target = path.join(root, budgetFile);
  if (values) fs.writeFileSync(target, `${JSON.stringify(values, null, 2)}\n`);
  return JSON.parse(fs.readFileSync(target, 'utf8'));
}

function cli(root, ...args) {
  return spawnSync(process.execPath, [fileURLToPath(new URL('./check-agent-context.mjs', import.meta.url)), ...args], {
    cwd: root, encoding: 'utf8',
  });
}

test('accepts doc-relative links, repo-root inline pointers and canonical mirrors', t => {
  const root = fixture(t, {
    'CLAUDE.md': '[Guide](docs/agents/guide.md#heading)\n[ref]: docs/agents/guide.md',
    'docs/agents/guide.md': '# Heading\n`backend/CLAUDE.md`\n[Root](../../CLAUDE.md)',
    'backend/CLAUDE.md': '# Backend',
    '.agents/skills/example/SKILL.md': '# Example',
  });
  fs.mkdirSync(path.join(root, '.claude/skills'), { recursive: true });
  fs.symlinkSync('../../.agents/skills/example', path.join(root, '.claude/skills/example'));
  fs.symlinkSync('CLAUDE.md', path.join(root, 'AGENTS.md'));
  const result = checkContext(root);
  assert.equal(result.documents, 4);
  assert.deepEqual(result.errors, []);
});

test('reports missing links and inline pointers with line numbers', t => {
  const root = fixture(t, { 'CLAUDE.md': '[missing](docs/missing.md)\n`docs/absent.md`' });
  const { errors } = checkContext(root);
  assert.equal(errors.length, 2);
  assert.match(errors[0], /^CLAUDE\.md:1:.*missing target/);
  assert.match(errors[1], /^CLAUDE\.md:2:.*missing target/);
});

test('detects wrong-case paths independently of filesystem case sensitivity', t => {
  const root = fixture(t, {
    'CLAUDE.md': '[Case](.Codex/rules/example.md)\n`.Codex/rules/example.md`',
    '.codex/rules/example.md': '# Example',
  });
  const { errors } = checkContext(root);
  assert.equal(errors.length, 2);
  assert.ok(errors.every(error => error.includes('wrong case: .Codex (expected .codex)')));
});

test('reports broken, cyclic and wrong-case symlinks', t => {
  const root = fixture(t, { 'CLAUDE.md': '# Root', '.agents/skills/real/SKILL.md': '# Real' });
  fs.symlinkSync('missing.md', path.join(root, 'AGENTS.md'));
  fs.symlinkSync('Real', path.join(root, '.agents/skills/wrong-case'));
  fs.symlinkSync('cycle', path.join(root, '.agents/skills/cycle'));
  const { errors } = checkContext(root);
  assert.equal(errors.length, 3);
  assert.ok(errors.some(error => error.includes('missing target')));
  assert.ok(errors.some(error => error.includes('wrong case: Real')));
  assert.ok(errors.some(error => error.includes('cyclic symlink')));
});

test('rejects divergent bodies and identical copies, not just one kind of drift', t => {
  const root = fixture(t, {
    '.agents/skills/different/SKILL.md': 'One',
    '.claude/skills/different/SKILL.md': 'Two',
    'frontend/.agents/skills/same/SKILL.md': 'Same',
    'frontend/.claude/skills/same/SKILL.md': 'Same',
  });
  const { errors } = checkContext(root);
  assert.equal(errors.length, 2);
  assert.match(errors[0], /divergent skill/);
  assert.match(errors[1], /duplicated skill/);
});

test('requires the whole skill directory to share a source, not only SKILL.md', t => {
  const root = fixture(t, {
    '.agents/skills/example/SKILL.md': '# Example',
    '.agents/skills/example/reference.md': 'One',
    '.claude/skills/example/reference.md': 'Two',
  });
  fs.symlinkSync('../../../.agents/skills/example/SKILL.md', path.join(root, '.claude/skills/example/SKILL.md'));
  const { errors } = checkContext(root);
  assert.equal(errors.length, 1);
  assert.match(errors[0], /duplicated skill/);
});

test('ignores examples, remote links, optional and external paths', t => {
  const root = fixture(t, {
    'CLAUDE.md': [
      '# Heading', '```markdown', '[Example](missing.md)', '```',
      '[Remote](https://example.com/file.md) [Anchor](#heading)',
      'Read `CLAUDE.local.md` if present.', '[Optional](missing.md) is optional.',
      '[External](../other-repo/CLAUDE.md)', '`docs/*.md` `docs/<name>.md`',
      'A skill contains `SKILL.md`.',
    ].join('\n'),
    '.claude/worktrees/old/CLAUDE.md': '[Old](missing.md)',
  });
  const result = checkContext(root);
  assert.equal(result.documents, 1);
  assert.deepEqual(result.errors, []);
});

test('validates standalone imports relative to their containing file', t => {
  const root = fixture(t, {
    'CLAUDE.md': '@frontend/AGENTS.md',
    'frontend/AGENTS.md': '# Frontend',
    'frontend/CLAUDE.md': '@AGENTS.md\n@missing.md\n@agents.md',
  });
  const { errors } = checkContext(root);
  assert.equal(errors.length, 2);
  assert.match(errors[0], /^frontend\/CLAUDE\.md:2: missing\.md: missing target/);
  assert.match(errors[1], /^frontend\/CLAUDE\.md:3: agents\.md: wrong case/);
});

test('checks .codex symlinks in root and area directories', t => {
  const root = fixture(t, { '.codex/README.md': '# Codex', 'backend/.codex/README.md': '# Backend' });
  fs.symlinkSync('absent', path.join(root, '.codex/rules'));
  fs.symlinkSync('readme.md', path.join(root, 'backend/.codex/agents'));
  const { errors } = checkContext(root);
  assert.equal(errors.length, 2);
  assert.match(errors[0], /^\.codex\/rules: missing target/);
  assert.match(errors[1], /^backend\/\.codex\/agents: wrong case/);
});

test('checks local ATX, duplicate, Unicode and explicit HTML anchors', t => {
  const root = fixture(t, {
    'CLAUDE.md': [
      '[Guide](docs/agents/guide.md#hello-world)',
      '[Duplicate](docs/agents/guide.md#hello-world-1)',
      '[Unicode](docs/agents/guide.md#gr%C3%BC%C3%9Fe)',
      '[Explicit](docs/agents/guide.md#Stable-ID)',
      '[Legacy](docs/agents/guide.md#legacy)',
      '[Missing](docs/agents/guide.md#old-heading)',
      '[Fenced](docs/agents/guide.md#not-a-heading)',
      '[Self](#local)', '## Local',
    ].join('\n'),
    'docs/agents/guide.md': [
      '# Hello **World**!', '## Hello World', '### Grüße ###',
      '<span id="Stable-ID"></span>', '<a name="legacy"></a>',
      '```markdown', '# Not a heading', '```',
    ].join('\n'),
  });
  const { errors } = checkContext(root);
  assert.equal(errors.length, 2);
  assert.match(errors[0], /^CLAUDE\.md:6:.*missing anchor: #old-heading/);
  assert.match(errors[1], /^CLAUDE\.md:7:.*missing anchor: #not-a-heading/);
});

test('optional qualification applies only to its adjacent pointer', t => {
  const root = fixture(t, {
    'CLAUDE.md': '[Notes](notes.md) is optional; [Required](required.md) is not optional.\n`docs/local.md` (if present); `docs/required.md`',
  });
  const { errors } = checkContext(root);
  assert.equal(errors.length, 2);
  assert.match(errors[0], /^CLAUDE\.md:1: required\.md:/);
  assert.match(errors[1], /^CLAUDE\.md:2: docs\/required\.md:/);
});

test('anchors strip underscore emphasis but retain identifiers and inline code', t => {
  const root = fixture(t, {
    'CLAUDE.md': [
      '[Emphasis](#what-does-not-qualify)', '### What does _not_ qualify',
      '[Strong](#keep-the-boundary)', '### Keep __the boundary__',
      '[Identifier](#tenant_id-and-school_id)', '### tenant_id and school_id',
      '[Code](#keep-_tenant_id_-and-__school_id__)', '### Keep `_tenant_id_` and ``__school_id__``',
      '[Wrong emphasis](#what-does-_not_-qualify)',
      '[Wrong identifier](#tenantid-and-schoolid)',
      '[Wrong code](#keep-tenant_id-and-school_id)',
    ].join('\n'),
  });
  const { errors } = checkContext(root);
  assert.equal(errors.length, 3);
  assert.match(errors[0], /^CLAUDE\.md:10:.*missing anchor: #tenantid-and-schoolid/);
  assert.match(errors[1], /^CLAUDE\.md:11:.*missing anchor: #keep-tenant_id-and-school_id/);
  assert.match(errors[2], /^CLAUDE\.md:9:.*missing anchor: #what-does-_not_-qualify/);
});

test('reports ADRs that claim a number another file already holds', t => {
  const root = fixture(t, {
    'CLAUDE.md': '# Root',
    'docs/adr/0001-first.md': '# First',
    'docs/adr/0002-second.md': '# Second',
    'docs/adr/0002-third.md': '# Third',
    'docs/adr/2-fourth.md': '# Fourth',
    'docs/adr/README.md': '# Index',
  });
  assert.deepEqual(checkContext(root).errors, [
    'docs/adr/0002-third.md: ADR number 0002 already taken by 0002-second.md',
    'docs/adr/2-fourth.md: ADR number 2 already taken by 0002-second.md',
  ]);
});

test('Codex cap counts resolved root and area AGENTS.md bytes, including a real frontend file', t => {
  const root = fixture(t, {
    'CLAUDE.md': 'ä'.repeat(8192),
    'backend/CLAUDE.md': 'b'.repeat(16384),
    'frontend/AGENTS.md': 'f'.repeat(16385),
  });
  fs.symlinkSync('CLAUDE.md', path.join(root, 'AGENTS.md'));
  fs.symlinkSync('CLAUDE.md', path.join(root, 'backend/AGENTS.md'));
  const result = checkContext(root);
  assert.deepEqual(result.codex, { root: 16384, backend: 32768, frontend: 32769 });
  assert.deepEqual(result.errors, [
    'frontend AGENTS.md chain: 32769 bytes exceeds the 32768-byte cap; Codex would silently drop the rest',
  ]);
  fs.appendFileSync(path.join(root, 'backend/CLAUDE.md'), 'b');
  assert.ok(checkContext(root, { updateBudget: true }).errors.some(error => /^backend AGENTS.md chain: 32769 bytes.*32768.*Codex would silently drop the rest/.test(error)));
});

test('Codex also enforces the root-only cap', t => {
  const root = fixture(t, { 'CLAUDE.md': 'x'.repeat(32769) });
  fs.symlinkSync('CLAUDE.md', path.join(root, 'AGENTS.md'));
  assert.match(checkContext(root).errors[0], /^root AGENTS.md chain: 32769 bytes.*32768.*Codex would silently drop the rest/);
});

test('skill descriptions enforce 1024 characters for plain, quoted and block scalars', t => {
  const descriptions = [
    'x'.repeat(1025),
    `"${'x'.repeat(1025)}"`,
    `'${'x'.repeat(1025)}'`,
    `>-\n  ${'x'.repeat(512)}\n  ${'x'.repeat(512)}`,
    `|-\n  ${'x'.repeat(512)}\n  ${'x'.repeat(512)}`,
    `${'x'.repeat(512)}\n  ${'x'.repeat(512)}`,
  ];
  for (const [index, description] of descriptions.entries()) {
    const root = fixture(t, { [`.agents/skills/example-${index}/SKILL.md`]: `---\nname: example\ndescription: ${description}\n---\n# Skill` });
    assert.match(checkContext(root).errors[0], /skill description is 1025 characters; spec limit is 1024 characters/);
  }
  const root = fixture(t, { '.agents/skills/example/SKILL.md': `---\nname: example\ndescription: "${'😀'.repeat(1024)}"\n---\n${'body'.repeat(1024)}` });
  assert.deepEqual(checkContext(root).errors, []);
});

test('over-budget surfaces report current, budget, delta and the review instruction', t => {
  const root = fixture(t, { 'CLAUDE.md': 'abc' });
  budget(root, { ...budget(root), 'CLAUDE.md': 3 });
  fs.appendFileSync(path.join(root, 'CLAUDE.md'), 'ä');
  assert.deepEqual(checkContext(root).errors, [
    'CLAUDE.md: current 5 bytes, budget 3 bytes, delta +2 bytes; cut it, or raise the budget in this PR with a reason in the PR body',
  ]);
});

test('new nested rule surfaces need a budget and symlink bytes come from the target', t => {
  const root = fixture(t, { 'reference.md': 'ä' });
  fs.mkdirSync(path.join(root, '.claude/rules/security'), { recursive: true });
  fs.symlinkSync('../../../reference.md', path.join(root, '.claude/rules/security/new.md'));
  const result = checkContext(root);
  assert.equal(result.sizes['.claude/rules/security/new.md'], 2);
  assert.deepEqual(result.errors, [
    '.claude/rules/security/new.md: new surface has no budget; run node scripts/check-agent-context.mjs --update-budget',
  ]);
});

test('deleted budgeted surfaces fail, while unused headroom only hints', t => {
  const root = fixture(t, { 'CLAUDE.md': 'abc', '.claude/rules/old.md': '# Old' });
  fs.rmSync(path.join(root, '.claude/rules/old.md'));
  fs.writeFileSync(path.join(root, 'CLAUDE.md'), 'a');
  const result = checkContext(root);
  assert.deepEqual(result.errors, [
    '.claude/rules/old.md: budgeted surface no longer exists; run node scripts/check-agent-context.mjs --update-budget',
  ]);
  assert.match(result.hints[0], /CLAUDE.md: current 1 bytes leaves budget 259 more than the slack.*--update-budget/);
});

test('--update-budget sets size plus slack, lowers, adds and removes entries but never raises an existing budget', t => {
  const root = fixture(t, { 'CLAUDE.md': 'abc', '.claude/rules/old.md': '# Old' });
  fs.writeFileSync(path.join(root, 'CLAUDE.md'), 'a');
  fs.rmSync(path.join(root, '.claude/rules/old.md'));
  fs.writeFileSync(path.join(root, '.claude/rules/new.md'), '# New');
  assert.equal(cli(root, '--update-budget').status, 0);
  assert.deepEqual(budget(root), { '.claude/rules/new.md': 261, 'CLAUDE.md': 257, 'skill-catalog': 256 });
  const saved = fs.readFileSync(path.join(root, budgetFile), 'utf8');
  fs.writeFileSync(path.join(root, 'CLAUDE.md'), 'ab');
  assert.deepEqual(checkContext(root).errors, [], 'a small edit fits the slack');
  fs.writeFileSync(path.join(root, 'CLAUDE.md'), 'a'.repeat(258));
  const result = cli(root, '--update-budget');
  assert.equal(result.status, 1);
  assert.match(result.stderr, /CLAUDE.md: current 258 bytes, budget 257 bytes, delta \+1 bytes/);
  assert.equal(fs.readFileSync(path.join(root, budgetFile), 'utf8'), saved);
  assert.equal(cli(root, '--update-budget', root).status, 1);
});

test('missing or malformed budgets fail instead of silently disabling the gate', t => {
  const root = fixture(t, { 'CLAUDE.md': '# Root' });
  fs.rmSync(path.join(root, budgetFile));
  assert.match(checkContext(root).errors[0], /missing budget file/);
  assert.deepEqual(checkContext(root, { updateBudget: true }).errors, []);
  for (const invalid of ['{', 'null', '[]', '{"CLAUDE.md": -1}', '{"CLAUDE.md": "100"}']) {
    fs.writeFileSync(path.join(root, budgetFile), invalid);
    assert.match(checkContext(root, { updateBudget: true }).errors[0], /expected an object of non-negative integer byte budgets/);
    assert.equal(fs.readFileSync(path.join(root, budgetFile), 'utf8'), invalid);
  }
});

test('catalog counts canonical frontmatter bytes once across all root and area skill directories', t => {
  const frontmatter = '---\nname: example\ndescription: Grüße\n---\n';
  const files = {};
  for (const area of ['', 'backend/', 'frontend/']) {
    for (const directory of ['.agents/skills', '.claude/skills']) {
      files[`${area}${directory}/${area.replace('/', '') || 'root'}-${directory.slice(1, 7)}/SKILL.md`] = `${frontmatter}# Body`;
    }
  }
  const root = fixture(t, files);
  fs.symlinkSync('../../.agents/skills/root-agents', path.join(root, '.claude/skills/mirror'));
  const result = checkContext(root);
  assert.equal(result.sizes['skill-catalog'], Buffer.byteLength(frontmatter) * 6);
  assert.deepEqual(result.errors, []);
  fs.appendFileSync(path.join(root, '.agents/skills/root-agents/SKILL.md'), '\nExtra body text');
  assert.equal(checkContext(root).sizes['skill-catalog'], result.sizes['skill-catalog']);
  budget(root, { ...budget(root), 'skill-catalog': result.sizes['skill-catalog'] - 1 });
  assert.match(checkContext(root).errors[0], /skill-catalog: current .*delta \+1 bytes/);
});
