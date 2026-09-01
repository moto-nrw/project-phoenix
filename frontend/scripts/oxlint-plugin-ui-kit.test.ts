import { afterEach, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const temporaryDirectories: string[] = [];

function lintSource(source: string, { underSrc = false } = {}) {
  const directory = mkdtempSync(join(tmpdir(), "ui-kit-"));
  temporaryDirectories.push(directory);
  // The class-string rules only run on repo-relative "src/…" files.
  const sourceDirectory = underSrc ? join(directory, "src") : directory;
  if (underSrc) mkdirSync(sourceDirectory);
  const sourcePath = join(sourceDirectory, "probe.tsx");
  writeFileSync(sourcePath, source);

  return spawnSync(
    resolve("node_modules/.bin/oxlint"),
    ["-c", resolve(".oxlintrc.json"), sourcePath],
    { encoding: "utf8" },
  );
}

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe("ui-kit/no-hand-rolled-surface", () => {
  it("reports a hand-built card surface outside the baseline", () => {
    const result = lintSource(
      `
      function Probe() {
        return <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm" />;
      }
      void Probe;
    `,
      { underSrc: true },
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-surface\)/g)).toHaveLength(1);
  });

  it("accepts moto-content-surface and non-card shapes", () => {
    const result = lintSource(
      `
      function Probe() {
        return <>
          <div className="moto-content-surface rounded-2xl border p-4 shadow-sm" />
          <button className="rounded-lg border border-gray-300 bg-white px-4 py-2" />
          <div className="rounded-2xl border border-gray-200 bg-gray-50 p-4" />
        </>;
      }
      void Probe;
    `,
      { underSrc: true },
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-hand-rolled-surface");
  });
});

describe("ui-kit/require-checkbox-label", () => {
  it("reports sibling labels and aria-label-only checkboxes", () => {
    const result = lintSource(`
      const Checkbox = (props: { id?: string; "aria-label"?: string }) =>
        <input type="checkbox" {...props} />;
      function Probe() {
        return <>
          <div>
            <Checkbox id="monday" />
            <label htmlFor="monday">Montag</label>
          </div>
          <Checkbox aria-label="Kind auswählen" />
        </>;
      }
      void Probe;
    `);
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(require-checkbox-label\)/g)).toHaveLength(2);
  });

  it("accepts a checkbox nested anywhere inside its label", () => {
    const result = lintSource(`
      const Checkbox = (props: { id: string }) =>
        <input type="checkbox" {...props} />;
      function Probe() {
        return <label htmlFor="monday">
          <span><Checkbox id="monday" /></span>
          <span>Montag</span>
        </label>;
      }
      void Probe;
    `);
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("require-checkbox-label");
  });
});
