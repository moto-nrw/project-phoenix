import schoolTodayBaseline from "./school-today-baseline.mjs";
import { afterAll, describe, expect, it } from "vitest";
import {
  existsSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

import {
  lintSource as lintProbe,
  removeProbeDirectories,
} from "./oxlint-probe";

// Only the date-safety plugin with its two rules, as before; the shared probe
// batches the concurrent cases of this file into one oxlint run.
const configDirectory = mkdtempSync(join(tmpdir(), "school-today-"));
const configPath = join(configDirectory, "oxlint.json");
writeFileSync(
  configPath,
  JSON.stringify({
    categories: { correctness: "off", suspicious: "off", perf: "off" },
    jsPlugins: [resolve("scripts/oxlint-plugin-date-safety.mjs")],
    rules: {
      "date-safety/no-browser-today": "error",
      "date-safety/no-utc-date-extraction": "error",
    },
  }),
);

function lintSource(source: string, relativePath = "src/probe.ts") {
  return lintProbe(source, relativePath, configPath);
}

afterAll(() => {
  removeProbeDirectories();
  rmSync(configDirectory, { recursive: true, force: true });
});

describe.concurrent("date-safety/no-browser-today", () => {
  it.each([
    "const date = toISODate(new Date());",
    "const date = toISODate(new Date(Date.now()));",
    "const date = toISODate(new globalThis.Date());",
    "const date = new globalThis.Date(globalThis.Date.now());",
    'import { isToday as currentDay } from "date-fns/isToday"; const yes = currentDay(input);',
    'import { startOfToday } from "date-fns/startOfToday"; const day = startOfToday();',
    'import * as fns from "date-fns"; const yes = fns.isToday(input);',
    'import currentDay from "date-fns/isToday"; const yes = currentDay(input);',
    "const date = formatDateISO(new Date());",
    'const date = format(new Date(), "yyyy-MM-dd");',
    "const now = new Date(); const date = now.toLocaleDateString();",
    "const date = new Date().getDate();",
    "const date = todayISO();",
    "const date = helpers.todayISO();",
    'import { todayISO as localDay } from "~/lib/date-helpers"; const date = localDay();',
  ])("reports the browser-local source: %s", async (source) => {
    const result = await lintSource(source);
    expect(result.status).toBe(1);
    expect(result.output).toContain("date-safety(no-browser-today)");
  });

  it("allows the explicit Berlin helpers, conversions of supplied dates and instants", async () => {
    const result = await lintSource(
      'const day = berlinTodayISO(); const reactive = useBerlinToday(); const date = toISODate(new Date("2026-05-27T00:00:00"));',
    );
    expect(result.status, result.output).toBe(0);
  });

  it("limits the central helper exception to its reviewed clock-source sites", async () => {
    const source = readFileSync(resolve("src/lib/date-helpers.ts"), "utf8");
    const [helpers, added, testFile] = await Promise.all([
      lintSource(source, "src/lib/date-helpers.ts"),
      lintSource(
        source + "\nexport const unsafe = toISODate(new Date());",
        "src/lib/date-helpers.ts",
      ),
      lintSource("const date = toISODate(new Date());", "src/probe.test.ts"),
    ]);
    expect(helpers.status).toBe(0);
    expect(added.status).toBe(1);
    expect(added.output).toContain("School today must come from");
    expect(testFile.status).toBe(0);
  });

  it("rejects baseline entries for deleted files", () => {
    for (const file of Object.keys(schoolTodayBaseline)) {
      expect(existsSync(resolve(file)), `Stale baseline file: ${file}`).toBe(
        true,
      );
    }
  });

  it("consumes reviewed local exceptions once and rejects new or stale ones", async () => {
    const file = "src/components/students/student-form-fields.tsx";
    const reviewed =
      "export function Probe() { return (<input\nmax={todayISO()}\n/>); }";
    const duplicate =
      "export function Probe() { return (<><input\nmax={todayISO()}\n/><input\nmax={todayISO()}\n/></>); }";
    const [reviewedResult, added, stale, noLongerViolating] = await Promise.all(
      [
        lintSource(reviewed, file),
        lintSource(duplicate, file),
        lintSource("export const value = true;", file),
        lintSource('export const note = "max={todayISO()}";', file),
      ],
    );
    expect(reviewedResult.status).toBe(0);
    expect(added.status).toBe(1);
    expect(added.output).toContain("School today must come from");
    expect(stale.status).toBe(1);
    expect(stale.output).toContain(
      "Remove the stale school-today baseline entry",
    );
    expect(noLongerViolating.status).toBe(1);
    expect(noLongerViolating.output).toContain(
      "Remove the stale school-today baseline entry",
    );
  });

  it("keeps the UTC-extraction guard", async () => {
    const result = await lintSource(
      "const date = input.toISOString().slice(0, 10);",
    );
    expect(result.status).toBe(1);
    expect(result.output).toContain("date-safety(no-utc-date-extraction)");
  });
});
