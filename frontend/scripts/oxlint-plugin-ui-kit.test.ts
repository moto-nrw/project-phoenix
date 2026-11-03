import { afterAll, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

import {
  buttonRatchetAllowlist,
  buttonRatchetBaseline,
} from "./ui-kit-button-baseline.mjs";
import {
  lintSource as lintProbe,
  removeProbeDirectories,
} from "./oxlint-probe";

const temporaryDirectories: string[] = [];

function lintSource(source: string, relativePath = "probe.tsx") {
  return lintProbe(source, relativePath);
}

describe.concurrent("ui-kit/no-hand-rolled-overlay", () => {
  it("reports the first hand-rolled overlay in a formerly baselined file", async () => {
    const result = await lintSource(
      `function Probe() {
        return <div className="fixed inset-0">Overlay</div>;
      }
      void Probe;`,
      "src/components/background-wrapper.tsx",
    );
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output).toContain("ui-kit(no-hand-rolled-overlay)");
  });
});

describe.concurrent("ui-kit/no-tiny-text", () => {
  it("rejects a legacy token reintroduced at a different source location", async () => {
    const result = await lintSource(
      `function Probe() {
        return <span className="text-[11px]">Probe</span>;
      }
      void Probe;`,
      "src/app/[tenant]/(protected)/calendar/page.tsx",
    );
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output).toContain("ui-kit(no-tiny-text)");
  });

  it("rejects an additional legacy token on the same baseline line", async () => {
    const result = await lintSource(
      `${"\n".repeat(31)}const className = "text-[10px] text-[10px]";`,
      "src/components/ui/avatar.tsx",
    );
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-tiny-text\)/g)).toHaveLength(1);
  });
});

async function ruleHits(
  source: string,
  rule: string,
  relativePath = "src/probe.tsx",
) {
  const { output } = await lintSource(source, relativePath);
  return {
    output,
    count:
      output.match(new RegExp(String.raw`ui-kit\(${rule}\)`, "g"))?.length ?? 0,
  };
}

describe.concurrent("ui-kit/no-dead-surface-utility", () => {
  it("reports colour, shadow, ring and blur utilities beside a kit surface", async () => {
    const { count, output } = await ruleHits(
      `
      function Probe() {
        return <>
          <div className="moto-content-surface border-moto-red/30 bg-moto-red/5 rounded-xl border" />
          <div className="moto-content-surface rounded-2xl border shadow-sm" />
          <div className="moto-content-surface rounded-2xl border backdrop-blur-md" />
          <div className="moto-content-surface rounded-2xl border focus-visible:ring-2" />
          <button className="moto-content-surface h-10 hover:border-gray-300" />
          <div className="moto-popover-surface rounded-xl border shadow-xl" />
        </>;
      }
      void Probe;
    `,
      "no-dead-surface-utility",
    );

    expect(count, output).toBe(6);
  });

  it("follows conditional className branches", async () => {
    const { count, output } = await ruleHits(
      `
      function Probe({ href }: { href?: string }) {
        return <div className={\`moto-content-surface rounded-2xl border \${href ? "group-hover:shadow-md" : ""}\`} />;
      }
      void Probe;
    `,
      "no-dead-surface-utility",
    );

    expect(count, output).toBe(1);
  });

  it("follows const values passed to a className composer", async () => {
    const { count, output } = await ruleHits(
      `
      declare const cn: (...classes: string[]) => string;
      const surface = "moto-content-surface";
      const focus = "focus-visible:ring-2";
      function Probe() {
        return <div className={cn(surface, focus)} />;
      }
      void Probe;
    `,
      "no-dead-surface-utility",
    );

    expect(count, output).toBe(1);
  });

  it("accepts layout, border widths, important overrides and popover blur", async () => {
    const { count, output } = await ruleHits(
      `
      function Probe() {
        return <>
          <div className="moto-content-surface rounded-2xl border border-b-2 p-5 divide-y divide-gray-100" />
          <div className="moto-content-surface rounded-2xl border border-dashed group-hover:shadow-md!" />
          <div className="moto-popover-surface rounded-xl border backdrop-blur-sm" />
          <div className="rounded-xl border border-gray-200 bg-gray-50 shadow-sm" />
        </>;
      }
      void Probe;
    `,
      "no-dead-surface-utility",
    );

    expect(count, output).toBe(0);
  });
});

describe.concurrent("ui-kit/no-dynamic-color-class", () => {
  it("reports colour utilities assembled from a template expression", async () => {
    const { count, output } = await ruleHits(
      `
      function Probe({ color }: { color: string }) {
        return <>
          <svg className={\`h-4 w-4 text-\${color}\`} />
          <div className={\`hover:bg-\${color}/10 border-t-\${color}\`} />
          <div className={\`ring-\${color}\`} />
        </>;
      }
      void Probe;
    `,
      "no-dynamic-color-class",
    );

    expect(count, output).toBe(4);
  });

  it("reports a const passed to className", async () => {
    const { count, output } = await ruleHits(
      `
      function Probe({ color }: { color: string }) {
        const iconClass = \`h-4 w-4 text-\${color}\`;
        return <svg className={iconClass} />;
      }
      void Probe;
    `,
      "no-dynamic-color-class",
    );

    expect(count, output).toBe(1);
  });

  it("reports a mutable alias passed to className", async () => {
    const { count, output } = await ruleHits(
      `
      function Probe({ color }: { color: string }) {
        let iconClass = \`h-4 w-4 text-\${color}\`;
        return <svg className={iconClass} />;
      }
      void Probe;
    `,
      "no-dynamic-color-class",
    );

    expect(count, output).toBe(1);
  });

  it("ignores non-class template literals and accepts static class strings", async () => {
    const { count, output } = await ruleHits(
      `
      function Probe({ id, cls }: { id: string; cls: string }) {
        return <>
          <textarea id={\`text-\${id}\`} />
          <p>{\`bg-\${id}\`}</p>
          <div className={id === \`ring-\${id}\` ? "selected" : ""} />
          <div className={\`text-sm \${cls}\`} />
        </>;
      }
      void Probe;
    `,
      "no-dynamic-color-class",
    );

    expect(count, output).toBe(0);
  });
});

describe.concurrent("ui-kit/no-white-on-warm-fill", () => {
  it("reports white text on the orange and amber fills", async () => {
    const { count, output } = await ruleHits(
      `
      import { LOCATION_COLORS, MOTO_COLOR_PALETTE } from "~/lib/location-helper";
      function Probe() {
        return <>
          <div className="px-4 text-white" style={{ backgroundColor: LOCATION_COLORS.SCHOOLYARD }} />
          <div className="text-white" style={{ background: LOCATION_COLORS.WARNING }} />
          <span className="bg-moto-orange text-white" />
          <span className="bg-moto-amber text-white" />
          <div className="text-white" style={{ backgroundColor: MOTO_COLOR_PALETTE.orange.base }} />
        </>;
      }
      void Probe;
    `,
      "no-white-on-warm-fill",
    );

    expect(count, output).toBe(5);
  });

  it("accepts dark text on warm fills and white on the strong tones", async () => {
    const { count, output } = await ruleHits(
      `
      import { LOCATION_COLORS } from "~/lib/location-helper";
      function Probe() {
        return <>
          <div className="text-gray-900" style={{ backgroundColor: LOCATION_COLORS.SCHOOLYARD }} />
          <span className="bg-moto-orange-strong text-white" />
          <span className="bg-moto-orange-soft text-moto-orange-strong" />
          <div className="text-white" style={{ backgroundColor: LOCATION_COLORS.SICK }} />
        </>;
      }
      void Probe;
    `,
      "no-white-on-warm-fill",
    );

    expect(count, output).toBe(0);
  });

  it("reports warm fills and white text composed from const values", async () => {
    const { count, output } = await ruleHits(
      `
      declare const cn: (...classes: string[]) => string;
      const fill = "bg-moto-orange";
      const text = "text-white";
      function Probe() {
        return <div className={cn(fill, text)} />;
      }
      void Probe;
    `,
      "no-white-on-warm-fill",
    );

    expect(count, output).toBe(1);
  });
});

describe.concurrent("ui-kit/no-hand-rolled-surface", () => {
  it("reports a hand-built card surface anywhere outside the kit", async () => {
    const result = await lintSource(
      `
      function Probe() {
        return <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm" />;
      }
      void Probe;
    `,
      "src/probe.tsx",
    );
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-surface\)/g)).toHaveLength(1);
  });

  it("reports a surface assembled across conditional className branches", async () => {
    const result = await lintSource(
      `
      function Probe({ active }: { active: boolean }) {
        return <div className={\`rounded-xl border p-3 \${active ? "border-gray-900 bg-gray-50" : "border-gray-200 bg-white"}\`} />;
      }
      void Probe;
    `,
      "src/probe.tsx",
    );
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-surface\)/g)).toHaveLength(1);
  });

  it("accepts the kit surfaces and non-card shapes", async () => {
    const result = await lintSource(
      `
      function Probe() {
        return <>
          <div className="moto-content-surface rounded-2xl border p-4" />
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
    const output = result.output;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-hand-rolled-surface");
  });

  it("ignores prefixed tokens, frameless borders and translucent fills", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-hand-rolled-surface");
  });

  it("honours the documented oxlint-disable-next-line escape hatch", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-hand-rolled-surface");
  });
});

describe.concurrent("ui-kit/no-rounded-3xl", () => {
  it("reports rounded-3xl, also inside the kit", async () => {
    const paths = ["src/probe.tsx", "src/components/ui/probe.tsx"];
    const results = await Promise.all(
      paths.map((path) =>
        lintSource(
          `function Probe() {
          return <div className="rounded-3xl p-4" />;
        }
        void Probe;`,
          path,
        ),
      ),
    );
    for (const [index, path] of paths.entries()) {
      const result = results[index]!;
      const output = result.output;

      expect(result.status, path).toBe(1);
      expect(output.match(/ui-kit\(no-rounded-3xl\)/g), path).toHaveLength(1);
    }
  });

  it("accepts the canonical radius and exempts test files", async () => {
    const [canonical, fixture] = await Promise.all([
      lintSource(
        `function Probe() {
        return <div className="moto-content-surface rounded-2xl border p-4" />;
      }
      void Probe;`,
        "src/probe.tsx",
      ),
      lintSource(
        `const className = "rounded-3xl";
      void className;`,
        "src/probe.test.tsx",
      ),
    ]);

    for (const result of [canonical, fixture]) {
      const output = result.output;
      expect(result.status, output).toBe(0);
      expect(output).not.toContain("no-rounded-3xl");
    }
  });
});

describe.concurrent("ui-kit/no-tenant-kicker", () => {
  const kickerSource = `function SectionCard(_: { kicker?: string; title: string }) {
      return null;
    }
    function Probe() {
      return <SectionCard kicker="Verwaltung" title="Räume" />;
    }
    void Probe;`;

  it("reports a kicker in the tenant portal", async () => {
    const result = await lintSource(
      kickerSource,
      "src/app/[tenant]/(protected)/rooms/page.tsx",
    );
    const output = result.output;

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
  ])("leaves %s alone", async (path) => {
    const result = await lintSource(kickerSource, path);
    const output = result.output;

    expect(result.status, `${path}\n${output}`).toBe(0);
    expect(output).not.toContain("no-tenant-kicker");
  });
});

describe.concurrent("ui-kit/no-hand-rolled-page-card", () => {
  it.each([
    "src/app/[tenant]/(protected)/page.tsx",
    "src/app/[tenant]/(protected)/loading.tsx",
    "src/app/[tenant]/(protected)/rooms/page.tsx",
    "src/app/[tenant]/(protected)/rooms/[id]/loading.tsx",
    // Das Träger-Portal rendert dasselbe Gerüst (#5146).
    "src/app/traeger/page.tsx",
    "src/app/traeger/ferien/[id]/page.tsx",
  ])("reports the kit card chain spelled out in %s", async (path) => {
    const result = await lintSource(
      `function Probe({ wide }: { wide: boolean }) {
          return <>
            <div className="moto-content-surface rounded-2xl border p-6" />
            <div className={\`rounded-2xl border p-4 moto-content-surface \${wide ? "w-full" : ""}\`} />
          </>;
        }
        void Probe;`,
      path,
    );
    const output = result.output;

    expect(result.status, path).toBe(1);
    expect(
      output.match(/ui-kit\(no-hand-rolled-page-card\)/g),
      path,
    ).toHaveLength(2);
  });

  it("reports a card surface composed from className arguments or branches", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-page-card\)/g)).toHaveLength(2);
  });

  it("accepts control surfaces in pages and the chain in components", async () => {
    const [pageControl, componentCard] = await Promise.all([
      lintSource(
        `function Probe() {
        return <span className="moto-content-surface h-9 rounded-full border px-3" />;
      }
      void Probe;`,
        "src/app/[tenant]/(protected)/rooms/page.tsx",
      ),
      lintSource(
        `function Probe() {
        return <div className="moto-content-surface rounded-2xl border p-6" />;
      }
      void Probe;`,
        "src/components/rooms/room-card.tsx",
      ),
    ]);

    for (const result of [pageControl, componentCard]) {
      const output = result.output;
      expect(result.status, output).toBe(0);
      expect(output).not.toContain("no-hand-rolled-page-card");
    }
  });
});

describe.concurrent("ui-kit/no-tabs-as-value-switcher", () => {
  const tabsSource = (
    label: string,
  ) => `import { TabsTrigger } from "~/components/ui/tabs";
    function Probe() {
      return <TabsTrigger value="probe">${label}</TabsTrigger>;
    }
    void Probe;`;

  it("reports a value choice on ui/Tabs", async () => {
    const labels = ["Woche", "Monat", "Woche A", "Ganzer Tag"];
    const results = await Promise.all(
      labels.map((label) =>
        lintSource(tabsSource(label), "src/components/probe.tsx"),
      ),
    );
    for (const [index, label] of labels.entries()) {
      const result = results[index]!;
      const output = result.output;

      expect(result.status, label).toBe(1);
      expect(
        output.match(/ui-kit\(no-tabs-as-value-switcher\)/g),
        label,
      ).toHaveLength(1);
    }
  });

  it("reports nested and translated value-choice labels", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-tabs-as-value-switcher\)/g)).toHaveLength(
      2,
    );
  });

  it("reports dynamic tabs without content panels", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output).toContain("ui-kit(no-tabs-as-value-switcher)");
  });

  it("does not count content panels in nested tab groups", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output).toContain("ui-kit(no-tabs-as-value-switcher)");
  });

  it("accepts dynamic labels with all supported content-panel imports", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-tabs-as-value-switcher");
  });

  it("ignores unrelated components named TabsTrigger", async () => {
    const result = await lintSource(
      `function TabsTrigger(_: { children: unknown }) {
        return null;
      }
      function Probe() {
        return <TabsTrigger>Woche</TabsTrigger>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const output = result.output;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-tabs-as-value-switcher");
  });

  it("accepts content tabs and leaves the other portals alone", async () => {
    const cases = [
      ["Bearbeiten", "src/components/probe.tsx"],
      ["Verlauf", "src/components/probe.tsx"],
      ["Woche", "src/components/parent/probe.tsx"],
      ["Woche", "src/components/probe.test.tsx"],
    ] as const;
    const results = await Promise.all(
      cases.map(([label, path]) => lintSource(tabsSource(label), path)),
    );
    for (const [index, [label, path]] of cases.entries()) {
      const result = results[index]!;
      const output = result.output;

      expect(result.status, `${label} ${path}\n${output}`).toBe(0);
      expect(output).not.toContain("no-tabs-as-value-switcher");
    }
  });
});

afterAll(() => {
  removeProbeDirectories();
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe.concurrent("ui-kit/require-checkbox-label", () => {
  it("reports sibling labels and aria-label-only checkboxes", async () => {
    const result = await lintSource(`
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(require-checkbox-label\)/g)).toHaveLength(2);
  });

  it("accepts a checkbox inside a label-shaped ChoiceTile, not a button one", async () => {
    const result = await lintSource(`
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(require-checkbox-label\)/g)).toHaveLength(1);
  });

  it("accepts a checkbox nested anywhere inside its label", async () => {
    const result = await lintSource(`
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
    const output = result.output;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("require-checkbox-label");
  });
});

describe.concurrent("ui-kit/no-raw-input", () => {
  it("reports hand-built text, checkbox and untyped inputs", async () => {
    const result = await lintSource(`
      function Probe() {
        return <form>
          <input aria-label="Vorname" />
          <input type="email" aria-label="E-Mail" />
          <label><input type="checkbox" /> Zustimmen</label>
        </form>;
      }
      void Probe;
    `);
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-raw-input\)/g)).toHaveLength(3);
  });

  it("keeps hidden and file inputs native", async () => {
    const result = await lintSource(`
      function Probe() {
        return <form>
          <input type="hidden" name="token" value="x" />
          <input type={"file"} aria-label="Datei" hidden />
        </form>;
      }
      void Probe;
    `);
    const output = result.output;

    expect(output).not.toContain("no-raw-input");
  });

  it("leaves the kit itself alone", async () => {
    const result = await lintSource(
      `
      export function Field() {
        return <input aria-label="Vorname" />;
      }
    `,
      "src/components/ui/field.tsx",
    );
    const output = result.output;

    expect(output).not.toContain("no-raw-input");
  });
});

// #5531: Button-Ratchet. Ein Fund je Element; die Baseline hält den Ist-Stand
// je Datei fest und darf nur sinken, bewusste Ausnahmen stehen begründet in
// der Allowlist.
describe.concurrent("ui-kit/no-hand-rolled-button", () => {
  const handRolled = (count: number) =>
    `function Probe() {
      return <div>${Array.from(
        { length: count },
        (_, index) =>
          `<button type="button" key="${index}" className="rounded-lg bg-gray-900 px-4 py-2 text-white">Speichern</button>`,
      ).join("")}</div>;
    }
    void Probe;`;

  it("fails a native button with button optics outside the kit", async () => {
    const result = await lintSource(handRolled(1), "src/components/probe.tsx");
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-button\)/g)).toHaveLength(1);
  });

  it("fails a link and a role=button element that look like buttons", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-button\)/g)).toHaveLength(2);
  });

  it("follows aliased link imports", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-hand-rolled-button\)/g)).toHaveLength(3);
  });

  it("does not read border sides or widths as a colour", async () => {
    const result = await lintSource(
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

    expect(result.output).not.toContain("no-hand-rolled-button");
  });

  it("follows a same-file class constant", async () => {
    const result = await lintSource(
      `const chip = "rounded-full bg-gray-100 px-3 py-1";
      function Probe() {
        return <button type="button" className={chip}>A</button>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.output).toContain("ui-kit(no-hand-rolled-button)");
  });

  it("accepts plain text buttons and kit-styled links", async () => {
    const result = await lintSource(
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

    expect(result.output).not.toContain("no-hand-rolled-button");
  });

  it("keeps the shrink-only baseline: equal and fewer pass, more fails", async () => {
    const [file, count] = Object.entries(
      buttonRatchetBaseline["no-hand-rolled-button"] ?? {},
    )[0]!;
    // Andere Klassenketten derselben Datei fallen weg: Die Sonde ersetzt sie.
    const [equal, more, fewer] = await Promise.all([
      lintSource(handRolled(count), file),
      lintSource(handRolled(count + 1), file),
      lintSource(handRolled(count - 1), file),
    ]);
    expect(equal.output).not.toContain("no-hand-rolled-button");

    expect(more.status).toBe(1);
    expect(more.output).toContain("ui-kit(no-hand-rolled-button)");

    // Weniger Funde sind im Lint erlaubt; die gesenkte Baseline verlangt der
    // Test „button ratchet baseline“ unten.
    expect(fewer.output).not.toContain("no-hand-rolled-button");
  });

  it("allows only the documented allowlist location", async () => {
    const entry = buttonRatchetAllowlist.find(
      (candidate) => candidate.rule === "no-hand-rolled-button",
    )!;
    expect(entry.reason.length).toBeGreaterThan(10);
    const line = entry.lines[0]!;
    const sourceAtLine = (targetLine: number) =>
      `${"\n".repeat(targetLine - 2)}function Probe() {\nreturn <button type="button" className="rounded-lg bg-gray-900 px-4 py-2">A</button>;\n}\nvoid Probe;`;

    const [allowed, moved] = await Promise.all([
      lintSource(sourceAtLine(line), entry.file),
      lintSource(sourceAtLine(line + 1), entry.file),
    ]);
    expect(allowed.output).not.toContain("no-hand-rolled-button");

    expect(moved.status).toBe(1);
    expect(moved.output).toContain("ui-kit(no-hand-rolled-button)");
  });
});

describe.concurrent("ui-kit/no-button-override", () => {
  it("fails sizing, colour and scale classes on a kit Button", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-button-override\)/g)).toHaveLength(3);
  });

  it("follows aliased kit button imports", async () => {
    const result = await lintSource(
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
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-button-override\)/g)).toHaveLength(3);
  });

  it("fails optics passed through buttonClassName", async () => {
    const result = await lintSource(
      `import { buttonClassName } from "~/components/ui/button";
      export const cls = buttonClassName({ variant: "primary", size: "md", className: "h-10 shadow-sm" });`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-button-override)");
  });

  it("accepts pure layout classes", async () => {
    const result = await lintSource(
      `import { Button } from "~/components/ui/button";
      function Probe() {
        return <Button size="md" className="hidden w-full flex-1 shrink-0 gap-2 self-end mt-2 md:flex justify-start">A</Button>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain("no-button-override");
  });
});

describe("ui-kit/no-grey-button-fill", () => {
  const lint = async (body: string) => {
    const result = await lintSource(body, "src/components/probe.tsx");
    return {
      status: result.status,
      count: (result.output.match(/ui-kit\(no-grey-button-fill\)/g) ?? [])
        .length,
    };
  };

  it("fails a grey resting fill on native buttons, links and kit buttons", async () => {
    const result = await lint(
      `import Link from "next/link";
      import { Button, buttonClassName } from "~/components/ui/button";
      const idle = "bg-gray-100 text-gray-700 hover:bg-gray-200";
      function Probe({ open }: { open: boolean }) {
        return (
          <>
            <button type="button" className={open ? "bg-moto-red-soft" : idle}>A</button>
            <Link href="/a" className="bg-gray-200 text-gray-800">B</Link>
            <Button className="bg-gray-100">C</Button>
            <a href="/d" className={buttonClassName({ variant: "outline", size: "md", className: "bg-gray-200" })}>D</a>
          </>
        );
      }
      void Probe;`,
    );

    expect(result.status).toBe(1);
    expect(result.count).toBe(4);
  });

  it("fails gray-300 and responsive grey resting fills", async () => {
    const result = await lint(
      `function Probe() {
        return (
          <>
            <button type="button" className="bg-gray-300">A</button>
            <button type="button" className="md:bg-gray-100">B</button>
          </>
        );
      }
      void Probe;`,
    );

    expect(result.status).toBe(1);
    expect(result.count).toBe(2);
  });

  it("accepts state tints, tracks, switches and navigation rows", async () => {
    const result = await lint(
      `import { NavLink as SideLink } from "~/components/ui/nav-link";
      function Probe({ on }: { on: boolean }) {
        return (
          <>
            <button type="button" className="rounded-md px-3 py-2 hover:bg-gray-100 active:bg-gray-200">A</button>
            <div className="rounded-lg bg-gray-100 p-1"><button type="button" aria-pressed={on}>B</button></div>
            <button type="button" role="switch" aria-checked={on} className={on ? "bg-gray-900" : "bg-gray-200"} />
            <button type="button" aria-pressed={on} className={on ? "bg-gray-300" : "bg-gray-100"}>C</button>
            <button type="button" aria-selected={on} className="bg-gray-100">D</button>
            <a href="/c" aria-current={on ? "page" : undefined} className={on ? "bg-gray-100" : ""}>C</a>
            <SideLink href="/d" className="bg-gray-100">D</SideLink>
            <button type="button" className="bg-gray-50 rounded-md px-2">E</button>
          </>
        );
      }
      void Probe;`,
    );

    expect(result.count).toBe(0);
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

  // Ein zweiter Volllauf über src kostet so viel wie der Test oben. Die
  // Schreibsperre läuft deshalb in einem Sandbox-Frontend: das Skript als
  // Kopie der echten Datei, das echte oxlint und das echte ui-kit-Plugin, ein
  // src mit genau den Allowlist-Fundstellen und einem Fund mehr.
  it("refuses to raise the baseline in write mode", () => {
    const directory = mkdtempSync(join(tmpdir(), "button-baseline-"));
    temporaryDirectories.push(directory);
    mkdirSync(join(directory, "scripts"));
    for (const file of [
      "ui-kit-button-baseline-update.mjs",
      "ui-kit-button-baseline.mjs",
      "ui-kit-button-baseline.json",
    ]) {
      copyFileSync(resolve("scripts", file), join(directory, "scripts", file));
    }
    symlinkSync(resolve("node_modules"), join(directory, "node_modules"));
    writeFileSync(
      join(directory, ".oxlintrc.json"),
      JSON.stringify({
        categories: { correctness: "off", suspicious: "off", perf: "off" },
        jsPlugins: [resolve("scripts/oxlint-plugin-ui-kit.mjs")],
        rules: {
          "ui-kit/no-hand-rolled-button": "error",
          "ui-kit/no-button-override": "error",
          "ui-kit/no-grey-button-fill": "error",
        },
      }),
    );
    // Eine Ausnahme für die graue Ruhefläche steht auf derselben Zeile wie
    // die für den handgebauten Knopf; ihr Fund trifft deshalb beide Regeln.
    const findingAt: Record<string, string> = {
      "no-hand-rolled-button": `<button type="button" className="rounded-lg bg-gray-900 px-4 py-2">A</button>`,
      "no-button-override": `<Button className="h-10">A</Button>`,
      "no-grey-button-fill": `<button type="button" className="rounded-lg bg-gray-100 px-4 py-2">A</button>`,
    };
    const sources = new Map<string, string[]>();
    for (const entry of buttonRatchetAllowlist) {
      const lines = sources.get(entry.file) ?? [
        `import { Button } from "~/components/ui/button";`,
      ];
      for (const line of entry.lines) {
        while (lines.length < line) lines.push("");
        lines[line - 1] =
          `export const probe${line} = ${findingAt[entry.rule]};`;
      }
      sources.set(entry.file, lines);
    }
    // Der zusätzliche Fund liegt in einer Datei, die die echte Baseline schon
    // zählt, und bleibt innerhalb dieser Zahl. Liest das Skript versehentlich
    // die Standard-Baseline statt UI_KIT_BUTTON_BASELINE_PATH, sieht es keine
    // Erhöhung, schreibt und endet mit 0; dann wird dieser Test rot.
    const [baselinedFile] = Object.keys(
      buttonRatchetBaseline["no-hand-rolled-button"] ?? {},
    ).filter((file) => file.endsWith(".tsx"));
    expect(baselinedFile).toBeDefined();
    const baselinedLines = sources.get(baselinedFile!) ?? [
      `import { Button } from "~/components/ui/button";`,
    ];
    baselinedLines.push(
      `export const probeExtra = ${findingAt["no-hand-rolled-button"]};`,
    );
    sources.set(baselinedFile!, baselinedLines);
    for (const [file, lines] of sources) {
      mkdirSync(dirname(join(directory, file)), { recursive: true });
      writeFileSync(join(directory, file), `${lines.join("\n")}\n`);
    }
    const baselinePath = join(directory, "ui-kit-button-baseline.json");
    const initial =
      '{\n  "no-hand-rolled-button": {},\n  "no-button-override": {}\n}\n';
    writeFileSync(baselinePath, initial);

    const result = spawnSync(
      process.execPath,
      [join(directory, "scripts/ui-kit-button-baseline-update.mjs")],
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
