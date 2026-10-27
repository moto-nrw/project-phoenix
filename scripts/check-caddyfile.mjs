#!/usr/bin/env node
// Checks the production Caddyfile (deploy/caddy/Caddyfile, #5091). Every
// rate limit keys on the client IP that Caddy derives from it (#5081).
//
//   node scripts/check-caddyfile.mjs [file]
//     Every reverse_proxy imports the client_ip_upstream snippet, so the
//     upstream sees exactly one X-Forwarded-For value. Runs in CI.
//   node scripts/check-caddyfile.mjs --cloudflare [file]
//     Additionally compares trusted_proxies with Cloudflare's published
//     ranges and prints the difference plus the corrected line. A new edge
//     range missing here makes Caddy log Cloudflare IPs as client IPs.
//     Runs weekly in caddy-cloudflare-ranges.yml.
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

export const defaultCaddyfile = fileURLToPath(new URL('../deploy/caddy/Caddyfile', import.meta.url));
export const cloudflareLists = ['https://www.cloudflare.com/ips-v4', 'https://www.cloudflare.com/ips-v6'];
const cidr = /^(?:\d{1,3}(?:\.\d{1,3}){3}\/\d{1,2}|[0-9a-f:]+:[0-9a-f:]*\/\d{1,3})$/i;

// Quoted strings (the maintenance page holds CSS braces) and comments carry
// no Caddyfile structure.
function structural(line) {
  return line.replace(/`[^`]*`|"(?:[^"\\]|\\.)*"/g, '""').replace(/(^|\s)#.*$/, '$1').trim();
}

export function checkReverseProxies(source) {
  const lines = source.split(/\r?\n/).map(structural);
  const errors = [];
  let count = 0;
  lines.forEach((line, index) => {
    if (!/^reverse_proxy(\s|$)/.test(line)) return;
    count++;
    const where = `line ${index + 1}: ${line}`;
    if (!line.endsWith('{')) {
      errors.push(`${where} has no block; it must contain "import client_ip_upstream".`);
      return;
    }
    let depth = 0;
    let imported = false;
    for (let i = index; i < lines.length; i++) {
      if (i > index && depth === 1 && lines[i] === 'import client_ip_upstream') imported = true;
      for (const char of lines[i]) depth += char === '{' ? 1 : char === '}' ? -1 : 0;
      if (depth <= 0) break;
    }
    if (!imported) errors.push(`${where} does not import client_ip_upstream.`);
  });
  if (count === 0) errors.push('No reverse_proxy found; is this the right Caddyfile?');
  if (!/^\(client_ip_upstream\)\s*\{/m.test(source)) errors.push('The (client_ip_upstream) snippet is missing.');
  return errors;
}

export function trustedProxies(source) {
  const line = source.split(/\r?\n/).map(structural).find(l => /^trusted_proxies\s+static(\s|$)/.test(l));
  if (!line) throw new Error('No "trusted_proxies static" line found.');
  return line.split(/\s+/).slice(2);
}

export function parseRangeList(text, url) {
  const ranges = text.split(/\s+/).filter(Boolean);
  const invalid = ranges.filter(range => !cidr.test(range));
  if (ranges.length === 0 || invalid.length) {
    throw new Error(`${url} did not return a CIDR list (${invalid.length ? `got "${invalid[0].slice(0, 60)}"` : 'empty'}).`);
  }
  return ranges;
}

export function diffRanges(configured, published) {
  const norm = range => range.toLowerCase();
  const have = new Set(configured.map(norm));
  const want = new Set(published.map(norm));
  return {
    missing: published.filter(range => !have.has(norm(range))),
    extra: configured.filter(range => !want.has(norm(range))),
  };
}

export function formatRangeDiff({ missing, extra }, published, file) {
  if (!missing.length && !extra.length) return '';
  return [
    `trusted_proxies in ${file} differ from https://www.cloudflare.com/ips/:`,
    ...missing.map(range => `+ ${range}   (published by Cloudflare, missing in the Caddyfile)`),
    ...extra.map(range => `- ${range}   (in the Caddyfile, no longer published)`),
    '',
    'Corrected line:',
    `\t\ttrusted_proxies static ${published.join(' ')}`,
  ].join('\n');
}

async function fetchPublished(fetchImpl = fetch) {
  const lists = await Promise.all(cloudflareLists.map(async url => {
    const response = await fetchImpl(url, { signal: AbortSignal.timeout(20_000) });
    if (!response.ok) throw new Error(`${url} answered HTTP ${response.status}.`);
    return parseRangeList(await response.text(), url);
  }));
  return lists.flat();
}

export async function main(args, { fetchImpl, log = console.log, error = console.error } = {}) {
  const compare = args.includes('--cloudflare');
  const file = args.find(arg => !arg.startsWith('--')) ?? defaultCaddyfile;
  const source = readFileSync(file, 'utf8');
  const errors = checkReverseProxies(source);
  for (const message of errors) error(message);
  if (compare) {
    const published = await fetchPublished(fetchImpl);
    const diff = formatRangeDiff(diffRanges(trustedProxies(source), published), published, file);
    if (diff) {
      error(diff);
      errors.push(diff);
    } else {
      log(`trusted_proxies match Cloudflare's ${published.length} published ranges.`);
    }
  }
  if (!errors.length) log(`${file}: every reverse_proxy imports client_ip_upstream.`);
  return errors.length ? 1 : 0;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2)).then(code => { process.exitCode = code; }, err => {
    console.error(err.message);
    process.exitCode = 2;
  });
}
