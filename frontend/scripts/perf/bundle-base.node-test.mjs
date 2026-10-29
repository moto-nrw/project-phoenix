import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const scripts = dirname(fileURLToPath(import.meta.url));

function fixture(run) {
  const directory = mkdtempSync(join(tmpdir(), "moto-bundle-base-test-"));
  try {
    const frontend = join(directory, "frontend");
    mkdirSync(frontend);
    writeFileSync(join(frontend, "source.txt"), "committed base source");
    writeFileSync(join(frontend, ".gitignore"), "perf-results/\n.env.local\n");
    const git = (...args) =>
      execFileSync("git", args, {
        cwd: directory,
        encoding: "utf8",
        stdio: ["ignore", "pipe", "pipe"],
      }).trim();
    git("init");
    git("add", ".");
    git(
      "-c",
      "user.name=Fixture",
      "-c",
      "user.email=fixture@example.invalid",
      "commit",
      "-qm",
      "base",
    );
    const revision = git("rev-parse", "HEAD");
    const bin = join(directory, "bin");
    mkdirSync(bin);
    const log = join(directory, "commands.jsonl");
    writeFileSync(
      join(bin, "pnpm"),
      `#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");
const args = process.argv.slice(2);
fs.appendFileSync(process.env.FIXTURE_LOG, JSON.stringify({ args, cwd: process.cwd(), gh: process.env.GH_TOKEN, github: process.env.GITHUB_TOKEN, source: fs.readFileSync("source.txt", "utf8"), env: fs.readFileSync(".env.local", "utf8") })+"\\n");
if (process.env.FAIL_BASE === "1") process.exit(7);
if (args[0] === "exec") {
 const header = Buffer.from(JSON.stringify({ output_files: [{ filename: "[output]/.next/static/chunks/test.js" }], chunk_parts: [{ output_file_index: 0, source_index: 0, size: 100, compressed_size: 42 }], sources: [{ path: "source.ts" }] }));
 const length = Buffer.alloc(4); length.writeUInt32BE(header.length);
 const data = ".next/diagnostics/analyze/data/parents/children";
 fs.mkdirSync(data, { recursive: true });
 fs.writeFileSync(path.join(data, "analyze.data"), Buffer.concat([length, header]));
}
`,
      { mode: 0o755 },
    );
    const keys = [
      "NEXT_PUBLIC_APP_ENV",
      "API_URL",
      "NEXT_PUBLIC_API_URL",
      "NEXT_PUBLIC_OPERATOR_HOSTNAME",
      "NEXT_PUBLIC_PARENTS_HOSTNAME",
      "NEXT_PUBLIC_SCHOOL_HOSTNAME",
      "NEXT_PUBLIC_CATERER_HOSTNAME",
      "NEXT_PUBLIC_TRAEGER_HOSTNAME",
      "TENANT_DOMAIN",
      "NEXT_PUBLIC_TENANT_DOMAIN",
      "NEXTAUTH_URL",
      "NEXTAUTH_SECRET",
      "AUTH_JWT_EXPIRY",
      "AUTH_JWT_REFRESH_EXPIRY",
      "METRICS_BEARER_TOKEN",
    ];
    const env = {
      ...process.env,
      ...Object.fromEntries(keys.map((key) => [key, "fixture"])),
      PATH: `${bin}:${process.env.PATH}`,
      FIXTURE_LOG: log,
      GH_TOKEN: "fixture",
      GITHUB_TOKEN: "fixture",
    };
    writeFileSync(
      join(frontend, ".env.local"),
      "NEXT_PUBLIC_APP_ENV=fixture\n",
    );
    execFileSync(
      process.execPath,
      [join(scripts, "bundle-measurement.mjs"), "--prepare-ci", revision],
      { cwd: frontend, env },
    );
    run({ directory, frontend, env, log, revision, git });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

test("base build uses committed source, frozen dependencies, same env and parser; cleans temporary sources", () =>
  fixture(({ frontend, env, log, revision }) => {
    const result = spawnSync(
      process.execPath,
      [join(scripts, "bundle-base.mjs")],
      { cwd: frontend, env, encoding: "utf8" },
    );
    assert.equal(result.status, 0, result.stdout + result.stderr);
    const measurement = JSON.parse(
      readFileSync(join(frontend, "perf-results/bundle-base.json"), "utf8"),
    );
    assert.equal(measurement.sourceRevision, revision);
    assert.deepEqual(measurement.routes, { "/parents/children": 42 });
    const calls = readFileSync(log, "utf8").trim().split("\n").map(JSON.parse);
    assert.deepEqual(
      calls.map(({ args }) => args),
      [
        ["install", "--frozen-lockfile"],
        ["exec", "next", "experimental-analyze", "--output"],
      ],
    );
    for (const call of calls) {
      assert.equal(call.source, "committed base source");
      assert.equal(call.env, "NEXT_PUBLIC_APP_ENV=fixture\n");
      assert.equal(call.gh, undefined);
      assert.equal(call.github, undefined);
      assert.equal(existsSync(call.cwd), false);
    }
  }));

test("base build failure is red, publishes no measurement and cleans its temporary sources", () =>
  fixture(({ frontend, env, log }) => {
    const result = spawnSync(
      process.execPath,
      [join(scripts, "bundle-base.mjs")],
      { cwd: frontend, env: { ...env, FAIL_BASE: "1" }, encoding: "utf8" },
    );
    assert.notEqual(result.status, 0);
    assert.equal(
      existsSync(join(frontend, "perf-results/bundle-base.json")),
      false,
    );
    const call = JSON.parse(readFileSync(log, "utf8").trim());
    assert.equal(existsSync(call.cwd), false);
  }));
