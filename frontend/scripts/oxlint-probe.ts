// Shared harness for the bauart oxlint rule tests: each probe file goes
// through the real oxlint with the repo config.
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

const temporaryDirectories: string[] = [];

type LintResult = { status: number; output: string };
type Probe = { path: string; source: string; resolve: (r: LintResult) => void };

// Each oxlint start costs about 0.2 s, most of it loading the JS plugins.
// Probes queued in the same tick share one oxlint run; the tests below run
// concurrently so their probes batch. Every probe keeps its own temp root,
// so path-based exemptions and baselines see the same relative path.
let queue: Probe[] = [];

function flush() {
  const batch = queue;
  queue = [];
  const result = spawnSync(
    resolve("node_modules/.bin/oxlint"),
    [
      "-c",
      resolve(".oxlintrc.json"),
      "-f",
      "json",
      ...batch.map((p) => p.path),
    ],
    { encoding: "utf8", maxBuffer: 64 * 1024 * 1024 },
  );
  const { diagnostics } = JSON.parse(result.stdout) as {
    diagnostics: {
      code: string;
      message: string;
      severity: string;
      filename: string;
      labels: { span: { line: number } }[];
    }[];
  };
  for (const probe of batch) {
    const own = diagnostics.filter((d) => d.filename === probe.path);
    const lines = probe.source.split("\n");
    // Mirror the default reporter: code, message and the flagged source lines.
    const output = own
      .map((d) =>
        [
          d.code,
          d.message,
          ...d.labels.map((l) => lines[l.span.line - 1] ?? ""),
        ].join("\n"),
      )
      .join("\n");
    const status = own.some((d) => d.severity === "error") ? 1 : 0;
    probe.resolve({ status, output });
  }
}

export function lintSource(
  source: string,
  relativePath = "src/components/probe.tsx",
): Promise<LintResult> {
  const directory = mkdtempSync(join(tmpdir(), "bauart-"));
  temporaryDirectories.push(directory);
  const sourcePath = join(directory, relativePath);
  mkdirSync(dirname(sourcePath), { recursive: true });
  writeFileSync(sourcePath, source);

  return new Promise((done) => {
    if (queue.length === 0) setTimeout(flush, 0);
    queue.push({ path: sourcePath, source, resolve: done });
  });
}

// Concurrent tests share the directory list: call this from afterAll.
export function removeProbeDirectories() {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
}
