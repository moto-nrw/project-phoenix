import { afterAll, describe, expect, it } from "vitest";
import { lintSource, removeProbeDirectories } from "./oxlint-probe";

// bauart/no-autosave (BAUARTEN-SPEC, Bauart 2 Regel 4, #3112). Same harness
// as oxlint-plugin-bauart.test.ts: a probe file goes through the real oxlint
// with the repo config.

afterAll(removeProbeDirectories);

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

describe.concurrent("bauart/no-autosave", () => {
  it("rejects a write fired straight out of onBlur", async () => {
    const { status, output } = await lintSource(BLUR_SAVE);

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-autosave)");
    expect(output).toContain("onBlur schreibt sofort (save)");
  });

  it("rejects a direct write out of onBlur", async () => {
    const { status, output } = await lintSource(
      `import { Input } from "~/components/ui/input";
      export function Probe({ save, value }: { save: (v: string) => Promise<void>; value: string }) {
        return <Input value={value} onChange={() => {}} onBlur={() => save(value)} />;
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("onBlur schreibt sofort (save)");
  });

  it("rejects a direct Promise-returning save prop in an arrow component", async () => {
    const { status, output } = await lintSource(
      `import { Input } from "~/components/ui/input";
      const Probe = ({ save, value }: { save: (v: string) => Promise<void>; value: string }) => (
        <Input value={value} onChange={() => {}} onBlur={() => save(value)} />
      );
      export { Probe };`,
    );

    expect(status).toBe(1);
    expect(output).toContain("onBlur schreibt sofort (save)");
  });

  it("rejects a direct imported write out of onBlur", async () => {
    const { status, output } = await lintSource(
      `import { Input } from "~/components/ui/input";
      import { updateMasterDataField as savePayment } from "~/lib/parent-api";
      export function Probe({ value }: { value: string }) {
        return <Input value={value} onChange={() => {}} onBlur={() => savePayment(value)} />;
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("onBlur schreibt sofort (savePayment)");
  });

  it("lets validation from onBlur pass", async () => {
    const { status, output } = await lintSource(
      `import { Input } from "~/components/ui/input";
      export function Probe({ validate, value }: { validate: (v: string) => Promise<void>; value: string }) {
        return <Input value={value} onChange={() => {}} onBlur={() => void validate(value)} />;
      }`,
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("rejects a kit select whose change handler writes", async () => {
    const { status, output } = await lintSource(
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

  it("rejects a direct write from a kit field change handler", async () => {
    const { status, output } = await lintSource(
      `import { Input } from "~/components/ui/input";
      export function Probe({ update }: { update: (v: string) => Promise<void> }) {
        return <Input value="" onChange={(event) => update(event.target.value)} />;
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("Input speichert im onChange sofort (update)");
  });

  it("rejects a direct imported write from a kit field change handler", async () => {
    const { status, output } = await lintSource(
      `import { CustomSelect } from "~/components/ui/custom-select";
      import { updateMasterDataField as updatePayment } from "~/lib/parent-api";
      export function Probe() {
        return <CustomSelect value="" options={[]} onChange={(value) => updatePayment(value)} />;
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain(
      "CustomSelect speichert im onChange sofort (updatePayment)",
    );
  });

  it("sees a promise chain as a fired write", async () => {
    const { status, output } = await lintSource(
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

  it("lets a change handler that only edits a draft pass", async () => {
    const { status, output } = await lintSource(
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

  it("lets a synchronous write-named draft helper pass", async () => {
    const { status, output } = await lintSource(
      `import { useState } from "react";
      import { Input } from "~/components/ui/input";
      export function Probe() {
        const [draft, setDraft] = useState("");
        const updateDraft = (next: string) => setDraft(next);
        return <Input value={draft} onChange={(event) => updateDraft(event.target.value)} />;
      }`,
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("lets a read out of a change handler pass (search is not a save)", async () => {
    const { status, output } = await lintSource(
      `import { Input } from "~/components/ui/input";
      export function Probe({ search }: { search: (q: string) => Promise<void> }) {
        return <Input value="" onChange={(e) => void search(e.target.value)} />;
      }`,
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("lets a handler passed by name through (review covers it)", async () => {
    const { status, output } = await lintSource(
      `import { Input } from "~/components/ui/input";
      export function Probe({ onBlur }: { onBlur: () => void }) {
        return <Input value="" onChange={() => {}} onBlur={onBlur} />;
      }`,
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("exempts the settings page, the only Bauart that auto-saves", async () => {
    const { status, output } = await lintSource(
      BLUR_SAVE,
      "src/components/settings/probe-field.tsx",
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("exempts the operator portal, which the spec does not cover", async () => {
    const { status, output } = await lintSource(
      BLUR_SAVE,
      "src/app/operator/probe/page.tsx",
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });

  it("exempts the named baseline entries", async () => {
    const { status, output } = await lintSource(
      BLUR_SAVE,
      "src/app/[tenant]/(protected)/payroll/page.tsx",
    );

    expect(output).not.toContain("bauart(no-autosave)");
    expect(status).toBe(0);
  });
});
