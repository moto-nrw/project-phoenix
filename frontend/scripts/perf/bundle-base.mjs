// Build the exact merge-base locally, on the same runner/configuration as head.
// No downloaded artifact, PR credentials, worktree or historical baseline needed.
import { execFileSync } from "node:child_process";
import {
  copyFileSync,
  existsSync,
  mkdtempSync,
  readFileSync,
  rmSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { perfServerEnv } from "./env.mjs";
import { readRouteBundles } from "./bundle-routes.mjs";
import {
  identityForRevision,
  makeMeasurement,
  measurementKey,
  writeMeasurement,
} from "./bundle-measurement.mjs";

export function buildBase(directory = process.cwd()) {
  const requested = JSON.parse(
    readFileSync(
      join(directory, "perf-results/bundle-base-identity.json"),
      "utf8",
    ),
  );
  const current = identityForRevision(requested.sourceRevision, directory);
  if (measurementKey(current.identity) !== measurementKey(requested.identity))
    throw new Error("Base source/configuration changed since preparation");
  const temporary = mkdtempSync(join(tmpdir(), "moto-bundle-base-"));
  try {
    const repository = execFileSync("git", ["rev-parse", "--show-toplevel"], {
      cwd: directory,
      encoding: "utf8",
    }).trim();
    const archive = join(temporary, "source.tar");
    execFileSync(
      "git",
      [
        "archive",
        "--format=tar",
        `--output=${archive}`,
        current.sourceRevision,
        "frontend",
      ],
      { cwd: repository },
    );
    execFileSync("tar", ["-xf", archive, "-C", temporary]);
    const baseDirectory = join(temporary, "frontend");
    // Next reads these files too. Both builds must see exactly the head's env.
    for (const file of [
      ".env",
      ".env.local",
      ".env.production",
      ".env.production.local",
    ])
      if (existsSync(join(directory, file)))
        copyFileSync(join(directory, file), join(baseDirectory, file));
    const env = {
      ...process.env,
      ...perfServerEnv(),
      SKIP_ENV_VALIDATION: "true",
    };
    delete env.GH_TOKEN;
    delete env.GITHUB_TOKEN;
    const options = { cwd: baseDirectory, env, stdio: "inherit" };
    execFileSync("pnpm", ["install", "--frozen-lockfile"], options);
    // The archive has no .next cache. Use the same analyzer and parser as head.
    execFileSync(
      "pnpm",
      ["exec", "next", "experimental-analyze", "--output"],
      options,
    );
    const routes = new Map(
      readRouteBundles(
        join(baseDirectory, ".next/diagnostics/analyze/data"),
      ).map((route) => [route.route, route.compressed]),
    );
    writeMeasurement(
      join(directory, "perf-results/bundle-base.json"),
      makeMeasurement(
        current.identity,
        current.sourceRevision,
        routes,
        process.env.GITHUB_RUN_ID ?? null,
      ),
    );
    process.stdout.write(
      `Cold merge-base measurement completed: ${current.sourceRevision}, ${routes.size} routes.\n`,
    );
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
)
  buildBase();
