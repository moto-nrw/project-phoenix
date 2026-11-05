// Gemeinsamer Kern der shrink-only Ratchet-Baselines des ui-kit-Plugins
// (Knöpfe #5531, Pillen #5584): zählt die Funde je Datei und Regel, zieht die
// Allowlist ab und schreibt die Baseline neu; sie darf nur sinken. `--check`
// schreibt nichts und meldet Abweichungen.
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import ts from "typescript";

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), "..");

// Der Lauf braucht nur das ui-kit-Plugin und die Regeln der Ratsche, mit
// ihren Einstellungen aus .oxlintrc.json. Ihre Funde hängen von keiner
// anderen Regel ab; ohne die übrigen Plugins braucht der Lauf über src rund
// ein Zehntel des Speichers (0,24 statt 2,7 GB) und ein Sechstel der CPU-Zeit.
function ratchetConfig(rules) {
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
  // overrides könnten die Regeln je Datei ändern; die abgeleitete Config
  // übernähme das nicht und zählte dann anders als `pnpm run lint`.
  if (overrides?.length) {
    throw new Error(
      ".oxlintrc.json hat overrides: die Ratsche zählt dann nicht mehr wie der Volllauf",
    );
  }
  const uiKitPlugin = jsPlugins.find((plugin) =>
    /oxlint-plugin-ui-kit\.mjs$/.test(plugin),
  );
  if (!uiKitPlugin) {
    throw new Error(".oxlintrc.json lädt das ui-kit-Plugin nicht");
  }
  const config = {
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
  return { config, ignorePatterns };
}

/**
 * @param {{
 *   rules: string[];
 *   baselineFile: string;
 *   pathEnv: string;
 *   countEnv: string;
 *   allowlist: ReadonlyArray<{ rule: string; file: string; lines: readonly number[] }>;
 *   command: string;
 * }} options
 */
export function updateRatchetBaseline({
  rules,
  baselineFile,
  pathEnv,
  countEnv,
  allowlist,
  command,
}) {
  const baselinePath = resolve(
    process.env[pathEnv] ?? resolve(frontend, "scripts", baselineFile),
  );
  const { config, ignorePatterns } = ratchetConfig(rules);
  const configDirectory = mkdtempSync(join(tmpdir(), "ui-kit-ratchet-"));
  let result;
  try {
    const configPath = join(configDirectory, "oxlintrc.json");
    writeFileSync(configPath, JSON.stringify(config));
    // ignorePatterns gelten relativ zu .oxlintrc.json, also zu frontend/.
    result = spawnSync(
      resolve(frontend, "node_modules/.bin/oxlint"),
      [
        "-c",
        configPath,
        ...ignorePatterns.flatMap((pattern) => ["--ignore-pattern", pattern]),
        "-f",
        "json",
        "src",
      ],
      {
        cwd: frontend,
        encoding: "utf8",
        env: { ...process.env, [countEnv]: "count" },
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
  for (const entry of allowlist) {
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
      `${baselineFile} darf nur sinken. Neue Funde zuerst abbauen:\n${increases.join("\n")}`,
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
        `${baselineFile} weicht vom Ist-Stand ab. Nach einem Abbau: ${command}\n${drift.join("\n")}`,
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
}
