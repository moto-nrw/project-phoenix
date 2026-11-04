#!/usr/bin/env node
// Schreibt scripts/ui-kit-button-baseline.json neu (#5531): Funde je Datei der
// Regeln ui-kit/no-hand-rolled-button, ui-kit/no-button-override und
// ui-kit/no-grey-button-fill (#5574, immer leer), ohne die
// Ausnahmen der Allowlist. Nur nach dem Abbau von Funden ausführen; die
// Baseline darf nur sinken. `--check` schreibt nichts und meldet Abweichungen.
import { spawnSync } from "node:child_process";
import {
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import ts from "typescript";

import { buttonRatchetAllowlist } from "./ui-kit-button-baseline.mjs";

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const baselinePath = resolve(
  process.env.UI_KIT_BUTTON_BASELINE_PATH ??
    resolve(frontend, "scripts/ui-kit-button-baseline.json"),
);
// no-grey-button-fill (#5574) ist hart null: Die Zeile hier prüft nur ihre
// Allowlist; ein Fund außerhalb davon hebt die Baseline nicht, er schlägt fehl.
const rules = [
  "no-hand-rolled-button",
  "no-button-override",
  "no-grey-button-fill",
];

// Der Lauf braucht nur das ui-kit-Plugin und diese drei Regeln, mit ihren
// Einstellungen aus .oxlintrc.json. Ihre Funde hängen von keiner anderen
// Regel ab; ohne die übrigen acht Plugins braucht der Lauf über src rund ein
// Zehntel des Speichers (0,24 statt 2,7 GB) und ein Sechstel der CPU-Zeit.
const configPath = resolve(frontend, ".oxlintrc.json");
const parsed = ts.parseConfigFileTextToJson(
  configPath,
  readFileSync(configPath, "utf8"),
);
if (parsed.error || !parsed.config) {
  throw new Error(`.oxlintrc.json nicht lesbar: ${configPath}`);
}
const {
  plugins: _plugins,
  jsPlugins = [],
  categories: _categories,
  rules: configuredRules = {},
  overrides,
  ignorePatterns = [],
  ...shared
} = parsed.config;
// overrides könnten die drei Regeln je Datei ändern; die abgeleitete Config
// übernähme das nicht und zählte dann anders als `pnpm run lint`.
if (overrides?.length) {
  throw new Error(
    ".oxlintrc.json hat overrides: die Button-Ratsche zählt dann nicht mehr wie der Volllauf",
  );
}
const uiKitPlugin = jsPlugins.find((plugin) =>
  /oxlint-plugin-ui-kit\.mjs$/.test(plugin),
);
if (!uiKitPlugin) throw new Error(".oxlintrc.json lädt das ui-kit-Plugin nicht");
const ratchetConfig = {
  ...shared,
  plugins: [],
  categories: Object.fromEntries(
    [
      "correctness",
      "suspicious",
      "pedantic",
      "perf",
      "style",
      "restriction",
      "nursery",
    ].map((category) => [category, "off"]),
  ),
  jsPlugins: [resolve(frontend, uiKitPlugin)],
  rules: Object.fromEntries(
    rules.map((rule) => {
      const setting = configuredRules[`ui-kit/${rule}`];
      if (setting === undefined) {
        throw new Error(`.oxlintrc.json schaltet ui-kit/${rule} nicht ein`);
      }
      return [`ui-kit/${rule}`, setting];
    }),
  ),
};
const configDirectory = mkdtempSync(join(tmpdir(), "ui-kit-button-ratchet-"));
let result;
try {
  const ratchetConfigPath = join(configDirectory, "oxlintrc.json");
  writeFileSync(ratchetConfigPath, JSON.stringify(ratchetConfig));
  // ignorePatterns gelten relativ zu .oxlintrc.json, also zu frontend/.
  result = spawnSync(
    resolve(frontend, "node_modules/.bin/oxlint"),
    [
      "-c",
      ratchetConfigPath,
      ...ignorePatterns.flatMap((pattern) => ["--ignore-pattern", pattern]),
      "-f",
      "json",
      "src",
    ],
    {
      cwd: frontend,
      encoding: "utf8",
      env: { ...process.env, UI_KIT_BUTTON_RATCHET: "count" },
      maxBuffer: 64 * 1024 * 1024,
    },
  );
} finally {
  rmSync(configDirectory, { recursive: true, force: true });
}
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
const current = JSON.parse(readFileSync(baselinePath, "utf8"));
const increases = [];
for (const rule of rules) {
  for (const [file, now] of Object.entries(sorted[rule])) {
    const was = current[rule]?.[file] ?? 0;
    if (now > was) {
      increases.push(`${rule} ${file}: Baseline ${was}, Ist ${now}`);
    }
  }
}
if (increases.length > 0) {
  console.error(
    `ui-kit-button-baseline.json darf nur sinken. Neue Funde zuerst abbauen:\n${increases.join("\n")}`,
  );
  process.exit(1);
}

if (process.argv.includes("--check")) {
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
