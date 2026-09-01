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

describe("ui-kit/no-hand-rolled-surface", () => {
  it("reports a hand-built card surface anywhere outside the kit", () => {
    const result = lintSource(
      `
      function Probe() {
        return <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm" />;
      }
      void Probe;
    `,
      "src/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-surface\)/g)).toHaveLength(1);
  });

  it("reports a surface assembled across conditional className branches", () => {
    const result = lintSource(
      `
      function Probe({ active }: { active: boolean }) {
        return <div className={\`rounded-xl border p-3 \${active ? "border-gray-900 bg-gray-50" : "border-gray-200 bg-white"}\`} />;
      }
      void Probe;
    `,
      "src/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-surface\)/g)).toHaveLength(1);
  });

  it("accepts the kit surfaces and non-card shapes", () => {
    const result = lintSource(
      `
      function Probe() {
        return <>
          <div className="moto-content-surface rounded-2xl border p-4 shadow-sm" />
          <div className="moto-popover-surface rounded-xl border py-1" />
          <button className="rounded-lg border border-gray-300 bg-white px-4 py-2" />
          <div className="rounded-2xl border border-gray-200 bg-gray-50 p-4" />
        </>;
      }
      void Probe;
    `,
      "src/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-hand-rolled-surface");
  });

  it("ignores prefixed tokens, frameless borders and translucent fills", () => {
    const result = lintSource(
      `
      function Probe() {
        return <>
          <input className="rounded-xl border-0 bg-white px-4 ring-1" />
          <div className="rounded-xl border-none bg-white p-4" />
          <div className="rounded-2xl border-b bg-white p-4" />
          <div className="rounded-2xl border border-gray-200 bg-white/95 p-4 shadow-lg" />
          <div className="rounded-xl border focus:bg-white p-4" />
          <div className="flex gap-4 print:rounded-2xl print:border print:bg-white" />
          <div className="rounded-lg border bg-white sm:rounded-2xl" />
        </>;
      }
      void Probe;
    `,
      "src/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-hand-rolled-surface");
  });

  it("honours the documented oxlint-disable-next-line escape hatch", () => {
    const result = lintSource(
      `
      function Probe() {
        return (
          // oxlint-disable-next-line ui-kit/no-hand-rolled-surface -- white pill control, not a card
          <span className="rounded-xl border border-gray-200 bg-white px-2" />
        );
      }
      void Probe;
    `,
      "src/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-hand-rolled-surface");
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

  it("accepts a checkbox inside a label-shaped ChoiceTile, not a button one", () => {
    const result = lintSource(`
      const Checkbox = (props: { id?: string }) =>
        <input type="checkbox" {...props} />;
      const ChoiceTile = (props: { as?: string; children: unknown }) =>
        <div>{String(props.children)}</div>;
      function Probe() {
        return <>
          <ChoiceTile><Checkbox /> Montag</ChoiceTile>
          <ChoiceTile as="label"><Checkbox /> Dienstag</ChoiceTile>
          <ChoiceTile as="div"><Checkbox /> Mittwoch</ChoiceTile>
        </>;
      }
      void Probe;
    `);
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(require-checkbox-label\)/g)).toHaveLength(1);
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
