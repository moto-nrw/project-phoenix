import { afterEach, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

// bauart/no-autosave (BAUARTEN-SPEC, Bauart 2 Regel 4, #3112). Same harness
// as oxlint-plugin-bauart.test.ts: a probe file goes through the real oxlint
// with the repo config.

const temporaryDirectories: string[] = [];

function lintSource(source: string, relativePath = "src/components/probe.tsx") {
  const directory = mkdtempSync(join(tmpdir(), "bauart-autosave-"));
  temporaryDirectories.push(directory);
  const sourcePath = join(directory, relativePath);
  mkdirSync(dirname(sourcePath), { recursive: true });
  writeFileSync(sourcePath, source);

  const result = spawnSync(
    resolve("node_modules/.bin/oxlint"),
    ["-c", resolve(".oxlintrc.json"), sourcePath],
    { encoding: "utf8" },
  );
  return { status: result.status, output: `${result.stdout}${result.stderr}` };
}

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

const BLUR_SAVE = `import { Input } from "~/components/ui/input";
export function Probe({ save, value }: { save: (v: string) => Promise<void>; value: string }) {
  return (
    <Input
      value={value}
      onChange={() => {}}
      onBlur={() => {
        if (value !== "") {
          void save(value);
        }
      }}
    />
  );
}`;

describe("bauart/no-autosave", () => {
  it("rejects a write fired straight out of onBlur", () => {
    const { status, output } = lintSource(BLUR_SAVE);

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-autosave)");
    expect(output).toContain("onBlur schreibt sofort (save)");
  });

  it("rejects a kit select whose change handler writes", () => {
    const { status, output } = lintSource(
      `import { CustomSelect } from "~/components/ui/custom-select";
      export function Probe({ studentId }: { studentId: string }) {
        const setStudentPayer = async (_id: string, _v: string) => {};
        return (
          <CustomSelect
            value="a"
            options={[]}
            onChange={(value) => void setStudentPayer(studentId, value)}
          />
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("CustomSelect speichert im onChange sofort");
    expect(output).toContain("setStudentPayer");
  });

  it("sees a promise chain as a fired write", () => {
    const { status, output } = lintSource(
      `import { Checkbox } from "~/components/ui/checkbox";
      export function Probe({ update }: { update: (v: boolean) => Promise<void> }) {
        return (
          <label>
            <Checkbox checked onChange={(e) => update(e.target.checked).then(() => {})} />
            Aktiv
          </label>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-autosave)");
  });

  it("lets a change handler that only edits a draft pass", () => {
    const { status, output } = lintSource(
      `import { useState } from "react";
      import { Input } from "~/components/ui/input";
      export function Probe() {
        const [draft, setDraft] = useState("");
        return <Input value={draft} onChange={(e) => setDraft(e.target.value)} />;
      }`,
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("lets a read out of a change handler pass (search is not a save)", () => {
    const { status, output } = lintSource(
      `import { Input } from "~/components/ui/input";
      export function Probe({ search }: { search: (q: string) => Promise<void> }) {
        return <Input value="" onChange={(e) => void search(e.target.value)} />;
      }`,
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("lets a handler passed by name through (review covers it)", () => {
    const { status, output } = lintSource(
      `import { Input } from "~/components/ui/input";
      export function Probe({ onBlur }: { onBlur: () => void }) {
        return <Input value="" onChange={() => {}} onBlur={onBlur} />;
      }`,
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("exempts the settings page, the only Bauart that auto-saves", () => {
    const { status, output } = lintSource(
      BLUR_SAVE,
      "src/components/settings/probe-field.tsx",
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("exempts the operator portal, which the spec does not cover", () => {
    const { status, output } = lintSource(
      BLUR_SAVE,
      "src/app/operator/probe/page.tsx",
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("exempts the named baseline entries", () => {
    const { status, output } = lintSource(
      BLUR_SAVE,
      "src/app/[tenant]/(protected)/payroll/page.tsx",
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });
});
