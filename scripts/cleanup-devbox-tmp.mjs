#!/usr/bin/env node
// NixOS/nix#10404 leaves profile trees after successful installs. Only our
// private namespace is eligible; caller-supplied TMPDIRs are never swept.
import { existsSync, lstatSync, readFileSync, readdirSync, readlinkSync, realpathSync, rmSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const common = spawnSync('git', ['rev-parse', '--git-common-dir'], { cwd: repo, encoding: 'utf8' });
if (common.status !== 0) throw new Error('Cannot locate repository temp namespace');
const namespaceRoot = join(realpathSync(resolve(repo, common.stdout.trim())), 'moto-nix-tmp');
const uid = process.getuid();
const inside = (path, directory) => path === directory || path.startsWith(`${directory}/`);
const alive = pid => {
  try { process.kill(pid, 0); return true; } catch (error) { return error.code !== 'ESRCH'; }
};

function references(root) {
  if (process.platform === 'linux') {
    const paths = [];
    let complete = true;
    const namespaces = new Set();
    for (const pid of readdirSync('/proc').filter(name => /^\d+$/.test(name))) {
      const proc = `/proc/${pid}`;
      try {
        // Inspect namespace identity before user filtering or cwd inspection:
        // an inaccessible cwd must not hide a namespace we can identify.
        try {
          const namespace = /^pid:\[([0-9]+)\]$/.exec(readlinkSync(`${proc}/ns/pid`));
          if (namespace) namespaces.add(namespace[1]);
        } catch (error) {
          if (error.code !== 'ENOENT' && error.code !== 'ESRCH') complete = false;
        }
        if (lstatSync(proc).uid !== uid) continue;
        paths.push(readlinkSync(`${proc}/cwd`));
        for (const fd of readdirSync(`${proc}/fd`)) {
          try {
            const reference = readlinkSync(`${proc}/fd/${fd}`);
            paths.push(reference);
            const namespace = /^pid:\[([0-9]+)\]$/.exec(reference);
            if (namespace) namespaces.add(namespace[1]);
          }
          catch (error) { if (error.code !== 'ENOENT') throw error; }
        }
        // A mapped file may remain referenced after its descriptor closes.
        paths.push(...readFileSync(`${proc}/maps`, 'utf8').split('\n').map(line => line.slice(line.indexOf('/')).replace(/\\([0-7]{3})/g, (_, octal) => String.fromCharCode(parseInt(octal, 8)))).filter(path => path.startsWith('/')));
      } catch (error) {
        if (error.code !== 'ENOENT' && error.code !== 'ESRCH') complete = false;
      }
    }
    return { paths, complete, namespaces };
  }
  if (process.platform === 'darwin') {
    // macOS supplies lsof. Any diagnostic/unsupported platform fails closed.
    const result = spawnSync('/usr/sbin/lsof', ['-nP', '-a', '-u', String(uid), '+D', root, '-Fn'], { encoding: 'utf8' });
    return {
      complete: !result.error && !result.stderr && [0, 1].includes(result.status),
      paths: (result.stdout || '').split('\n').filter(line => line.startsWith('n')).map(line => line.slice(1)),
    };
  }
  throw new Error('Unsupported process reference inspection');
}

// Root mtime alone misses changes inside nested directories. Never follow
// symlinks: Nix profiles contain links into the persistent Nix store.
export function unchangedForHour(path, now) {
  const stat = lstatSync(path);
  if (Math.max(stat.mtimeMs, stat.ctimeMs) >= now - 60 * 60 * 1000) return false;
  return !stat.isDirectory() || readdirSync(path).every(name => unchangedForHour(join(path, name), now));
}

function privateDirectory(path) {
  const stat = lstatSync(path);
  return stat.isDirectory() && stat.uid === uid && (stat.mode & 0o777) === 0o700 && realpathSync(path) === path;
}

function cleanup() {
  let root = namespaceRoot;
  if (process.platform === 'linux') {
    const namespace = /^pid:\[([0-9]+)\]$/.exec(readlinkSync('/proc/self/ns/pid'));
    if (!namespace) return;
    root = join(root, `pid-${namespace[1]}`);
  }
  if (resolve(process.env.TMPDIR || '/tmp') !== root) return;
  if (!privateDirectory(namespaceRoot) || !privateDirectory(root)) return;
  const candidates = readdirSync(root).flatMap(name => {
    const match = /^nix-([1-9]\d*)-\d+$/.exec(name);
    if (!match || alive(Number(match[1]))) return [];
    const path = join(root, name);
    const stat = lstatSync(path);
    return stat.isDirectory() && stat.uid === uid ? [{ path, pid: Number(match[1]), stat }] : [];
  });
  const abandoned = process.platform === 'linux' ? readdirSync(namespaceRoot).flatMap(name => {
    const match = /^pid-([0-9]+)$/.exec(name);
    const path = join(namespaceRoot, name);
    if (!match || path === root || !privateDirectory(path)) return [];
    return [{ path, namespace: match[1], stat: lstatSync(path) }];
  }) : [];
  if (!candidates.length && !abandoned.length) return;
  const { paths: open, complete, namespaces } = references(namespaceRoot);
  const now = Date.now();
  for (const { path, pid, stat } of candidates) {
    if (open.some(reference => inside(reference.replace(/ \(deleted\)$/, ''), path))) continue;
    // ptrace_scope can hide unrelated same-user processes. Retain uncertain
    // trees, except the explicitly bounded Linux fallback within the same PID
    // namespace: the creator is absent and every entry is unchanged for an hour.
    if (!complete && !(process.platform === 'linux' && !existsSync(`/proc/${pid}`) && unchangedForHour(path, now))) continue;
    try {
      const current = lstatSync(path);
      if (!alive(pid) && current.ino === stat.ino && current.dev === stat.dev && current.isDirectory() && current.uid === uid) {
        rmSync(path, { recursive: true });
      }
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
  for (const { path, namespace, stat } of abandoned) {
    // No PID in this namespace can be interpreted using our own /proc view.
    // The explicitly permitted fallback also covers an old, uninspectable
    // namespace, but known namespaces and open references always win.
    if (namespaces.has(namespace) || open.some(reference => inside(reference.replace(/ \(deleted\)$/, ''), path))) continue;
    try {
      if (!unchangedForHour(path, now)) continue;
      const current = lstatSync(path);
      if (current.ino === stat.ino && current.dev === stat.dev && privateDirectory(path)) rmSync(path, { recursive: true });
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
}

if (resolve(process.argv[1] || '') === fileURLToPath(import.meta.url)) {
try { cleanup(); }
catch (error) {
  // A process may exit or a concurrent bootstrap may clean first. When process
  // inspection is denied, use only the documented one-hour fallback.
  if (error.code !== 'ENOENT') console.error(`Devbox temp cleanup skipped: ${error.message}`);
}
}
