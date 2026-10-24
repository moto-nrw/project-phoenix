import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import test from 'node:test';

const migrations = new URL('../backend/database/migrations/', import.meta.url);
const withoutComments = text => text.replace(/\/\*[\s\S]*?\*\/|\/\/[^\n]*|--[^\n]*/g, '');
const source = readdirSync(migrations)
  .filter(name => name.endsWith('.go') && !name.endsWith('_test.go'))
  .map(name => withoutComments(readFileSync(new URL(name, migrations), 'utf8')))
  .join('\n');
const workflow = readFileSync(new URL('../.github/workflows/test.yml', import.meta.url), 'utf8');
const step = workflow.split('      - name: Restore database from snapshot\n')[1]
  ?.split('      - name: Run database migrations\n')[0];
assert.ok(step, 'snapshot restore step must exist');
const sql = step.match(/<<EOF\n([\s\S]*?)\n\s+EOF/)?.[1];
assert.ok(sql, 'snapshot role SQL must be explicit and inspectable');
const normalized = text => text.trim().replace(/\s+/g, ' ').toLowerCase();
const statements = (text, pattern) => [...new Set([...text.matchAll(pattern)].map(match => normalized(match[0])))].sort();

test('snapshot restore creates every migration-owned role, not just roles referenced by pg_dump', () => {
  const names = text => [...new Set([...text.matchAll(/\bCREATE ROLE\s+([a-z_][a-z_0-9]*)\b/gi)].map(match => match[1].toLowerCase()))].sort();
  assert.deepEqual(names(sql), names(source));
  // Dynamic/quoted role names need explicit support rather than silently
  // disappearing from the guard. Migration tests do not define app roles.
  assert.equal([...source.matchAll(/\bCREATE ROLE\b/gi)].length,
    [...source.matchAll(/\bCREATE ROLE\s+[a-z_][a-z_0-9]*\b/gi)].length);
});

test('snapshot restore preserves migration role memberships and role settings', () => {
  const memberships = /\bGRANT\s+[a-z_][a-z_0-9]*\s+TO\s+[a-z_][a-z_0-9]*\b[^;]*;/gi;
  const settings = /\bALTER ROLE\s+[a-z_][a-z_0-9]*\s+SET\s+[^;]+;/gi;
  assert.deepEqual(statements(sql, memberships), statements(source, memberships));
  assert.deepEqual(statements(sql, settings), statements(source, settings));
});
