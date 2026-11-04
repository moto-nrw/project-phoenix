import { afterAll, describe, expect, it } from "vitest";
import { lintSource, removeProbeDirectories } from "./oxlint-probe";

afterAll(removeProbeDirectories);

const RULE = "school-class(no-hand-built-label)";

describe.concurrent("school-class/no-hand-built-label", () => {
  it.each([
    "export const a = (c: string) => `Klasse ${c}`;",
    "export const a = (c: string) => `für die Klasse ${c} entfernen`;",
    "export const a = (c: string) => `Die Klasse „${c}“ passt nicht.`;",
    'export const a = (c: string[]) => `Klassen ${c.join(", ")}`;',
    'export const a = (c: string) => "Klasse " + c;',
    'export const a = (n: string, c: string) => n + ", Klasse " + c;',
    "export const A = ({ c }: { c: string }) => <span>Klasse {c}</span>;",
    'export const A = ({ c }: { c: string }) => <span>Klasse{" "}{c}</span>;',
    "export const A = ({ c }: { c: string }) => (\n  <span>\n    Klasse\n    {c}\n  </span>\n);",
    'export const A = ({ c }: { c: string }) => <><span className="sr-only">x</span>Klasse {c}</>;',
  ])("rejects a hand-built class label: %s", async (source) => {
    const result = await lintSource(source);
    expect(result.status, result.output).toBe(1);
    expect(result.output).toContain(RULE);
    expect(result.output).toContain("schoolClassLabel");
  });

  it("accepts the helper, grade levels and plain words", async () => {
    const result = await lintSource(`
      import { schoolClassLabel } from "~/lib/school-class-label";
      export const a = (c: string) => \`die \${schoolClassLabel(c)}\`;
      export const b = (grade: number) => \`Kind: Klasse \${grade}\`;
      export const c = (item: { target_grade_level: number }) => \`Klasse \${item.target_grade_level}\`;
      export const d = (n: number) => \`\${n}. Klasse\`;
      export const e = "Klasse " + 3;
      export const f = (c: string) => \`Unterklasse \${c}\`;
      export const G = ({ grade }: { grade: number }) => <span>Klasse {grade}</span>;
      export const H = () => <p>Klasse <span>*</span></p>;
    `);
    expect(result.status, result.output).toBe(0);
  });

  it("leaves the helper itself and tests alone", async () => {
    const source = "export const a = (c: string) => `Klasse ${c}`;";
    for (const file of [
      "src/lib/school-class-label.ts",
      "src/components/probe.test.tsx",
    ]) {
      const result = await lintSource(source, file);
      expect(result.status, result.output).toBe(0);
    }
  });
});
