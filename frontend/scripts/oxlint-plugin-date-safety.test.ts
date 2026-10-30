import { afterEach, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

const temporaryDirectories: string[] = [];

function lintSource(source: string, relativePath = "src/probe.ts") {
  const directory = mkdtempSync(join(tmpdir(), "school-today-"));
  temporaryDirectories.push(directory);
  const sourcePath = join(directory, relativePath);
  mkdirSync(dirname(sourcePath), { recursive: true });
  writeFileSync(sourcePath, source);

  const configPath = join(directory, "oxlint.json");
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
  const result = spawnSync(
    resolve("node_modules/.bin/oxlint"),
    ["-c", configPath, sourcePath],
    { encoding: "utf8" },
  );
  return { ...result, output: `${result.stdout}${result.stderr}` };
}

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe("date-safety/no-browser-today", () => {
  it.each([
    "const date = toISODate(new Date());",
    "const date = formatDateISO(new Date());",
    'const date = format(new Date(), "yyyy-MM-dd");',
    "const now = new Date(); const date = now.toLocaleDateString();",
    "const date = new Date().getDate();",
    "const date = todayISO();",
    "const date = helpers.todayISO();",
    'import { todayISO as localDay } from "~/lib/date-helpers"; const date = localDay();',
  ])("reports the browser-local source: %s", (source) => {
    const result = lintSource(source);
    expect(result.status).toBe(1);
    expect(result.output).toContain("date-safety(no-browser-today)");
  });

  it("allows the explicit Berlin helpers, conversions of supplied dates and instants", () => {
    const result = lintSource(
      'const day = berlinTodayISO(); const reactive = useBerlinToday(); const date = toISODate(new Date("2026-05-27T00:00:00"));',
    );
    expect(result.status, result.output).toBe(0);
  });

  it("allows local-date implementation in the central helper and tests", () => {
    expect(
      lintSource(
        "const date = toISODate(new Date());",
        "src/lib/date-helpers.ts",
      ).status,
    ).toBe(0);
    expect(
      lintSource("const date = toISODate(new Date());", "src/probe.test.ts")
        .status,
    ).toBe(0);
  });

  it("consumes reviewed local exceptions once and rejects new or stale ones", () => {
    const file = "src/components/students/student-form-fields.tsx";
    const reviewed =
      "export function Probe() { return (<input\nmax={todayISO()}\n/>); }";
    expect(lintSource(reviewed, file).status).toBe(0);
    const duplicate =
      "export function Probe() { return (<><input\nmax={todayISO()}\n/><input\nmax={todayISO()}\n/></>); }";
    const added = lintSource(duplicate, file);
    expect(added.status).toBe(1);
    expect(added.output).toContain("School today must come from");
    const stale = lintSource("export const value = true;", file);
    expect(stale.status).toBe(1);
    expect(stale.output).toContain(
      "Remove the stale school-today baseline entry",
    );
  });

  it("keeps the UTC-extraction guard", () => {
    const result = lintSource("const date = input.toISOString().slice(0, 10);");
    expect(result.status).toBe(1);
    expect(result.output).toContain("date-safety(no-utc-date-extraction)");
  });
});
