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
import { pillRatchetBaseline } from "./ui-kit-pill-baseline.mjs";
import { importantOverrideBaseline } from "./ui-kit-important-override-baseline.mjs";
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

describe.concurrent("ui-kit/no-hand-rolled-icon-tile", () => {
  it("reports grey and tinted icon tiles, also across className branches", async () => {
    const { count, output } = await ruleHits(
      `
      declare const BellIcon: () => JSX.Element;
      function Probe({ icon, done }: { icon: JSX.Element; done: boolean }) {
        return <>
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 shadow-sm">
            <BellIcon />
          </span>
          <div className="bg-moto-red/10 text-moto-red flex h-9 w-9 items-center justify-center rounded-lg">{icon}</div>
          <span className={\`flex h-8 w-8 items-center justify-center rounded-xl \${done ? "bg-moto-green/15" : "bg-gray-100"}\`}>
            <BellIcon />
          </span>
          <span className="relative inline-flex size-10 items-center justify-center rounded-xl bg-gray-100">
            <BellIcon />
          </span>
        </>;
      }
      void Probe;
    `,
      "no-hand-rolled-icon-tile",
    );

    expect(count, output).toBe(4);
  });

  it("reports a hand-rolled tile inside the kit outside IconTile", async () => {
    const { count, output } = await ruleHits(
      `
      function Probe({ icon }: { icon: JSX.Element }) {
        return <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-gray-100">{icon}</div>;
      }
      void Probe;
    `,
      "no-hand-rolled-icon-tile",
      "src/components/ui/probe-card.tsx",
    );

    expect(count, output).toBe(1);
  });

  it("accepts text circles, buttons, the kit tile and other sizes", async () => {
    const { count, output } = await ruleHits(
      `
      declare const BellIcon: () => JSX.Element;
      declare const IconTile: (props: { icon: JSX.Element }) => JSX.Element;
      function Probe({ initials }: { initials: string }) {
        return <>
          <div className="flex h-9 w-9 items-center justify-center rounded-full bg-gray-100 text-xs">{initials}</div>
          <span className="flex h-8 w-8 items-center justify-center rounded-full bg-gray-100">3</span>
          <span className="flex h-8 w-8 items-center justify-center rounded-full bg-gray-100"><span>3</span></span>
          <span className="flex h-8 w-8 items-center justify-center rounded-full bg-gray-100"><span>{initials}</span></span>
          <button type="button" className="flex h-10 w-10 items-center justify-center rounded-full bg-gray-100"><BellIcon /></button>
          <span className="flex h-10 w-10 items-center justify-center rounded-full hover:bg-gray-100"><BellIcon /></span>
          <span className="flex h-16 w-16 items-center justify-center rounded-full bg-gray-100"><BellIcon /></span>
          <div className="flex h-10 w-full items-center justify-center rounded-lg bg-gray-100"><BellIcon /></div>
          <IconTile icon={<BellIcon />} />
        </>;
      }
      void Probe;
    `,
      "no-hand-rolled-icon-tile",
    );

    expect(count, output).toBe(0);
  });

  it("reports an icon wrapped in a presentational element", async () => {
    const { count, output } = await ruleHits(
      `
      declare const BellIcon: () => JSX.Element;
      function Probe() {
        return <>
          <span className="flex h-9 w-9 items-center justify-center rounded-full bg-gray-100"><span><BellIcon /></span></span>
          <span className="flex h-9 w-9 items-center justify-center rounded-full bg-gray-100"><span>{<BellIcon />}</span></span>
        </>;
      }
      void Probe;
    `,
      "no-hand-rolled-icon-tile",
    );

    expect(count, output).toBe(2);
  });
});

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

describe.concurrent("ui-kit/no-rounded-3xl per side", () => {
  it("reports the per-side and per-corner forms (#5579)", async () => {
    const result = await lintSource(
      `function Probe() {
        return (
          <>
            <div className="rounded-t-3xl" />
            <div className="sm:rounded-br-3xl" />
          </>
        );
      }
      void Probe;`,
      "src/components/ui/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output.match(/ui-kit\(no-rounded-3xl\)/g)).toHaveLength(2);
  });

  it("accepts rounded-t-2xl", async () => {
    const result = await lintSource(
      `function Probe() {
        return <div className="rounded-t-2xl" />;
      }
      void Probe;`,
      "src/components/ui/probe.tsx",
    );

    expect(result.status, result.output).toBe(0);
    expect(result.output).not.toContain("no-rounded-3xl");
  });
});

describe.concurrent("ui-kit/no-shadcn-tokens", () => {
  it("reports shadcn theme tokens, also inside the kit", async () => {
    const result = await lintSource(
      `function Probe() {
        return (
          <div className="bg-background focus-visible:ring-ring">
            <span className="text-muted-foreground" />
          </div>
        );
      }
      void Probe;`,
      "src/components/ui/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output.match(/ui-kit\(no-shadcn-tokens\)/g)).toHaveLength(3);
  });

  it("accepts the kit's white and gray scale", async () => {
    const result = await lintSource(
      `function Probe() {
        return <div className="bg-white text-gray-500 ring-moto-blue border-gray-200" />;
      }
      void Probe;`,
      "src/probe.tsx",
    );

    expect(result.status, result.output).toBe(0);
    expect(result.output).not.toContain("no-shadcn-tokens");
  });
});

describe.concurrent("ui-kit/no-raw-z-index", () => {
  it("reports a stacking number inside the kit", async () => {
    const result = await lintSource(
      `function Probe() {
        return <div className="fixed inset-0 z-[9999]" />;
      }
      void Probe;`,
      "src/components/ui/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-raw-z-index)");
  });

  it("accepts Z_LAYER itself and leaves files outside the kit alone", async () => {
    const [layers, outside] = await Promise.all([
      lintSource(
        `export const Z_LAYER = { modal: "z-[9999]" } as const;`,
        "src/components/ui/overlay-styles.ts",
      ),
      lintSource(
        `export const layer = "z-[60]";`,
        "src/components/students/probe.ts",
      ),
    ]);

    for (const result of [layers, outside]) {
      expect(result.status, result.output).toBe(0);
      expect(result.output).not.toContain("no-raw-z-index");
    }
  });
});

describe.concurrent("ui-kit/no-tenant-kicker", () => {
  const kickerSource = `// oxlint-disable-next-line ui-kit/no-shadow-kit-name
function SectionCard(_: { kicker?: string; title: string }) {
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

describe.concurrent("ui-kit/no-shadow-kit-name", () => {
  it("reports a local function and a local constant named like a kit export", async () => {
    const result = await lintSource(
      `function StatCard() {
        return null;
      }
      function Probe() {
        const exportMenuItems = [];
        return <StatCard items={exportMenuItems} />;
      }
      void Probe;`,
      "src/components/import/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output.match(/ui-kit\(no-shadow-kit-name\)/g)).toHaveLength(
      2,
    );
    expect(result.output).toContain("StatCard");
  });

  it("also knows the default export of a kit file", async () => {
    const result = await lintSource(
      `function NavigationLink() {
        return null;
      }
      void NavigationLink;`,
      "src/components/import/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("no-shadow-kit-name");
  });

  it("allows other names, the kit itself and test files", async () => {
    const other = await lintSource(
      `function StatusChip() {
        return null;
      }
      void StatusChip;`,
      "src/components/import/probe.tsx",
    );
    const kit = await lintSource(
      `export function StatCard() {
        return null;
      }`,
      "src/components/ui/stat-card.tsx",
    );
    const testFile = await lintSource(
      `function StatCard() {
        return null;
      }
      void StatCard;`,
      "src/components/import/probe.test.tsx",
    );
    const mock = await lintSource(
      `export function Drawer() {
        return null;
      }`,
      "src/test/mocks/probe.tsx",
    );

    for (const { output } of [other, kit, testFile, mock]) {
      expect(output).not.toContain("no-shadow-kit-name");
    }
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
      `// oxlint-disable-next-line ui-kit/no-shadow-kit-name
      function TabsTrigger(_: { children: unknown }) {
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
      // oxlint-disable-next-line ui-kit/no-shadow-kit-name
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

  it("reports a hand-built textarea", async () => {
    const result = await lintSource(`
      function Probe() {
        return <textarea aria-label="Notiz" />;
      }
      void Probe;
    `);

    expect(result.status).toBe(1);
    expect(result.output.match(/ui-kit\(no-raw-input\)/g)).toHaveLength(1);
  });

  it("accepts the kit Textarea", async () => {
    const result = await lintSource(`
      function Probe() {
        return <Textarea label="Notiz" />;
      }
      void Probe;
    `);

    expect(result.output).not.toContain("no-raw-input");
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

describe.concurrent("ui-kit/no-select-override", () => {
  it("reports frame, fill, height and radius classes on kit selects", async () => {
    const result = await lintSource(`
      import { CustomSelect } from "~/components/ui/custom-select";
      import { MultiSelect } from "~/components/ui/multi-select";
      import { ListboxDropdown } from "~/components/ui/listbox-dropdown";
      const TRIGGER = "moto-content-surface h-9 w-56";
      export function Probe() {
        return <form>
          <CustomSelect value="" options={[]} onChange={() => {}} className={TRIGGER} />
          <MultiSelect value={[]} options={[]} onChange={() => {}} className="border-gray-200 bg-gray-50" />
          <ListboxDropdown value="" options={[]} onChange={() => {}} menuClassName="rounded-lg shadow-lg" />
        </form>;
      }
    `);
    const output = result.output;

    expect(result.status).toBe(1);
    expect(output.match(/ui-kit\(no-select-override\)/g)).toHaveLength(3);
  });

  it("reports look classes on the DateRangePicker trigger", async () => {
    const result = await lintSource(`
      import { DateRangePicker } from "~/components/ui/date-range-picker";
      export function Probe() {
        return <DateRangePicker value={undefined} onChange={() => {}} triggerClassName="h-9 rounded-none border-0" />;
      }
    `);

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-select-override)");
  });

  it("allows layout classes on the DateRangePicker trigger", async () => {
    const result = await lintSource(`
      import { DateRangePicker } from "~/components/ui/date-range-picker";
      export function Probe() {
        return <DateRangePicker value={undefined} onChange={() => {}} className="w-full sm:w-auto" triggerClassName="w-full sm:w-auto" />;
      }
    `);

    expect(result.output).not.toContain("no-select-override");
  });

  it("rejects unstyledTrigger outside the kit", async () => {
    const result = await lintSource(`
      import { ListboxDropdown } from "~/components/ui/listbox-dropdown";
      export function Probe() {
        return <ListboxDropdown value="" options={[]} onChange={() => {}} unstyledTrigger />;
      }
    `);

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-select-override)");
  });

  it("allows size and layout classes", async () => {
    const result = await lintSource(`
      import { CustomSelect } from "~/components/ui/custom-select";
      export function Probe() {
        return <CustomSelect value="" options={[]} onChange={() => {}} controlSize="compact" className="w-full md:w-56 font-medium" menuClassName="min-w-48" />;
      }
    `);

    expect(result.output).not.toContain("no-select-override");
  });

  it("leaves the kit itself alone", async () => {
    const result = await lintSource(
      `
      import { ListboxDropdown } from "~/components/ui/listbox-dropdown";
      export function Chip() {
        return <ListboxDropdown value="" options={[]} onChange={() => {}} unstyledTrigger className="rounded-xl bg-white" />;
      }
    `,
      "src/components/ui/page-header/chip.tsx",
    );

    expect(result.output).not.toContain("no-select-override");
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
      "ui-kit-ratchet-baseline.mjs",
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

// #5584: Pillen-Ratchet. Ein Fund je Element; die Baseline hält den Ist-Stand
// je Datei fest und darf nur sinken.
describe.concurrent("ui-kit/no-hand-rolled-pill", () => {
  const handRolled = (count: number) =>
    `function Probe() {
      return <div>${Array.from(
        { length: count },
        (_, index) =>
          `<span key="${index}" className="inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">Entwurf</span>`,
      ).join("")}</div>;
    }
    void Probe;`;

  it("fails a tinted span with pill shape outside the kit", async () => {
    const result = await lintSource(handRolled(1), "src/components/probe.tsx");

    expect(result.status).toBe(1);
    expect(result.output.match(/ui-kit\(no-hand-rolled-pill\)/g)).toHaveLength(
      1,
    );
    expect(result.output).toContain("StatusBadge");
  });

  it("fails a pill whose surface comes from the style prop", async () => {
    const result = await lintSource(
      `function Probe({ color }: { color: string }) {
        return <span className="rounded-md px-1.5 py-px text-[10px] uppercase" style={{ backgroundColor: color }}>Neu</span>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.output).toContain("ui-kit(no-hand-rolled-pill)");
  });

  it("accepts kit pills, text without a surface and tooltips", async () => {
    const result = await lintSource(
      `import { StatusBadge } from "~/components/ui/status-badge";
      function Probe() {
        return (
          <>
            <StatusBadge label="Entwurf" tone="outline" size="xs" />
            <span className="rounded-full px-2 py-0.5 text-xs text-gray-600">Ohne Fläche</span>
            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-sm">Größere Schrift</span>
            <span role="tooltip" className="rounded-md bg-gray-900 px-2 py-1 text-xs text-white">Hilfe</span>
          </>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain("no-hand-rolled-pill");
  });

  it("keeps the shrink-only baseline: equal and fewer pass, more fails", async () => {
    const [file, count] = Object.entries(
      pillRatchetBaseline["no-hand-rolled-pill"] ?? {},
    ).find(([candidate]) => candidate.endsWith(".tsx"))!;
    const [equal, more, fewer] = await Promise.all([
      lintSource(handRolled(count), file),
      lintSource(handRolled(count + 1), file),
      lintSource(handRolled(count - 1), file),
    ]);
    expect(equal.output).not.toContain("no-hand-rolled-pill");

    expect(more.status).toBe(1);
    expect(more.output).toContain("ui-kit(no-hand-rolled-pill)");

    expect(fewer.output).not.toContain("no-hand-rolled-pill");
  });
});

describe.concurrent("ui-kit/no-pill-dot", () => {
  it("fails a dot inside a pill (#2476)", async () => {
    const result = await lintSource(
      `function Probe() {
        return (
          <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs">
            <span className="h-1.5 w-1.5 rounded-full bg-gray-400" aria-hidden />
            Keine Lücken
          </span>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output.match(/ui-kit\(no-pill-dot\)/g)).toHaveLength(1);
  });

  it("accepts an icon inside a pill and a dot outside of one", async () => {
    const result = await lintSource(
      `function Probe() {
        return (
          <>
            <span className="inline-flex gap-1 rounded-full px-2.5 py-0.5 text-xs">
              <svg className="h-3 w-3" aria-hidden />
              Erledigt
            </span>
            <div className="flex gap-2">
              <span className="h-2 w-2 rounded-full bg-gray-400" aria-hidden />
              Legende
            </div>
          </>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain("no-pill-dot");
  });
});

describe.concurrent("ui-kit/no-hand-rolled-dot (#5598)", () => {
  it.each([
    [
      "a fill class",
      `<span className="h-2 w-2 rounded-full bg-gray-400" aria-hidden />`,
    ],
    [
      "a data colour",
      `<span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: color }} />`,
    ],
    [
      "a computed fill class",
      `<span className={\`size-2 rounded-full \${fill}\`} />`,
    ],
    [
      "a colour bar",
      `<span className="h-9 w-1 shrink-0 rounded-full" style={{ backgroundColor: color }} />`,
    ],
  ])("fails a hand-built dot with %s", async (_name, element) => {
    const result = await lintSource(
      `function Probe({ color, fill }: { color: string; fill: string }) {
        void color; void fill;
        return ${element};
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("no-hand-rolled-dot");
  });

  it("accepts the kit, skeletons, spinners, tracks and the kit folder", async () => {
    const accepted = await lintSource(
      `import { StatusDot } from "~/components/ui/status-dot";
      import { Skeleton } from "~/components/ui/skeleton";
      function Probe() {
        return (
          <>
            <StatusDot className="bg-gray-400" decorative />
            <Skeleton className="h-2.5 w-2.5 rounded-full" />
            <span className="h-3 w-3 animate-spin rounded-full border-2 border-current" />
            <span className="h-2 w-2/3 rounded-full bg-gray-100" />
          </>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const kit = await lintSource(
      `function Probe() { return <span className="h-2 w-2 rounded-full bg-white" />; }
      void Probe;`,
      "src/components/ui/probe.tsx",
    );

    expect(accepted.output).not.toContain("no-hand-rolled-dot");
    expect(kit.output).not.toContain("no-hand-rolled-dot");
  });

  it("leaves a dot inside a pill to no-pill-dot", async () => {
    const result = await lintSource(
      `function Probe() {
        return (
          <span className="inline-flex rounded-full px-2.5 py-0.5 text-xs">
            <span className="h-1.5 w-1.5 rounded-full bg-gray-400" />
            Offen
          </span>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain("no-hand-rolled-dot");
  });
});

describe.concurrent("ui-kit/no-status-dot-outside-plans (#5598)", () => {
  const source = `import { StatusDot } from "~/components/ui/status-dot";
    function Probe() { return <StatusDot label="Läuft" />; }
    void Probe;`;

  it("fails a StatusDot outside legends and plans", async () => {
    const result = await lintSource(source, "src/components/devices/probe.tsx");

    expect(result.status).toBe(1);
    expect(result.output).toContain("no-status-dot-outside-plans");
  });

  it("also catches the other import alias", async () => {
    const result = await lintSource(
      source.replace("~/components", "@/components"),
      "src/components/devices/probe.tsx",
    );

    expect(result.output).toContain("no-status-dot-outside-plans");
  });

  it("accepts a StatusDot in a plan", async () => {
    const result = await lintSource(
      source,
      "src/components/timetable/probe.tsx",
    );

    expect(result.output).not.toContain("no-status-dot-outside-plans");
  });
});

describe.concurrent("ui-kit/no-hex-alpha-tint", () => {
  it("fails a hex alpha tint in a pill's style", async () => {
    const result = await lintSource(
      `function Probe({ color }: { color: string }) {
        return <span className="rounded-full px-2.5 py-0.5 text-xs" style={{ backgroundColor: \`\${color}1A\`, color }}>Übernommen</span>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-hex-alpha-tint)");
  });

  it("accepts a kit tone value in the style", async () => {
    const result = await lintSource(
      `function Probe({ tone }: { tone: { bg: string } }) {
        return <span className="rounded-full px-2.5 py-0.5 text-xs" style={{ backgroundColor: tone.bg, borderColor: \`\${tone.bg}33\` }}>A</span>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain("no-hex-alpha-tint");
  });
});

describe("pill ratchet baseline", () => {
  it("matches the current findings exactly", () => {
    const result = spawnSync(
      process.execPath,
      [resolve("scripts/ui-kit-pill-baseline-update.mjs"), "--check"],
      { encoding: "utf8" },
    );

    expect(result.status, `${result.stdout}${result.stderr}`).toBe(0);
  }, 120_000);

  it("names only files that exist and positive counts", () => {
    for (const [rule, files] of Object.entries(pillRatchetBaseline)) {
      for (const [file, count] of Object.entries(files)) {
        expect(existsSync(resolve(file)), `${rule}: ${file}`).toBe(true);
        expect(count).toBeGreaterThan(0);
      }
    }
  });
});

describe.concurrent("ui-kit/no-important-override (#5581)", () => {
  it("reports an important utility on a kit component", async () => {
    const result = await lintSource(`
      import { SectionCard } from "~/components/ui/section-card";
      import { StatusBadge as Badge } from "~/components/ui/status-badge";
      function Probe() {
        return <>
          <SectionCard className="!p-0">Inhalt</SectionCard>
          <Badge label="Offen" tone="gray" className="md:!rounded-md" />
          <SectionCard bodyClassName={\`mt-2 p-0!\`}>Inhalt</SectionCard>
        </>;
      }
      void Probe;
    `);

    expect(result.status).toBe(1);
    expect(
      result.output.match(/ui-kit\(no-important-override\)/g),
    ).toHaveLength(3);
  });

  it("allows kit props, plain utilities and native elements", async () => {
    const result = await lintSource(`
      import { SectionCard } from "~/components/ui/section-card";
      function Probe() {
        return <>
          <SectionCard padding="none" className="mt-4">Inhalt</SectionCard>
          <div className="!p-0">Kein Kit</div>
        </>;
      }
      void Probe;
    `);

    expect(result.output).not.toContain("no-important-override");
  });
});

describe.concurrent("ui-kit/no-section-card-bleed (#5581)", () => {
  it("reports negative margins that undo the SectionCard padding", async () => {
    const result = await lintSource(`
      import { SectionCard } from "~/components/ui/section-card";
      function Probe() {
        return <SectionCard title="Verlauf" bodyClassName="mt-4 -mx-5 -mb-5">
          <table />
        </SectionCard>;
      }
      void Probe;
    `);

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-section-card-bleed)");
  });

  it("allows flush and spacing in bodyClassName", async () => {
    const result = await lintSource(`
      import { SectionCard } from "~/components/ui/section-card";
      function Probe() {
        return <SectionCard title="Verlauf" flush bodyClassName="mt-4">
          <table />
        </SectionCard>;
      }
      void Probe;
    `);

    expect(result.output).not.toContain("no-section-card-bleed");
  });
});

describe.concurrent("ui-kit/no-important-override baseline (#5581)", () => {
  it("fails a second override in a baselined file", async () => {
    const result = await lintSource(
      `
      import { Input } from "~/components/ui/input";
      function Probe() {
        return <>
          <Input aria-label="A" className="!h-11" />
          <Input aria-label="B" className="px-3!" />
        </>;
      }
      void Probe;
    `,
      "src/components/help/help-view.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-important-override)");
  });
});

describe("ui-kit/no-important-override baseline is current (#5581)", () => {
  // Die Zahl darf nur sinken: Nach einem Abbau muss die Baseline mitsinken,
  // sonst könnte die nächste Änderung den Abbau unbemerkt wieder auffüllen.
  it("records exactly the findings each baselined file still has", async () => {
    const previous = process.env.UI_KIT_IMPORTANT_OVERRIDE;
    process.env.UI_KIT_IMPORTANT_OVERRIDE = "count";
    try {
      for (const [file, expected] of Object.entries(
        importantOverrideBaseline,
      )) {
        const result = await lintSource(readFileSync(file, "utf8"), file);
        const found =
          result.output.match(/ui-kit\(no-important-override\)/g)?.length ?? 0;
        expect({ file, found }).toEqual({ file, found: expected });
      }
    } finally {
      if (previous === undefined) delete process.env.UI_KIT_IMPORTANT_OVERRIDE;
      else process.env.UI_KIT_IMPORTANT_OVERRIDE = previous;
    }
  });
});
