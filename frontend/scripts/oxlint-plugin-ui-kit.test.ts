import { afterEach, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

const temporaryDirectories: string[] = [];

function lintSource(source: string, relativePath = "probe.tsx") {
  const directory = mkdtempSync(join(tmpdir(), "ui-kit-"));
  temporaryDirectories.push(directory);
  const sourcePath = join(directory, relativePath);
  mkdirSync(dirname(sourcePath), { recursive: true });
  writeFileSync(sourcePath, source);

  return spawnSync(
    resolve("node_modules/.bin/oxlint"),
    ["-c", resolve(".oxlintrc.json"), sourcePath],
    { encoding: "utf8" },
  );
}

describe("ui-kit/no-hand-rolled-overlay", () => {
  it("reports the first hand-rolled overlay in a formerly baselined file", () => {
    const result = lintSource(
      `function Probe() {
        return <div className="fixed inset-0">Overlay</div>;
      }
      void Probe;`,
      "src/components/background-wrapper.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output).toContain("ui-kit(no-hand-rolled-overlay)");
  });
});

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
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
