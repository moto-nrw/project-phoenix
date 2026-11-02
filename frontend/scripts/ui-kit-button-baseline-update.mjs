#!/usr/bin/env node
// Schreibt scripts/ui-kit-button-baseline.json neu (#5531): Funde je Datei der
// Regeln ui-kit/no-hand-rolled-button und ui-kit/no-button-override, ohne die
// Ausnahmen der Allowlist. Nur nach dem Abbau von Funden ausführen; die
// Baseline darf nur sinken. `--check` schreibt nichts und meldet Abweichungen.
import { spawnSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { buttonRatchetAllowlist } from "./ui-kit-button-baseline.mjs";

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const baselinePath = resolve(frontend, "scripts/ui-kit-button-baseline.json");
const rules = ["no-hand-rolled-button", "no-button-override"];

const result = spawnSync(
  resolve(frontend, "node_modules/.bin/oxlint"),
  ["-c", ".oxlintrc.json", "-f", "json", "src"],
  {
    cwd: frontend,
    encoding: "utf8",
    env: { ...process.env, UI_KIT_BUTTON_RATCHET: "count" },
    maxBuffer: 64 * 1024 * 1024,
  },
);
if (result.error) throw result.error;

const findings = Object.fromEntries(rules.map((rule) => [rule, {}]));
for (const diagnostic of JSON.parse(result.stdout).diagnostics) {
  const rule = /ui-kit\(([\w-]+)\)/.exec(diagnostic.code ?? "")?.[1];
  if (!rule || !(rule in findings)) continue;
  const line = diagnostic.labels?.[0]?.span?.line;
  if (!line) {
    throw new Error(`Fund ohne Quellzeile: ${diagnostic.filename}`);
  }
  (findings[rule][diagnostic.filename] ??= []).push(line);
}
const counts = Object.fromEntries(rules.map((rule) => [rule, {}]));
for (const rule of rules) {
  for (const [file, lines] of Object.entries(findings[rule])) {
    counts[rule][file] = lines.length;
  }
}
// Die Allowlist zieht nur ihre festen Fundstellen ab; eine Ausnahme ohne Fund
// ist veraltet. Ein anderer Fund in derselben Datei verbraucht sie nicht.
const allowlistErrors = [];
for (const entry of buttonRatchetAllowlist) {
  const lines = findings[entry.rule]?.[entry.file] ?? [];
  for (const line of entry.lines) {
    const index = lines.indexOf(line);
    if (index === -1) {
      allowlistErrors.push(
        `Allowlist ${entry.rule} ${entry.file}:${line}: kein Fund`,
      );
    } else {
      lines.splice(index, 1);
      counts[entry.rule][entry.file] -= 1;
    }
  }
  if (counts[entry.rule][entry.file] === 0) {
    delete counts[entry.rule][entry.file];
  }
}
if (allowlistErrors.length > 0) {
  console.error(allowlistErrors.join("\n"));
  process.exit(1);
}

const sorted = Object.fromEntries(
  rules.map((rule) => [
    rule,
    Object.fromEntries(
      Object.entries(counts[rule]).sort(([a], [b]) => a.localeCompare(b)),
    ),
  ]),
);
const next = `${JSON.stringify(sorted, null, 2)}\n`;

if (process.argv.includes("--check")) {
  const current = JSON.parse(readFileSync(baselinePath, "utf8"));
  const drift = [];
  for (const rule of rules) {
    const files = new Set([
      ...Object.keys(current[rule] ?? {}),
      ...Object.keys(sorted[rule]),
    ]);
    for (const file of files) {
      const was = current[rule]?.[file] ?? 0;
      const now = sorted[rule][file] ?? 0;
      if (was !== now)
        drift.push(`${rule} ${file}: Baseline ${was}, Ist ${now}`);
    }
  }
  if (drift.length > 0) {
    console.error(
      `ui-kit-button-baseline.json weicht vom Ist-Stand ab. Nach einem Abbau: pnpm run ui-kit:button-baseline\n${drift.join("\n")}`,
    );
    process.exit(1);
  }
} else {
  writeFileSync(baselinePath, next);
}
for (const rule of rules) {
  const files = Object.values(sorted[rule]);
  console.log(
    `${rule}: ${files.reduce((sum, count) => sum + count, 0)} Funde in ${files.length} Dateien`,
  );
}
