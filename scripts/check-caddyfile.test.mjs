import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  checkReverseProxies, defaultCaddyfile, diffRanges, formatRangeDiff, main, parseRangeList, trustedProxies,
} from './check-caddyfile.mjs';

const production = readFileSync(defaultCaddyfile, 'utf8');
const snippet = '(client_ip_upstream) {\n\theader_up X-Forwarded-For {client_ip}\n}\n';

test('the versioned Caddyfile passes the reverse_proxy check', () => {
  assert.deepEqual(checkReverseProxies(production), []);
});

test('flags a reverse_proxy without the import, also next to braces in quoted strings', () => {
  const source = `${snippet}(page) {\n\trespond \`body{margin:0}\` 200\n}\na.example {\n\treverse_proxy localhost:1 {\n\t\timport client_ip_upstream\n\t}\n}\nb.example {\n\treverse_proxy localhost:2 {\n\t\theader_up Host {host}\n\t}\n\timport client_ip_upstream\n}\n`;
  const errors = checkReverseProxies(source);
  assert.equal(errors.length, 1);
  assert.match(errors[0], /line 13: reverse_proxy localhost:2/);
});

test('flags a reverse_proxy without a block and a missing snippet', () => {
  const errors = checkReverseProxies('a.example {\n\treverse_proxy localhost:1\n}\n');
  assert.match(errors[0], /has no block/);
  assert.match(errors[1], /snippet is missing/);
});

test('a commented-out import does not count', () => {
  const errors = checkReverseProxies(`${snippet}a.example {\n\treverse_proxy localhost:1 {\n\t\t# import client_ip_upstream\n\t}\n}\n`);
  assert.match(errors[0], /does not import/);
});

test('reads the trusted_proxies ranges of the versioned Caddyfile', () => {
  const ranges = trustedProxies(production);
  assert.ok(ranges.includes('173.245.48.0/20'));
  assert.ok(ranges.includes('2c0f:f248::/32'));
  assert.throws(() => trustedProxies(snippet), /No "trusted_proxies static"/);
});

test('rejects a Cloudflare response that is not a CIDR list', () => {
  assert.deepEqual(parseRangeList('1.2.3.0/24\n2400:cb00::/32\n', 'u'), ['1.2.3.0/24', '2400:cb00::/32']);
  assert.throws(() => parseRangeList('<html>blocked</html>', 'u'), /did not return a CIDR list/);
  assert.throws(() => parseRangeList('', 'u'), /empty/);
});

test('reports added and dropped ranges with the corrected line', () => {
  const diff = diffRanges(['1.0.0.0/24', '2.0.0.0/24'], ['2.0.0.0/24', '3.0.0.0/24']);
  assert.deepEqual(diff, { missing: ['3.0.0.0/24'], extra: ['1.0.0.0/24'] });
  const text = formatRangeDiff(diff, ['2.0.0.0/24', '3.0.0.0/24'], 'Caddyfile');
  assert.match(text, /^\+ 3\.0\.0\.0\/24 /m);
  assert.match(text, /^- 1\.0\.0\.0\/24 /m);
  assert.match(text, /trusted_proxies static 2\.0\.0\.0\/24 3\.0\.0\.0\/24$/);
  assert.equal(formatRangeDiff(diffRanges(['A::/32'], ['a::/32']), ['a::/32'], 'f'), '');
});

function fakeFetch(v4, v6) {
  return async url => new Response(url.endsWith('v4') ? v4 : v6);
}

test('--cloudflare passes when the published ranges match and fails with a diff otherwise', async () => {
  const ranges = trustedProxies(production);
  const v4 = ranges.filter(range => !range.includes(':')).join('\n');
  const v6 = ranges.filter(range => range.includes(':')).join('\n');
  const output = [];
  const sink = { log: line => output.push(line), error: line => output.push(line) };

  assert.equal(await main(['--cloudflare'], { fetchImpl: fakeFetch(v4, v6), ...sink }), 0);
  assert.match(output.join('\n'), /match Cloudflare's 22 published ranges/);

  output.length = 0;
  assert.equal(await main(['--cloudflare'], { fetchImpl: fakeFetch(`${v4}\n198.51.100.0/24`, v6), ...sink }), 1);
  assert.match(output.join('\n'), /^\+ 198\.51\.100\.0\/24 /m);
});
