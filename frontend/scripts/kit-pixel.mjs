#!/usr/bin/env node
// Pixel-Vergleich der Kit-Stories (#6562), Desktop und Telefon.
//
//   pnpm run kit-pixel baseline [--ref <commit>] [--filter <regex>] [--all-stories]
//   pnpm run kit-pixel compare  [--filter <regex>] [--all-stories]
//
// baseline exportiert den Commit (Standard HEAD, ohne uncommittete Änderungen),
// baut daraus Storybook und schreibt je Story und Breite ein PNG nach
// .kit-pixel/baseline/. compare baut Storybook aus dem Arbeitsstand und
// vergleicht Pixel für Pixel. Anleitung: docs/agents/frontend-testing.md.
import { spawnSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  realpathSync,
  renameSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import {
  orphanedBaselines,
  parseKitPixelArgs,
  selectStories,
} from "./kit-pixel-lib.mjs";

const frontendDir = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);
const workDir = path.join(frontendDir, ".kit-pixel");
const baselineDir = path.join(workDir, "baseline");
// Neue Baseline entsteht hier und ersetzt die alte erst, wenn alle Bilder da sind.
const nextBaselineDir = path.join(workDir, "baseline-next");
const metaFile = path.join(baselineDir, "meta.json");

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    stdio: "inherit",
    ...options,
    env: { ...process.env, ...options.env },
  });
  if (result.error) throw result.error;
  return result.status ?? 1;
}

/** Fehler im Kindprozess brechen den Befehl ab; `finally` räumt trotzdem auf. */
class StepFailed extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
  }
}

function runChecked(command, args, options) {
  const status = run(command, args, options);
  if (status !== 0) {
    throw new StepFailed(
      `Abgebrochen: ${command} ${args.join(" ")} (Exit ${status})`,
      status,
    );
  }
}

function git(args, options = {}) {
  const result = spawnSync("git", args, {
    cwd: frontendDir,
    encoding: "utf8",
    maxBuffer: 64 * 1024 * 1024,
    ...options,
  });
  if (result.status !== 0) {
    throw new Error(`git ${args.join(" ")}: ${result.stderr.trim()}`);
  }
  return result.stdout;
}

const repoRoot = git(["rev-parse", "--show-toplevel"]).trim();
/** Pfad des Frontends im Repository, ohne Schrägstrich am Ende („frontend“). */
const frontendPrefix = path
  .relative(realpathSync(repoRoot), realpathSync(frontendDir))
  .split(path.sep)
  .join("/");

function buildStorybook(cwd, outputDir) {
  rmSync(outputDir, { recursive: true, force: true });
  console.log(`\nStorybook bauen: ${path.relative(frontendDir, cwd) || "."}`);
  runChecked(
    process.execPath,
    ["scripts/build-storybook.mjs", "--output-dir", outputDir, "--quiet"],
    { cwd, env: { STORYBOOK_DISABLE_TELEMETRY: "1" } },
  );
}

/** Frontend-Stand von `sha` nach .kit-pixel/source/frontend, mit Abhängigkeiten. */
function exportFrontend(sha) {
  const sourceDir = path.join(workDir, "source");
  rmSync(sourceDir, { recursive: true, force: true });
  mkdirSync(sourceDir, { recursive: true });
  const archive = path.join(workDir, "source.tar");
  // git archive statt Worktree: registriert nichts im gemeinsamen Repository.
  // `<sha>:frontend` ist der Baum des Frontend-Ordners; das Archiv enthält
  // also `package.json` usw. ohne Präfix. Vom Repo-Stamm aus aufgerufen, weil
  // git archive aus einem Unterordner nur dessen Teilbaum packt.
  git(["archive", "--format=tar", "-o", archive, `${sha}:${frontendPrefix}`], {
    cwd: repoRoot,
  });
  const exportedFrontend = path.join(sourceDir, "frontend");
  mkdirSync(exportedFrontend);
  runChecked("tar", ["-xf", archive, "-C", exportedFrontend]);
  rmSync(archive);

  const lockAtRef = git(["show", `${sha}:${frontendPrefix}/pnpm-lock.yaml`]);
  const lockHere = readFileSync(
    path.join(frontendDir, "pnpm-lock.yaml"),
    "utf8",
  );
  if (lockAtRef === lockHere) {
    symlinkSync(
      path.join(frontendDir, "node_modules"),
      path.join(exportedFrontend, "node_modules"),
      "dir",
    );
  } else {
    console.log(
      "\nAndere pnpm-lock.yaml im Basis-Commit: Abhängigkeiten installieren.",
    );
    runChecked("pnpm", ["install", "--frozen-lockfile", "--prefer-offline"], {
      cwd: exportedFrontend,
    });
  }
  return exportedFrontend;
}

function runPlaywright({
  mode,
  storybookDir,
  snapshotDir,
  filter,
  allStories,
}) {
  return run(
    "pnpm",
    [
      "exec",
      "playwright",
      "test",
      "-c",
      "playwright.kit-pixel.config.ts",
      `--update-snapshots=${mode === "baseline" ? "all" : "none"}`,
    ],
    {
      cwd: frontendDir,
      env: {
        KIT_PIXEL_MODE: mode,
        KIT_PIXEL_BASELINE_DIR: snapshotDir,
        KIT_PIXEL_WORK_DIR: workDir,
        KIT_PIXEL_STORYBOOK_DIR: storybookDir,
        KIT_PIXEL_FILTER: filter ?? "",
        KIT_PIXEL_ALL_STORIES: allStories ? "1" : "",
      },
    },
  );
}

function baseline({ ref, filter, allStories }) {
  const sha = git(["rev-parse", "--verify", `${ref}^{commit}`]).trim();
  console.log(`Baseline aus ${ref} (${sha.slice(0, 10)})`);
  const storybookDir = path.join(workDir, "storybook-baseline");
  const sourceDir = path.join(workDir, "source");
  try {
    buildStorybook(exportFrontend(sha), storybookDir);
  } finally {
    rmSync(sourceDir, { recursive: true, force: true });
    rmSync(path.join(workDir, "source.tar"), { force: true });
  }

  rmSync(nextBaselineDir, { recursive: true, force: true });
  mkdirSync(nextBaselineDir, { recursive: true });
  const status = runPlaywright({
    mode: "baseline",
    storybookDir,
    snapshotDir: nextBaselineDir,
    filter,
    allStories,
  });
  if (status !== 0) {
    rmSync(nextBaselineDir, { recursive: true, force: true });
    throw new StepFailed(
      "Baseline unvollständig: Mindestens eine Story ließ sich nicht aufnehmen (siehe oben). Die bisherige Baseline bleibt.",
      status,
    );
  }
  writeFileSync(
    path.join(nextBaselineDir, "meta.json"),
    `${JSON.stringify({ ref, sha, filter: filter ?? null, allStories }, null, 2)}\n`,
  );
  rmSync(baselineDir, { recursive: true, force: true });
  renameSync(nextBaselineDir, baselineDir);
  console.log(
    `\nBaseline fertig: ${path.relative(frontendDir, baselineDir)}. Weiter mit: pnpm run kit-pixel compare`,
  );
}

/** Bilder der Baseline, deren Story im Build fehlt (gleiche Auswahl vorausgesetzt). */
function findOrphans(storybookDir, selection) {
  const storyIds = selectStories(
    JSON.parse(readFileSync(path.join(storybookDir, "index.json"), "utf8")),
    selection,
  ).map((story) => story.id);
  const orphans = new Set();
  for (const project of ["desktop", "phone"]) {
    const dir = path.join(baselineDir, project);
    if (!existsSync(dir)) continue;
    for (const id of orphanedBaselines(readdirSync(dir), storyIds)) {
      orphans.add(id);
    }
  }
  return [...orphans].sort();
}

function compare({ filter, allStories }) {
  if (!existsSync(metaFile)) {
    throw new StepFailed(
      "Keine Baseline. Zuerst: pnpm run kit-pixel baseline --ref <commit>",
      1,
    );
  }
  const meta = JSON.parse(readFileSync(metaFile, "utf8"));
  // Ohne eigenen Filter gilt die Auswahl der Baseline; sonst fehlten Bilder.
  const effectiveFilter = filter ?? meta.filter ?? undefined;
  const effectiveAll = allStories || meta.allStories;
  console.log(
    `Vergleich mit Baseline aus ${meta.ref} (${meta.sha.slice(0, 10)})` +
      (effectiveFilter ? `, Filter ${effectiveFilter}` : ""),
  );
  const storybookDir = path.join(workDir, "storybook-current");
  buildStorybook(frontendDir, storybookDir);
  const status = runPlaywright({
    mode: "compare",
    storybookDir,
    snapshotDir: baselineDir,
    filter: effectiveFilter,
    allStories: effectiveAll,
  });

  // Nur bei derselben Auswahl wie die Baseline: Ein engerer Filter lässt
  // Bilder absichtlich liegen.
  const sameSelection =
    effectiveFilter === (meta.filter ?? undefined) &&
    effectiveAll === meta.allStories;
  const orphans = sameSelection
    ? findOrphans(storybookDir, {
        filter: effectiveFilter,
        allStories: effectiveAll,
      })
    : [];
  if (orphans.length > 0) {
    console.error(
      `\nStories mit Baseline-Bild fehlen im Arbeitsstand (gelöscht oder umbenannt):\n  ${orphans.join("\n  ")}`,
    );
  }

  const report = path.relative(frontendDir, path.join(workDir, "report"));
  if (status === 0 && orphans.length === 0) {
    console.log("\nKeine Abweichung.");
    return;
  }
  if (status !== 0) {
    console.error(
      `\nAbweichungen oder Fehler. Diff-Bilder: ${path.relative(frontendDir, workDir)}/results/**/*-diff.png\n` +
        `Bericht mit Vorher/Nachher/Diff: pnpm exec playwright show-report ${report}`,
    );
  }
  process.exitCode = status || 1;
}

try {
  const options = parseKitPixelArgs(process.argv.slice(2));
  if (options.command === "baseline") baseline(options);
  else compare(options);
} catch (error) {
  console.error(`\n${error.message}`);
  process.exitCode = error instanceof StepFailed ? error.status : 2;
}
