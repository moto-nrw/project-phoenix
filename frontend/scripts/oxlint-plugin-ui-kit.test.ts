import { afterEach, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

import {
  buttonRatchetAllowlist,
  buttonRatchetBaseline,
} from "./ui-kit-button-baseline.mjs";

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

describe("ui-kit/no-tiny-text", () => {
  it("rejects a legacy token reintroduced at a different source location", () => {
    const result = lintSource(
      `function Probe() {
        return <span className="text-[11px]">Probe</span>;
      }
      void Probe;`,
      "src/app/[tenant]/(protected)/calendar/page.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output).toContain("ui-kit(no-tiny-text)");
  });

  it("rejects an additional legacy token on the same baseline line", () => {
    const result = lintSource(
      `${"\n".repeat(31)}const className = "text-[10px] text-[10px]";`,
      "src/components/ui/avatar.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-tiny-text\)/g)).toHaveLength(1);
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
          {/* Knopfform als <span>: ein natives <button> fiele unter
              no-hand-rolled-button (#5531); geprüft wird hier nur die Fläche. */}
          <span className="rounded-lg border border-gray-300 bg-white px-4 py-2" />
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
          <div className="rounded-xl border-0 bg-white px-4 ring-1" />
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

describe("ui-kit/no-rounded-3xl", () => {
  it("reports rounded-3xl, also inside the kit", () => {
    for (const path of ["src/probe.tsx", "src/components/ui/probe.tsx"]) {
      const result = lintSource(
        `function Probe() {
          return <div className="rounded-3xl p-4" />;
        }
        void Probe;`,
        path,
      );
      const output = `${result.stdout}${result.stderr}`;

      expect(result.status, path).toBe(1);
      expect(output.match(/ui-kit\(no-rounded-3xl\)/g), path).toHaveLength(1);
    }
  });

  it("accepts the canonical radius and exempts test files", () => {
    const canonical = lintSource(
      `function Probe() {
        return <div className="moto-content-surface rounded-2xl border p-4" />;
      }
      void Probe;`,
      "src/probe.tsx",
    );
    const fixture = lintSource(
      `const className = "rounded-3xl";
      void className;`,
      "src/probe.test.tsx",
    );

    for (const result of [canonical, fixture]) {
      const output = `${result.stdout}${result.stderr}`;
      expect(result.status, output).toBe(0);
      expect(output).not.toContain("no-rounded-3xl");
    }
  });
});

describe("ui-kit/no-tenant-kicker", () => {
  const kickerSource = `function SectionCard(_: { kicker?: string; title: string }) {
      return null;
    }
    function Probe() {
      return <SectionCard kicker="Verwaltung" title="Räume" />;
    }
    void Probe;`;

  it("reports a kicker in the tenant portal", () => {
    const result = lintSource(
      kickerSource,
      "src/app/[tenant]/(protected)/rooms/page.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-tenant-kicker\)/g)).toHaveLength(1);
  });

  // Each CLI startup gets its own test budget, also on busy runners.
  it.each([
    "src/components/parent/probe.tsx",
    "src/app/parents/probe/page.tsx",
    "src/components/school/probe.tsx",
    "src/app/operator/probe/page.tsx",
    "src/app/[tenant]/(protected)/rooms/page.test.tsx",
  ])("leaves %s alone", (path) => {
    const result = lintSource(kickerSource, path);
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status, `${path}\n${output}`).toBe(0);
    expect(output).not.toContain("no-tenant-kicker");
  });
});

describe("ui-kit/no-hand-rolled-page-card", () => {
  it.each([
    "src/app/[tenant]/(protected)/page.tsx",
    "src/app/[tenant]/(protected)/loading.tsx",
    "src/app/[tenant]/(protected)/rooms/page.tsx",
    "src/app/[tenant]/(protected)/rooms/[id]/loading.tsx",
    // Das Träger-Portal rendert dasselbe Gerüst (#5146).
    "src/app/traeger/page.tsx",
    "src/app/traeger/ferien/[id]/page.tsx",
  ])("reports the kit card chain spelled out in %s", (path) => {
    const result = lintSource(
      `function Probe({ wide }: { wide: boolean }) {
          return <>
            <div className="moto-content-surface rounded-2xl border p-6" />
            <div className={\`rounded-2xl border p-4 moto-content-surface \${wide ? "w-full" : ""}\`} />
          </>;
        }
        void Probe;`,
      path,
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status, path).toBe(1);
    expect(
      output.match(/ui-kit\(no-hand-rolled-page-card\)/g),
      path,
    ).toHaveLength(2);
  });

  it("reports a card surface composed from className arguments or branches", () => {
    const result = lintSource(
      `function cn(...classes: string[]) {
        return classes.join(" ");
      }
      function Probe({ surface, rounded }: { surface: boolean; rounded: boolean }) {
        return <>
          <div className={cn("moto-content-surface", "rounded-2xl", "p-6")} />
          <div className={\`\${surface ? "moto-content-surface" : ""} \${rounded ? "rounded-2xl" : ""} p-6\`} />
        </>;
      }
      void Probe;`,
      "src/app/[tenant]/(protected)/rooms/page.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-page-card\)/g)).toHaveLength(2);
  });

  it("accepts control surfaces in pages and the chain in components", () => {
    const pageControl = lintSource(
      `function Probe() {
        return <span className="moto-content-surface h-9 rounded-full border px-3" />;
      }
      void Probe;`,
      "src/app/[tenant]/(protected)/rooms/page.tsx",
    );
    const componentCard = lintSource(
      `function Probe() {
        return <div className="moto-content-surface rounded-2xl border p-6" />;
      }
      void Probe;`,
      "src/components/rooms/room-card.tsx",
    );

    for (const result of [pageControl, componentCard]) {
      const output = `${result.stdout}${result.stderr}`;
      expect(result.status, output).toBe(0);
      expect(output).not.toContain("no-hand-rolled-page-card");
    }
  });
});

describe("ui-kit/no-tabs-as-value-switcher", () => {
  const tabsSource = (
    label: string,
  ) => `import { TabsTrigger } from "~/components/ui/tabs";
    function Probe() {
      return <TabsTrigger value="probe">${label}</TabsTrigger>;
    }
    void Probe;`;

  it("reports a value choice on ui/Tabs", () => {
    for (const label of ["Woche", "Monat", "Woche A", "Ganzer Tag"]) {
      const result = lintSource(tabsSource(label), "src/components/probe.tsx");
      const output = `${result.stdout}${result.stderr}`;

      expect(result.status, label).toBe(1);
      expect(
        output.match(/ui-kit\(no-tabs-as-value-switcher\)/g),
        label,
      ).toHaveLength(1);
    }
  });

  it("reports nested and translated value-choice labels", () => {
    const result = lintSource(
      `import { TabsTrigger } from "~/components/ui/tabs";
      function t(key: string) {
        return key;
      }
      function Probe() {
        return <>
          <TabsTrigger value="custom"><span>Woche</span></TabsTrigger>
          <TabsTrigger value="custom">{t("week")}</TabsTrigger>
        </>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-tabs-as-value-switcher\)/g)).toHaveLength(
      2,
    );
  });

  it("reports dynamic tabs without content panels", () => {
    const result = lintSource(
      `import {
        Tabs as ValueTabs,
        TabsList as ValueTabsList,
        TabsTrigger as ValueTabsTrigger,
      } from "~/components/ui/tabs";
      const tab = { id: "custom", label: "Eigene Ansicht" };
      function Probe() {
        return <ValueTabs><ValueTabsList>
          <ValueTabsTrigger value={tab.id}>{tab.label}</ValueTabsTrigger>
        </ValueTabsList></ValueTabs>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output).toContain("ui-kit(no-tabs-as-value-switcher)");
  });

  it("does not count content panels in nested tab groups", () => {
    const result = lintSource(
      `import { Tabs, TabsList, TabsTrigger, TabsContent } from "~/components/ui/tabs";
      const tab = { id: "custom", label: "Eigene Ansicht" };
      function Probe() {
        return <Tabs>
          <TabsList><TabsTrigger value={tab.id}>{tab.label}</TabsTrigger></TabsList>
          <Tabs><TabsContent value="inner">Inhalt</TabsContent></Tabs>
        </Tabs>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output).toContain("ui-kit(no-tabs-as-value-switcher)");
  });

  it("accepts dynamic labels with all supported content-panel imports", () => {
    const result = lintSource(
      `import * as TabsPrimitive from "@radix-ui/react-tabs";
      import {
        Tabs,
        TabsList,
        TabsTrigger,
        TabsContent as TildeContent,
      } from "~/components/ui/tabs";
      import { TabsContent as AtContent } from "@/components/ui/tabs";
      import { TabsContent as RelativeContent } from "./ui/tabs";
      const tab = { id: "week", label: "Eigene Ansicht" };
      function Probe() {
        return <Tabs>
          <TabsList><TabsTrigger value={tab.id}>{tab.label}</TabsTrigger></TabsList>
          <TabsPrimitive.Content value={tab.id}>Inhalt</TabsPrimitive.Content>
          <TildeContent value={tab.id}>Inhalt</TildeContent>
          <AtContent value={tab.id}>Inhalt</AtContent>
          <RelativeContent value={tab.id}>Inhalt</RelativeContent>
        </Tabs>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-tabs-as-value-switcher");
  });

  it("ignores unrelated components named TabsTrigger", () => {
    const result = lintSource(
      `function TabsTrigger(_: { children: unknown }) {
        return null;
      }
      function Probe() {
        return <TabsTrigger>Woche</TabsTrigger>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-tabs-as-value-switcher");
  });

  it("accepts content tabs and leaves the other portals alone", () => {
    for (const [label, path] of [
      ["Bearbeiten", "src/components/probe.tsx"],
      ["Verlauf", "src/components/probe.tsx"],
      ["Woche", "src/components/parent/probe.tsx"],
      ["Woche", "src/components/probe.test.tsx"],
    ] as const) {
      const result = lintSource(tabsSource(label), path);
      const output = `${result.stdout}${result.stderr}`;

      expect(result.status, `${label} ${path}\n${output}`).toBe(0);
      expect(output).not.toContain("no-tabs-as-value-switcher");
    }
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
        <span {...props} />;
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
        <span {...props} />;
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
        <span {...props} />;
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

describe("ui-kit/no-raw-input", () => {
  it("reports hand-built text, checkbox and untyped inputs", () => {
    const result = lintSource(`
      function Probe() {
        return <form>
          <input aria-label="Vorname" />
          <input type="email" aria-label="E-Mail" />
          <label><input type="checkbox" /> Zustimmen</label>
        </form>;
      }
      void Probe;
    `);
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-raw-input\)/g)).toHaveLength(3);
  });

  it("keeps hidden and file inputs native", () => {
    const result = lintSource(`
      function Probe() {
        return <form>
          <input type="hidden" name="token" value="x" />
          <input type={"file"} aria-label="Datei" hidden />
        </form>;
      }
      void Probe;
    `);
    const output = `${result.stdout}${result.stderr}`;

    expect(output).not.toContain("no-raw-input");
  });

  it("leaves the kit itself alone", () => {
    const result = lintSource(
      `
      export function Field() {
        return <input aria-label="Vorname" />;
      }
    `,
      "src/components/ui/field.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(output).not.toContain("no-raw-input");
  });
});

// #5531: Button-Ratchet. Ein Fund je Element; die Baseline hält den Ist-Stand
// je Datei fest und darf nur sinken, bewusste Ausnahmen stehen begründet in
// der Allowlist.
describe("ui-kit/no-hand-rolled-button", () => {
  const handRolled = (count: number) =>
    `function Probe() {
      return <div>${Array.from(
        { length: count },
        (_, index) =>
          `<button type="button" key="${index}" className="rounded-lg bg-gray-900 px-4 py-2 text-white">Speichern</button>`,
      ).join("")}</div>;
    }
    void Probe;`;

  it("fails a native button with button optics outside the kit", () => {
    const result = lintSource(handRolled(1), "src/components/probe.tsx");
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-button\)/g)).toHaveLength(1);
  });

  it("fails a link and a role=button element that look like buttons", () => {
    const result = lintSource(
      `import Link from "next/link";
      function Probe() {
        return (
          <>
            <Link href="/a" className="inline-flex h-10 items-center rounded-lg px-4 ring-1 ring-gray-300">A</Link>
            <div role="button" tabIndex={0} className="rounded-md bg-white px-3 py-2">B</div>
          </>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-button\)/g)).toHaveLength(2);
  });

  it("follows aliased link imports", () => {
    const result = lintSource(
      `import AppLink from "next/link";
      import { NavLink as SidebarLink } from "~/components/ui/nav-link";
      import ProgressLink from "~/components/ui/navigation-link";
      function Probe() {
        return <>
          <AppLink href="/a" className="rounded-lg bg-gray-900 px-4 py-2">A</AppLink>
          <SidebarLink href="/b" className="rounded-lg bg-gray-900 px-4 py-2">B</SidebarLink>
          <ProgressLink href="/c" className="rounded-lg bg-gray-900 px-4 py-2">C</ProgressLink>
        </>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-button\)/g)).toHaveLength(3);
  });

  it("does not read border sides or widths as a colour", () => {
    const result = lintSource(
      `function Probe() {
        return (
          <>
            <button type="button" className="border-b-2 rounded-t px-4">A</button>
            <button type="button" className="border-l-4 rounded-md py-2">B</button>
            <button type="button" className="bg-opacity-50 rounded-md p-2">C</button>
          </>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(`${result.stdout}${result.stderr}`).not.toContain(
      "no-hand-rolled-button",
    );
  });

  it("follows a same-file class constant", () => {
    const result = lintSource(
      `const chip = "rounded-full bg-gray-100 px-3 py-1";
      function Probe() {
        return <button type="button" className={chip}>A</button>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(`${result.stdout}${result.stderr}`).toContain(
      "ui-kit(no-hand-rolled-button)",
    );
  });

  it("accepts plain text buttons and kit-styled links", () => {
    const result = lintSource(
      `import Link from "next/link";
      import { buttonClassName } from "~/components/ui/button";
      function Probe() {
        return (
          <>
            <button type="button" className="text-sm text-gray-600 hover:underline">Mehr</button>
            <Link href="/a" className={buttonClassName({ variant: "outline", size: "md", className: "w-full" })}>A</Link>
          </>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(`${result.stdout}${result.stderr}`).not.toContain(
      "no-hand-rolled-button",
    );
  });

  it("keeps the shrink-only baseline: equal and fewer pass, more fails", () => {
    const [file, count] = Object.entries(
      buttonRatchetBaseline["no-hand-rolled-button"] ?? {},
    )[0]!;
    // Andere Klassenketten derselben Datei fallen weg: Die Sonde ersetzt sie.
    const equal = lintSource(handRolled(count), file);
    expect(`${equal.stdout}${equal.stderr}`).not.toContain(
      "no-hand-rolled-button",
    );

    const more = lintSource(handRolled(count + 1), file);
    expect(more.status).toBe(1);
    expect(`${more.stdout}${more.stderr}`).toContain(
      "ui-kit(no-hand-rolled-button)",
    );

    // Weniger Funde sind im Lint erlaubt; die gesenkte Baseline verlangt der
    // Test „button ratchet baseline“ unten.
    const fewer = lintSource(handRolled(count - 1), file);
    expect(`${fewer.stdout}${fewer.stderr}`).not.toContain(
      "no-hand-rolled-button",
    );
  });

  it("allows only the documented allowlist location", () => {
    const entry = buttonRatchetAllowlist.find(
      (candidate) => candidate.rule === "no-hand-rolled-button",
    )!;
    expect(entry.reason.length).toBeGreaterThan(10);
    const line = entry.lines[0]!;
    const sourceAtLine = (targetLine: number) =>
      `${"\n".repeat(targetLine - 2)}function Probe() {\nreturn <button type="button" className="rounded-lg bg-gray-900 px-4 py-2">A</button>;\n}\nvoid Probe;`;

    const allowed = lintSource(sourceAtLine(line), entry.file);
    expect(`${allowed.stdout}${allowed.stderr}`).not.toContain(
      "no-hand-rolled-button",
    );

    const moved = lintSource(sourceAtLine(line + 1), entry.file);
    expect(moved.status).toBe(1);
    expect(`${moved.stdout}${moved.stderr}`).toContain(
      "ui-kit(no-hand-rolled-button)",
    );
  });
});

describe("ui-kit/no-button-override", () => {
  it("fails sizing, colour and scale classes on a kit Button", () => {
    const result = lintSource(
      `import { Button, ButtonLink } from "~/components/ui/button";
      function Probe() {
        return (
          <>
            <Button size="md" className="min-h-11 rounded-xl">A</Button>
            <ButtonLink href="/b" className="font-semibold">B</ButtonLink>
            <Button className="hover:scale-105">C</Button>
          </>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-button-override\)/g)).toHaveLength(3);
  });

  it("follows aliased kit button imports", () => {
    const result = lintSource(
      `import { Button as ActionButton, ButtonLink as ActionLink, buttonClassName as kitButtonClassName } from "~/components/ui/button";
      const className = kitButtonClassName({ variant: "primary", size: "md", className: "h-10" });
      function Probe() {
        return <>
          <ActionButton className="h-10">A</ActionButton>
          <ActionLink href="/a" className="rounded-lg">B</ActionLink>
        </>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-button-override\)/g)).toHaveLength(3);
  });

  it("fails optics passed through buttonClassName", () => {
    const result = lintSource(
      `import { buttonClassName } from "~/components/ui/button";
      export const cls = buttonClassName({ variant: "primary", size: "md", className: "h-10 shadow-sm" });`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(`${result.stdout}${result.stderr}`).toContain(
      "ui-kit(no-button-override)",
    );
  });

  it("accepts pure layout classes", () => {
    const result = lintSource(
      `import { Button } from "~/components/ui/button";
      function Probe() {
        return <Button size="md" className="hidden w-full flex-1 shrink-0 gap-2 self-end mt-2 md:flex justify-start">A</Button>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(`${result.stdout}${result.stderr}`).not.toContain(
      "no-button-override",
    );
  });
});

describe("button ratchet baseline", () => {
  // Die Zahl darf nur sinken: Nach einem Abbau muss die Baseline mitsinken,
  // sonst könnte die nächste Änderung den Abbau unbemerkt wieder auffüllen.
  it("matches the current findings exactly", () => {
    const result = spawnSync(
      process.execPath,
      [resolve("scripts/ui-kit-button-baseline-update.mjs"), "--check"],
      { encoding: "utf8" },
    );

    expect(result.status, `${result.stdout}${result.stderr}`).toBe(0);
  }, 120_000);

  it("refuses to raise the baseline in write mode", () => {
    const directory = mkdtempSync(join(tmpdir(), "button-baseline-"));
    temporaryDirectories.push(directory);
    const baselinePath = join(directory, "ui-kit-button-baseline.json");
    const initial =
      '{\n  "no-hand-rolled-button": {},\n  "no-button-override": {}\n}\n';
    writeFileSync(baselinePath, initial);

    const result = spawnSync(
      process.execPath,
      [resolve("scripts/ui-kit-button-baseline-update.mjs")],
      {
        encoding: "utf8",
        env: { ...process.env, UI_KIT_BUTTON_BASELINE_PATH: baselinePath },
      },
    );

    expect(result.status).toBe(1);
    expect(`${result.stdout}${result.stderr}`).toContain("darf nur sinken");
    expect(readFileSync(baselinePath, "utf8")).toBe(initial);
  }, 120_000);

  it("names only files that exist and positive counts", () => {
    for (const [rule, files] of Object.entries(buttonRatchetBaseline)) {
      for (const [file, count] of Object.entries(files)) {
        expect(existsSync(resolve(file)), `${rule}: ${file}`).toBe(true);
        expect(count).toBeGreaterThan(0);
      }
    }
    for (const entry of buttonRatchetAllowlist) {
      expect(existsSync(resolve(entry.file)), entry.file).toBe(true);
    }
  });
});
