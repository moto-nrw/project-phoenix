import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { checkContext } from './check-agent-context.mjs';

function fixture(t, files) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-context-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  for (const [name, body] of Object.entries(files)) {
    const target = path.join(root, name);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, body);
  }
  return root;
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
  assert.deepEqual(checkContext(root), { documents: 4, errors: [] });
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
  assert.deepEqual(checkContext(root), { documents: 1, errors: [] });
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
