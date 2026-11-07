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
import { uppercaseLabelBaseline } from "./ui-kit-uppercase-label-baseline.mjs";
import { importantOverrideBaseline } from "./ui-kit-important-override-baseline.mjs";
import { surfaceBaseline } from "./ui-kit-surface-baseline.mjs";
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

describe.concurrent("ui-kit/no-hand-rolled-portal-nav", () => {
  const RULE = "ui-kit(no-hand-rolled-portal-nav)";

  it.each([
    "rounded-full bg-white/95 shadow-[0_-2px_20px_rgba(0,0,0,0.08)]",
    "absolute top-0 h-full rounded-full bg-gray-100",
    "relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full",
    "w-64 border-r border-gray-200/70 bg-white/95",
    "top-[73px] h-[calc(100vh-73px)]",
    "top-[121px] h-[calc(100vh-121px)]",
    "top-[153px] h-[calc(100vh-153px)]",
  ])("rejects the shell classes %s outside the kit", async (classes) => {
    const { output } = await lintSource(
      `export const Probe = () => <div className="${classes}" />;`,
      "src/components/parent/shell/probe-nav.tsx",
    );
    expect(output).toContain(RULE);
  });

  it("finds the classes in a template string too", async () => {
    const { output } = await lintSource(
      "const width = 'w-64 border-r';\nexport const aside = `${width} border-gray-200/70 bg-white/95`;",
      "src/components/dashboard/probe-nav.tsx",
    );
    expect(output).toContain(RULE);
  });

  it("finds shell classes when conditional combinations overflow", async () => {
    const { output } = await lintSource(
      [
        "declare const a: boolean;",
        "declare const b: boolean;",
        "declare const c: boolean;",
        "declare const d: boolean;",
        "declare const e: boolean;",
        "declare const f: boolean;",
        "declare const g: boolean;",
        'export const aside = `${a ? "border-r" : "border-l"} ${b ? "border-gray-200/70" : "border-gray-100"} ${c ? "bg-white/95" : "bg-gray-50"} ${d ? "p-1" : "p-2"} ${e ? "m-1" : "m-2"} ${f ? "gap-1" : "gap-2"} ${g ? "h-8" : "h-9"}`;',
      ].join("\n"),
      "src/components/dashboard/probe-nav.tsx",
    );
    expect(output).toContain(RULE);
  });

  it("finds shell classes combined by cn", async () => {
    const { output } = await lintSource(
      `declare function cn(...classes: string[]): string;
      export const Probe = () => <aside className={cn("border-r border-gray-200/70", "bg-white/95")} />;`,
      "src/components/dashboard/probe-nav.tsx",
    );
    expect(output).toContain(RULE);
  });

  it("rejects the kit's sidebar row classes outside the kit", async () => {
    const { output } = await lintSource(
      `import { portalSidebarRowClasses } from "~/components/ui/sidebar-geometry";
      void portalSidebarRowClasses;`,
      "src/components/parent/shell/probe-nav.tsx",
    );
    expect(output).toContain(RULE);
  });

  it("allows the kit itself and unrelated sticky headers", async () => {
    const shell = await lintSource(
      `const aside = "border-r border-gray-200/70 bg-white/95"; void aside;`,
      "src/components/ui/probe-nav.tsx",
    );
    expect(shell.output).not.toContain(RULE);
    const other = await lintSource(
      `const head = "sticky top-0 border-r bg-white/95"; void head;`,
      "src/components/dashboard/probe-nav.tsx",
    );
    expect(other.output).not.toContain(RULE);
  });
});

describe.concurrent("ui-kit/no-hand-rolled-dropzone", () => {
  const source = `function Probe() {
    return <div onDrop={() => undefined}>Ablegen</div>;
  }
  void Probe;`;

  it("reports a hand-built drop area", async () => {
    const result = await lintSource(source, "src/components/probe.tsx");

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-hand-rolled-dropzone)");
  });

  it("allows the kit Dropzone itself", async () => {
    const result = await lintSource(source, "src/components/ui/dropzone.tsx");

    expect(result.output).not.toContain("ui-kit(no-hand-rolled-dropzone)");
  });
});

describe.concurrent("ui-kit/no-hand-rolled-empty-state", () => {
  const RULE = "ui-kit(no-hand-rolled-empty-state)";
  const source = `function Probe() {
    return <div className="rounded-xl border border-dashed px-4 py-6 text-center">Leer</div>;
  }
  void Probe;`;

  it("reports a dashed, centered box", async () => {
    const result = await lintSource(source, "src/components/probe.tsx");

    expect(result.status).toBe(1);
    expect(result.output).toContain(RULE);
  });

  it("reports an expression-wrapped className string once", async () => {
    const result = await lintSource(
      `function Probe() {
        return <div className={"border-dashed text-center"}>Leer</div>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output.split(RULE)).toHaveLength(2);
  });

  it("reports a template-literal class constant", async () => {
    const result = await lintSource(
      `const classes = \`border-dashed text-center\`;
      export const Probe = () => <div className={classes}>Leer</div>;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain(RULE);
  });

  it("reports the classes across a template and cn arguments", async () => {
    const template = `function Probe({ surface }) {
      return <div className={\`\${surface} border-dashed text-center\`}>Leer</div>;
    }
    void Probe;`;
    const split = `function Probe({ cn }) {
      return <div className={cn("rounded-xl border-dashed", "text-center")}>Leer</div>;
    }
    void Probe;`;

    for (const probe of [template, split]) {
      const result = await lintSource(probe, "src/components/probe.tsx");
      expect(result.output).toContain(RULE);
    }
  });

  it("reports classes combined from constants", async () => {
    const result = await lintSource(
      `declare function cn(...classes: string[]): string;
      const dashedClasses = "border-dashed";
      const centeredClasses = "text-center";
      export const Probe = () => (
        <div className={cn(dashedClasses, centeredClasses)}>Leer</div>
      );`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain(RULE);
  });

  it("allows mutually exclusive dashed and centered classes", async () => {
    const result = await lintSource(
      `declare const loading: boolean;
      export const Probe = () => (
        <div className={loading ? "border-dashed" : "text-center"}>Leer</div>
      );`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain(RULE);
  });

  it("reports when a conditional branch has both classes", async () => {
    const result = await lintSource(
      `declare const loading: boolean;
      export const Probe = () => (
        <div className={loading ? "border-dashed text-center" : "p-4"}>Leer</div>
      );`,
      "src/components/probe.tsx",
    );

    expect(result.output).toContain(RULE);
  });

  it("reports dashed and centered keys in a cn class map", async () => {
    const result = await lintSource(
      `declare const empty: boolean;
      declare function cn(classes: Record<string, boolean>): string;
      export const Probe = () => (
        <div className={cn({ "border-dashed": empty, "text-center": empty })}>Leer</div>
      );`,
      "src/components/probe.tsx",
    );

    expect(result.output).toContain(RULE);
  });

  it("allows mutually exclusive keys in a cn class map", async () => {
    const result = await lintSource(
      `declare const empty: boolean;
      declare function cn(classes: Record<string, boolean>): string;
      export const Probe = () => (
        <div className={cn({ "border-dashed": empty, "text-center": !empty })}>Leer</div>
      );`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain(RULE);
  });

  it("allows a dashed box that is not centered", async () => {
    const result = await lintSource(
      source.replace(" text-center", ""),
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain(RULE);
  });

  it("allows the kit itself", async () => {
    const result = await lintSource(
      source,
      "src/components/ui/empty-state.tsx",
    );

    expect(result.output).not.toContain(RULE);
  });
});

describe.concurrent("ui-kit/no-hand-rolled-copy-field", () => {
  it("reports a framed code value", async () => {
    const result = await lintSource(
      `function Probe() {
        return <code className="rounded-lg border px-3 py-2">wert</code>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-hand-rolled-copy-field)");
  });

  it("reports a direct clipboard write", async () => {
    const result = await lintSource(
      `export const copy = (text: string) => navigator.clipboard.writeText(text);`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-hand-rolled-copy-field)");
  });

  it("reports a clipboard write through window", async () => {
    const result = await lintSource(
      `export const copy = (text: string) => window.navigator.clipboard.writeText(text);`,
      "src/components/probe.tsx",
    );

    expect(result.output).toContain("ui-kit(no-hand-rolled-copy-field)");
  });

  it("allows inline code and only the shared copy primitives", async () => {
    const inline = await lintSource(
      `function Probe() {
        return <code className="shrink-0">wert</code>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const hook = await lintSource(
      `export const copy = (text: string) => navigator.clipboard.writeText(text);`,
      "src/lib/use-clipboard-copy.ts",
    );
    const copyField = await lintSource(
      `function Probe() {
        return <code className="rounded-lg border px-3 py-2">wert</code>;
      }
      void Probe;`,
      "src/components/ui/copy-field.tsx",
    );
    const otherUiComponent = await lintSource(
      `export const copy = (text: string) => navigator.clipboard.writeText(text);`,
      "src/components/ui/probe.tsx",
    );

    expect(inline.output).not.toContain("ui-kit(no-hand-rolled-copy-field)");
    expect(hook.output).not.toContain("ui-kit(no-hand-rolled-copy-field)");
    expect(copyField.output).not.toContain("ui-kit(no-hand-rolled-copy-field)");
    expect(otherUiComponent.status).toBe(1);
    expect(otherUiComponent.output).toContain(
      "ui-kit(no-hand-rolled-copy-field)",
    );
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
      // moto-popover-surface steht seit #5601 nur noch im Kit.
      "src/components/ui/probe.tsx",
    );
    const output = result.output;

    expect(result.status, output).toBe(0);
    expect(output).not.toContain("no-hand-rolled-surface");
  });

  it("ignores unsupported variants, frameless borders and transparent fills", async () => {
    const result = await lintSource(
      `
      function Probe() {
        return <>
          <div className="rounded-xl border-0 bg-white px-4 ring-1" />
          <div className="rounded-xl border-none bg-white p-4" />
          <div className="rounded-2xl border-b bg-white p-4" />
          <div className="rounded-lg border bg-white/0 p-3" />
          <div className="flex gap-4 print:rounded-2xl print:border print:bg-white" />
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

  // #5594: außerhalb des Kits zählen auch kleinere Radien, halbdurchsichtiges
  // Weiß und ein Rahmen mit Polster ganz ohne Füllung.
  it.each([
    '<div className="rounded-lg border border-gray-200 bg-white p-3" />',
    '<div className="rounded-md border bg-white px-3 py-2" />',
    '<div className="rounded-2xl border border-gray-200 bg-white/95 p-4 shadow-lg" />',
    '<div className="rounded-lg border bg-white sm:rounded-2xl" />',
    '<div className="sm:rounded-lg sm:border sm:bg-white sm:p-3" />',
    '<div className="p-3 sm:rounded-lg md:border md:bg-white" />',
    '<div className="sm:rounded-lg max-md:border max-md:bg-white p-3" />',
    '<div className="min-[768px]:rounded-lg min-[900px]:border min-[1024px]:bg-white p-3" />',
    '<div className="p-3 md:hover:rounded-lg md:hover:border md:hover:bg-white" />',
    '<section className="space-y-3 rounded-md border border-gray-200 p-3" />',
    '<li className="rounded-xl border px-4 py-3" />',
    '<div className="rounded-lg border bg-transparent p-3" />',
    '<div className="rounded-lg border p-4 hover:bg-gray-50" />',
    '<div className="rounded-lg border p-4 focus:bg-white" />',
  ])("reports the widened surface %s outside the kit", async (element) => {
    const result = await lintSource(
      `export const Probe = () => ${element};`,
      "src/components/probe-surface.tsx",
    );

    expect(result.status, result.output).toBe(1);
    expect(
      result.output.match(/ui-kit\(no-hand-rolled-surface\)/g),
    ).toHaveLength(1);
  });

  it("does not join non-overlapping min and max breakpoint variants", async () => {
    const result = await lintSource(
      `export const Probe = () => (
        <div className="max-sm:rounded-lg sm:border sm:bg-white p-3" />
      );`,
      "src/components/probe-surface.tsx",
    );

    expect(result.output).not.toContain("ui-kit(no-hand-rolled-surface)");
  });

  it("does not join non-overlapping arbitrary breakpoint variants", async () => {
    const result = await lintSource(
      `export const Probe = () => (
        <div className="min-[1024px]:rounded-lg max-[900px]:border max-[900px]:bg-white p-3" />
      );`,
      "src/components/probe-surface.tsx",
    );

    expect(result.output).not.toContain("ui-kit(no-hand-rolled-surface)");
  });

  it("follows a class chain held in a local constant", async () => {
    const result = await lintSource(
      `const frame = "rounded-lg border border-gray-200 p-3";
      export const Probe = () => <div className={frame} />;`,
      "src/components/probe-surface.tsx",
    );

    expect(result.output).toContain("ui-kit(no-hand-rolled-surface)");
  });

  it("follows a module constant declared after its JSX use", async () => {
    const result = await lintSource(
      `export const Probe = () => <div className={frame} />;
      const frame = "rounded-lg border border-gray-200 p-3";`,
      "src/components/probe-surface.tsx",
    );

    expect(result.output).toContain("ui-kit(no-hand-rolled-surface)");
  });

  it("reports distinct legacy and widened branches in one className expression", async () => {
    const result = await lintSource(
      `const legacy = "rounded-xl border bg-white p-4";
      const frame = "rounded-lg border p-3";
      export const Probe = ({ legacyMode }: { legacyMode: boolean }) => (
        <div className={legacyMode ? legacy : frame} />
      );`,
      "src/components/probe-surface.tsx",
    );

    expect(
      result.output.match(/ui-kit\(no-hand-rolled-surface\)/g),
      result.output,
    ).toHaveLength(2);
    expect(result.output).toContain("rounded-xl/2xl … border … bg-white");
    expect(result.output).toContain(
      "rounded-md…2xl … border … bg-white(/NN) oder Rahmen mit Polster",
    );
  });

  it("does not join mutually exclusive class branches", async () => {
    const result = await lintSource(
      `export const Probe = ({ compact }: { compact: boolean }) => (
        <div className={compact ? "rounded-lg border" : "rounded-lg p-3"} />
      );`,
      "src/components/probe-surface.tsx",
    );

    expect(result.output).not.toContain("ui-kit(no-hand-rolled-surface)");
  });

  it.each([
    '<button type="button" className="rounded-lg border bg-white px-4 py-2" />',
    '<a href="/x" className="rounded-lg border bg-white px-4 py-2" />',
    '<label className="rounded-lg border p-3" />',
    '<input className="rounded-md border bg-white px-3 py-2" />',
    '<div className="rounded-lg border border-dashed p-4" />',
    '<div className="moto-content-surface rounded-xl border p-4" />',
    '<div className="rounded-lg border border-gray-200 bg-gray-50 p-3" />',
    '<div className="rounded-lg border border-gray-200" />',
    '<div className="rounded-full border bg-white px-2 py-0.5" />',
    '<div className="rounded-xl border border-transparent p-3" />',
    '<div className="rounded-lg border border-l-[3px] bg-white p-3" />',
    '<div className="hover:rounded-lg hover:border hover:bg-white hover:p-3" />',
  ])(
    "leaves controls, kit surfaces and other shapes alone: %s",
    async (element) => {
      const result = await lintSource(
        `export const Probe = () => ${element};`,
        "src/components/probe-surface.tsx",
      );

      expect(result.output).not.toContain("no-hand-rolled-surface");
    },
  );

  it.each(["Button", "ButtonLink", "ListRowLink", "ChoiceTile"])(
    "leaves the shared control component %s alone",
    async (component) => {
      const result = await lintSource(
        `export const Probe = () => <${component} className="rounded-lg border bg-white p-3" />;`,
        "src/components/probe-surface.tsx",
      );

      expect(result.output).not.toContain("no-hand-rolled-surface");
    },
  );

  it("reports a chain held in a constant once, not once per use", async () => {
    const result = await lintSource(
      `const card = "rounded-xl border bg-white p-4";
      export const Probe = () => <><div className={card} /><div className={card} /></>;`,
      "src/components/probe-surface.tsx",
    );

    expect(
      result.output.match(/ui-kit\(no-hand-rolled-surface\)/g),
    ).toHaveLength(1);
  });

  it("leaves the #2619 parent exceptions out of the widened check only", async () => {
    const [widened, classic] = await Promise.all([
      lintSource(
        'export const Probe = () => <div className="rounded-lg border bg-white p-4" />;',
        "src/components/parent/child/child-day-card.tsx",
      ),
      lintSource(
        'export const Probe = () => <div className="rounded-2xl border bg-white p-4" />;',
        "src/components/parent/child/child-day-card.tsx",
      ),
    ]);

    expect(widened.output).not.toContain("no-hand-rolled-surface");
    expect(classic.output).toContain("ui-kit(no-hand-rolled-surface)");
  });

  it("keeps the narrower chain inside the kit, where controls are defined", async () => {
    const result = await lintSource(
      'export const Probe = () => <div className="rounded-lg border bg-white px-3 py-2" />;',
      "src/components/ui/probe-control.tsx",
    );

    expect(result.output).not.toContain("no-hand-rolled-surface");
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

  it("honours the escape hatch for a composed surface expression", async () => {
    const result = await lintSource(
      `declare function cn(...classes: string[]): string;
      export const Probe = () => (
        // oxlint-disable-next-line ui-kit/no-hand-rolled-surface -- Nachrichtensprechblase, keine Kartenfläche
        <div className={cn("rounded-xl", "border", "bg-white")} />
      );`,
      "src/components/probe-surface.tsx",
    );

    expect(result.status, result.output).toBe(0);
    expect(result.output).not.toContain("no-hand-rolled-surface");
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
    "src/app/[tenant]/(protected)/rooms/page.test.tsx",
  ])("leaves %s alone", async (path) => {
    const result = await lintSource(kickerSource, path);
    const output = result.output;

    expect(result.status, `${path}\n${output}`).toBe(0);
    expect(output).not.toContain("no-tenant-kicker");
  });

  it.each([
    "src/components/school/probe.tsx",
    "src/components/class-day/probe.tsx",
    "src/app/operator/probe/page.tsx",
  ])("reports %s since #5604", async (path) => {
    const result = await lintSource(kickerSource, path);

    expect(result.status).toBe(1);
    expect(result.output).toContain("no-tenant-kicker");
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

  it("reports a card surface held in a composed local constant", async () => {
    const result = await lintSource(
      `declare function cn(...classes: string[]): string;
      const card = cn("moto-content-surface", "rounded-2xl", "p-6");
      export const Probe = () => <div className={card} />;`,
      "src/components/probe-card.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-hand-rolled-page-card)");
  });

  it("accepts control surfaces in pages and the chain in the kit", async () => {
    const [pageControl, kitCard] = await Promise.all([
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
        "src/components/ui/probe-card.tsx",
      ),
    ]);

    for (const result of [pageControl, kitCard]) {
      const output = result.output;
      expect(result.status, output).toBe(0);
      expect(output).not.toContain("no-hand-rolled-page-card");
    }
  });

  // #5594: die Regel gilt für jede .tsx unter app/ und components/ außer ui/.
  it.each([
    "src/components/rooms/room-card.tsx",
    "src/app/[tenant]/(public)/anmeldung/probe-view.tsx",
    "src/app/help/probe/page.tsx",
  ])("reports the card chain in %s", async (path) => {
    const result = await lintSource(
      `export const Probe = () => <div className="moto-content-surface rounded-2xl border p-6" />;`,
      path,
    );

    expect(result.status, path).toBe(1);
    expect(result.output).toContain("ui-kit(no-hand-rolled-page-card)");
  });

  it.each([
    "src/components/parent/child/child-day-card.tsx",
    "src/components/parent/child/child-switcher.tsx",
    "src/components/parent/start/parent-start-page.tsx",
  ])("leaves the #2619 parent exception %s alone", async (path) => {
    const result = await lintSource(
      `export const Probe = () => <div className="moto-content-surface rounded-2xl border p-6" />;`,
      path,
    );

    expect(result.output).not.toContain("no-hand-rolled-page-card");
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
      `// oxlint-disable-next-line ui-kit/no-radix-outside-kit -- Gegenstand dieses Tests ist die Erkennung der Reiter-Inhalte.
      import * as TabsPrimitive from "@radix-ui/react-tabs";
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

describe.concurrent("ui-kit/no-hand-rolled-toggle-control", () => {
  it("reports role switch, role checkbox and a painted check box", async () => {
    const result = await lintSource(
      `const Check = () => <svg />;
      function Probe({ on }: { on: boolean }) {
        return <>
          <button type="button" role="switch" aria-checked={on} aria-label="An" />
          <div role="checkbox" aria-checked={on} aria-label="Wahl" tabIndex={0} />
          <div role={on ? "checkbox" : undefined} aria-label="Wahl" tabIndex={0} />
          <span className="flex h-4 w-4 items-center justify-center rounded border">
            {on && <Check />}
          </span>
        </>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(
      result.output.match(/ui-kit\(no-hand-rolled-toggle-control\)/g),
    ).toHaveLength(4);
  });

  it("allows round status marks and the kit itself", async () => {
    const outside = await lintSource(
      `const Check = () => <svg />;
      function Probe() {
        return <span className="flex h-4 w-4 rounded-full border"><Check /></span>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const kit = await lintSource(
      `function Probe() {
        return <button type="button" role="switch" aria-checked aria-label="An" />;
      }
      void Probe;`,
      "src/components/ui/probe.tsx",
    );

    expect(outside.output).not.toContain(
      "ui-kit(no-hand-rolled-toggle-control)",
    );
    expect(kit.output).not.toContain("ui-kit(no-hand-rolled-toggle-control)");
  });
});

describe.concurrent("ui-kit/no-hand-rolled-fieldset", () => {
  it("reports a framed fieldset", async () => {
    const result = await lintSource(
      `function Probe() {
        return <fieldset className="rounded-xl border border-gray-200 p-4">
          <legend>Wochentage</legend>
        </fieldset>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-hand-rolled-fieldset)");
  });

  it("allows an unframed group and a fieldset used as a card", async () => {
    const result = await lintSource(
      `function Probe() {
        return <>
          <fieldset className="space-y-2"><legend>Aktion</legend></fieldset>
          <fieldset className="moto-content-surface rounded-2xl border p-4">
            <legend>Kind</legend>
          </fieldset>
        </>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain("ui-kit(no-hand-rolled-fieldset)");
  });
});

describe.concurrent("ui-kit/no-hand-rolled-choice-tile", () => {
  it("reports a framed label around a Checkbox or Radio", async () => {
    const result = await lintSource(
      `const Checkbox = () => <input type="checkbox" />;
      const Radio = () => <input type="radio" />;
      function Probe() {
        return <>
          <label className="flex rounded-lg border px-3 py-2"><Checkbox /> Montag</label>
          <label className="moto-content-surface flex rounded-xl p-3">
            <span><Radio /></span> Dienstag
          </label>
        </>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(
      result.output.match(/ui-kit\(no-hand-rolled-choice-tile\)/g),
    ).toHaveLength(2);
  });

  it("allows a plain label row and a framed label without a choice control", async () => {
    const result = await lintSource(
      `const Checkbox = () => <input type="checkbox" />;
      function Probe() {
        return <>
          <label className="flex items-center gap-2"><Checkbox /> Montag</label>
          <label className="rounded-lg border px-3 py-2">Name<input /></label>
        </>;
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain("ui-kit(no-hand-rolled-choice-tile)");
  });
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

  it("leaves the kit's primitives alone", async () => {
    const result = await lintSource(
      `
      export function Field() {
        return <input aria-label="Vorname" />;
      }
    `,
      "src/components/ui/input.tsx",
    );
    const output = result.output;

    expect(output).not.toContain("no-raw-input");
  });
});

describe.concurrent("ui-kit/no-raw-table", () => {
  it("reports a table outside the kit", async () => {
    const result = await lintSource(`
      export function Probe() {
        return <table><tbody><tr><td>Mia</td></tr></tbody></table>;
      }
    `);

    expect(result.status).toBe(1);
    expect(result.output.match(/ui-kit\(no-raw-table\)/g)).toHaveLength(1);
  });

  it("accepts DataTable and the kit's own table", async () => {
    const outside = await lintSource(`
      import { DataTable } from "~/components/ui/data-table";
      export function Probe() {
        return <DataTable columns={[]} rows={[]} getRowKey={() => 1} />;
      }
    `);
    const kit = await lintSource(
      `
      export function Grid() {
        return <table><tbody /></table>;
      }
    `,
      "src/components/ui/planning/grid.tsx",
    );

    expect(outside.output).not.toContain("no-raw-table");
    expect(kit.output).not.toContain("no-raw-table");
  });
});

describe.concurrent("ui-kit/no-em-dash-empty-value", () => {
  it("reports an em dash as JSX text and as a fallback value", async () => {
    const result = await lintSource(`
      export function Probe({ name }: { name?: string }) {
        return <div><span>—</span><span>{name ?? "—"}</span><span>Abholzeit: —</span><span>{\`Gehzeit: —\`}</span></div>;
      }
    `);

    expect(result.status).toBe(1);
    expect(
      result.output.match(/ui-kit\(no-em-dash-empty-value\)/g),
    ).toHaveLength(4);
  });

  it("accepts the en dash and an em dash inside a sentence", async () => {
    const result = await lintSource(`
      export function Probe({ name }: { name?: string }) {
        return <p title="Mia — Klasse 3">{name ?? "–"} — läuft ab</p>;
      }
    `);

    expect(result.output).not.toContain("no-em-dash-empty-value");
  });
});

describe.concurrent("ui-kit/no-control-height-override", () => {
  it("reports h-8, h-9 and !size-* on kit controls, also behind a variant", async () => {
    const result = await lintSource(`
      import { Input } from "~/components/ui/input";
      import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
      const FIELD = "h-9 py-1";
      export function Probe() {
        return <form>
          <Input className={FIELD} />
          <Input className="sm:h-8" />
          <OverflowMenu items={[]} triggerClassName="!size-7" />
        </form>;
      }
    `);

    expect(result.status).toBe(1);
    expect(
      result.output.match(/ui-kit\(no-control-height-override\)/g),
    ).toHaveLength(3);
  });

  it("reports the height on selects and the date range trigger", async () => {
    const result = await lintSource(`
      import { CustomSelect } from "~/components/ui/custom-select";
      import { MultiSelect } from "~/components/ui/multi-select";
      import { DateRangePicker } from "~/components/ui/date-range-picker";
      export function Probe() {
        return <form>
          <CustomSelect value="" options={[]} onChange={() => {}} className="h-9" />
          <MultiSelect value={[]} options={[]} onChange={() => {}} className="h-8" />
          <DateRangePicker value={undefined} onChange={() => {}} triggerClassName="h-9" />
        </form>;
      }
    `);

    expect(
      result.output.match(/ui-kit\(no-control-height-override\)/g),
    ).toHaveLength(3);
  });

  it("allows the kit sizes and layout classes", async () => {
    const result = await lintSource(`
      import { Input } from "~/components/ui/input";
      import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
      export function Probe() {
        return <form>
          <Input controlSize="compact" className="w-full tabular-nums md:w-28" />
          <OverflowMenu items={[]} triggerSize="sm" triggerClassName="ml-auto" />
        </form>;
      }
    `);

    expect(result.output).not.toContain("no-control-height-override");
  });

  it("leaves other components and the kit itself alone", async () => {
    const other = await lintSource(`
      import { Spark } from "~/components/ui/spark";
      export const Probe = () => <Spark className="h-9 !size-7" />;
    `);
    const kit = await lintSource(
      `
      import { Input } from "~/components/ui/input";
      export const Probe = () => <Input className="h-9" />;
    `,
      "src/components/ui/probe.tsx",
    );

    expect(other.output).not.toContain("no-control-height-override");
    expect(kit.output).not.toContain("no-control-height-override");
  });
});

describe.concurrent("Kit-Varianten aus #5606", () => {
  it("reports look classes on tabs and allows layout", async () => {
    const result = await lintSource(`
      import { TabsList, TabsTrigger } from "~/components/ui/tabs";
      export function Probe() {
        return <TabsList className="mt-2 w-full justify-start">
          <TabsTrigger value="a" className="min-h-16 rounded-lg text-base" />
          <TabsTrigger value="b" className="flex-1" />
        </TabsList>;
      }
    `);

    expect(result.output.match(/ui-kit\(no-tabs-override\)/g)).toHaveLength(1);
  });

  it("reports a text size in titleClassName and allows other classes", async () => {
    const result = await lintSource(`
      import { SectionCard } from "~/components/ui/section-card";
      export function Probe() {
        return <>
          <SectionCard title="A" titleClassName="text-xl" />
          <SectionCard title="B" titleClassName="font-medium text-gray-600" />
        </>;
      }
    `);

    expect(
      result.output.match(/ui-kit\(no-section-card-title-size\)/g),
    ).toHaveLength(1);
  });

  it("reports a card on EmptyState and allows spacing", async () => {
    const result = await lintSource(`
      import { EmptyState } from "~/components/ui/empty-state";
      export function Probe() {
        return <>
          <EmptyState title="A" className="moto-content-surface rounded-2xl border" />
          <EmptyState title="B" className="py-0" />
        </>;
      }
    `);

    expect(
      result.output.match(/ui-kit\(no-empty-state-surface\)/g),
    ).toHaveLength(1);
  });

  it("reports initials in a tinted circle and allows other text", async () => {
    const result = await lintSource(`
      export function Probe({ first, label }: { first: string; label: string }) {
        return <>
          <span className="size-11 rounded-full bg-gray-100">{initials(first)}</span>
          <span className="rounded-full bg-gray-100 px-2">{label}</span>
        </>;
      }
    `);

    expect(
      result.output.match(/ui-kit\(no-hand-rolled-avatar\)/g),
    ).toHaveLength(1);
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

// #5584: hard-zero. Ein Fund je Element, ohne Ausnahme für den alten Bestand.
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

  it("is hard-zero: a file from the former baseline fails too", async () => {
    const result = await lintSource(
      handRolled(1),
      "src/components/ui/page-header/ActiveFilterChips.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-hand-rolled-pill)");
  });
});

// #5584 Abschlussrunde: auch `div` und Klassen aus erkennbar statischen Quellen.
describe.concurrent(
  "ui-kit/no-hand-rolled-pill, div und statische Quellen",
  () => {
    const RULE = "ui-kit(no-hand-rolled-pill)";

    it("fails a div pill", async () => {
      const result = await lintSource(
        `export const Probe = () => (
        <div className="bg-moto-orange-soft flex items-center gap-1.5 rounded-full px-3 py-1 text-xs">Achtung</div>
      );`,
        "src/components/probe.tsx",
      );

      expect(result.output).toContain(RULE);
    });

    it("accepts a hint box, a brand surface and a tooltip on a div", async () => {
      const result = await lintSource(
        `export const Probe = () => (
        <>
          <div className="rounded-md bg-gray-50 px-3 py-2 text-xs text-gray-600">Hinweis in ganzer Breite</div>
          <div className="w-fit rounded-md bg-gray-50 px-3 py-2 text-xs text-gray-600">Kurzer Hinweis</div>
          <div className="inline-flex items-center gap-2 rounded-full border border-gray-200 bg-white/85 px-3 py-1.5 text-xs shadow-sm backdrop-blur-sm">Für Schulen</div>
          <div role="tooltip" className="inline-block rounded-md bg-gray-900 px-2 py-1 text-xs text-white">Hilfe</div>
        </>
      );`,
        "src/components/probe.tsx",
      );

      expect(result.output).not.toContain(RULE);
    });

    it("finds a surface in a helper or table declared below the element", async () => {
      const result = await lintSource(
        `export const Probe = ({ status }: { status: string }) => (
        <>
          <span className={\`rounded-full px-2 py-1 text-xs \${statusClass(status)}\`}>Status</span>
          <span className={\`rounded-full px-2 py-1 text-xs \${TONES[status]}\`}>Status</span>
        </>
      );
      function statusClass(status: string) {
        return status === "aktiv" ? "bg-moto-green-soft" : "bg-gray-100";
      }
      const TONES: Record<string, string> = { aktiv: "bg-moto-green-soft" };`,
        "src/components/probe.tsx",
      );

      expect(
        result.output.match(/ui-kit\(no-hand-rolled-pill\)/g),
      ).toHaveLength(2);
    });

    it("finds a surface in an exported helper of another module", () => {
      const root = mkdtempSync(join(tmpdir(), "pill-import-"));
      temporaryDirectories.push(root);
      mkdirSync(join(root, "src/lib"), { recursive: true });
      mkdirSync(join(root, "src/components"), { recursive: true });
      writeFileSync(
        join(root, "src/lib/tones.ts"),
        `export function statusClass(status: string) {
  return \`font-medium \${status === "aktiv" ? "bg-moto-green-soft" : "bg-gray-100"}\`;
}
export function statusName(status: string) {
  return status === "aktiv" ? "Aktiv" : "Inaktiv";
}
async function other() {
  return "bg-moto-red-soft";
}
void other;`,
      );
      const lintProbeFile = (name: string, helper: string) => {
        const path = join(root, "src/components", name);
        writeFileSync(
          path,
          `import { ${helper} } from "~/lib/tones";
        export const Probe = ({ status }: { status: string }) => (
          <span className={\`rounded-full px-2 py-1 text-xs \${${helper}(status)}\`}>Status</span>
        );`,
        );
        return spawnSync(
          resolve("node_modules/.bin/oxlint"),
          ["-c", resolve(".oxlintrc.json"), path],
          { encoding: "utf8" },
        ).stdout;
      };

      expect(lintProbeFile("surface.tsx", "statusClass")).toContain(
        "no-hand-rolled-pill",
      );
      // Der andere Export desselben Moduls trägt keine Fläche bei.
      expect(lintProbeFile("name.tsx", "statusName")).not.toContain(
        "no-hand-rolled-pill",
      );
    });

    it("stays silent on classes it cannot resolve statically (known limit)", async () => {
      const result = await lintSource(
        `export const Probe = ({ toneClass }: { toneClass: string }) => (
        <span className={\`rounded-full px-2 py-1 text-xs \${toneClass}\`}>Status</span>
      );`,
        "src/components/probe.tsx",
      );

      expect(result.output).not.toContain(RULE);
    });
  },
);

describe.concurrent("ui-kit/no-pill-dot", () => {
  it("fails a dot inside a div pill", async () => {
    const result = await lintSource(
      `export const Probe = () => (
        <div className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs">
          <span className="h-1.5 w-1.5 rounded-full bg-gray-400" aria-hidden />
          Keine Lücken
        </div>
      );`,
      "src/components/probe.tsx",
    );

    expect(result.output).toContain("ui-kit(no-pill-dot)");
  });

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

describe.concurrent("ui-kit/no-hand-rolled-field-label (#5589)", () => {
  const RULE = "ui-kit(no-hand-rolled-field-label)";

  it.each([
    '<label htmlFor="a" className="block text-sm font-medium text-gray-700">A</label>',
    '<label htmlFor="a" className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase">A</label>',
    '<label htmlFor="a" className="text-sm font-medium">A</label>',
    '<span className="mb-1.5 block text-sm font-medium text-gray-700">A</span>',
    '<label><span className="block text-sm font-medium">A</span><Input /></label>',
    '<label><span className="text-xs font-medium text-gray-700">A</span><Input /></label>',
    '<label className="flex flex-col gap-1 text-sm font-medium">A<Input /></label>',
  ])("reports %s", async (element) => {
    const { output } = await lintSource(
      `import { Input } from "~/components/ui/input";
      export const Probe = () => ${element};
      void Input;`,
      "src/components/probe-form.tsx",
    );
    expect(output).toContain(RULE);
  });

  it.each([
    '<label className="flex items-center gap-2 text-sm font-medium"><Checkbox /> A</label>',
    '<label className="sm:flex text-sm font-medium"><Checkbox /> A</label>',
    '<label className="flex"><span className="block text-sm font-medium">A</span><Checkbox /></label>',
    '<label htmlFor="a" className="sr-only text-sm font-medium">A</label>',
    '<label htmlFor="a" className="rounded-lg border px-3 py-2 text-sm font-medium">Datei wählen</label>',
    '<span className="block text-sm font-medium text-gray-900">Name einer Zeile</span>',
    '<span className="mb-1 text-sm font-medium">Inline</span>',
    '<label><Input /><span className="text-xs font-medium">Nach dem Feld</span></label>',
    '<label className="flex"><span className="text-sm font-medium">A</span><Checkbox /></label>',
    '<label htmlFor="a" className="block text-sm text-gray-700">Ohne Gewicht</label>',
  ])("accepts %s", async (element) => {
    const { output } = await lintSource(
      `import { Checkbox } from "~/components/ui/checkbox";
      import { Input } from "~/components/ui/input";
      export const Probe = () => ${element};
      void Checkbox;
      void Input;`,
      "src/components/probe-form.tsx",
    );
    expect(output).not.toContain(RULE);
  });

  it("allows the label in the kit component itself", async () => {
    const { output } = await lintSource(
      'export const Probe = () => <label htmlFor="a" className="block text-sm font-medium">A</label>;',
      "src/components/ui/field-label.tsx",
    );
    expect(output).not.toContain(RULE);
  });

  // Hard-zero seit #5590: auch die früheren Baseline-Dateien haben keinen
  // erlaubten Fund mehr.
  it.each([
    "src/components/surveys/survey-editor-fields.tsx",
    "src/components/timetable/bulk-substitution-modal.tsx",
  ])("fails the first finding in %s", async (file) => {
    const result = await lintSource(
      'export const Probe = () => <label htmlFor="f" className="block text-sm font-medium">A</label>;',
      file,
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain(RULE);
  });
});

describe.concurrent("ui-kit/no-uppercase-label (#5590)", () => {
  const RULE = "ui-kit(no-uppercase-label)";
  const lintProbe = (
    source: string,
    file = "src/components/probe-section.tsx",
  ) => lintSource(source, file);

  it.each([
    '<h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">Gruppe</h3>',
    '<p className="mb-2 text-[11px] font-bold tracking-wider uppercase">Gruppe</p>',
    '<span className={cn("uppercase", "tracking-widest")}>Tag</span>',
    '<legend className={open ? "tracking-wide uppercase" : "text-sm"}>Gruppe</legend>',
    '<SectionCard titleClassName="uppercase tracking-wide" title="A" />',
  ])("reports %s", async (element) => {
    const { output } = await lintProbe(
      `import { cn } from "~/lib/utils";
      import { SectionCard } from "~/components/ui/section-card";
      const open = true;
      export const Probe = () => ${element};
      void cn; void SectionCard; void open;`,
    );
    expect(output).toContain(RULE);
  });

  it.each([
    '<h3 className="text-sm font-semibold text-gray-600">Gruppe</h3>',
    '<span className="uppercase">Nur Versalien</span>',
    '<span className="tracking-wide">Nur Laufweite</span>',
    '<span className="normal-case tracking-tight">Normal</span>',
  ])("accepts %s", async (element) => {
    const { output } = await lintProbe(
      `export const Probe = () => ${element};`,
    );
    expect(output).not.toContain(RULE);
  });

  it("accepts mutually exclusive conditional classes", async () => {
    const { output } = await lintProbe(`
      const condensed = true;
      export const Probe = () => (
        <span className={condensed ? "uppercase" : "tracking-wide"}>
          Gruppe
        </span>
      );
    `);
    expect(output).not.toContain(RULE);
  });

  it("keeps conditional cn() arguments on separate class paths", async () => {
    const { output } = await lintProbe(`
      import { cn } from "~/lib/utils";
      const condensed = true;
      export const Probe = () => (
        <span className={cn("text-xs", condensed ? "uppercase" : "tracking-wide")}>
          Gruppe
        </span>
      );
      void cn;
    `);
    expect(output).not.toContain(RULE);
  });

  it("reports a complete class chain within one conditional cn() path", async () => {
    const { output } = await lintProbe(`
      import { cn } from "~/lib/utils";
      const condensed = true;
      export const Probe = () => (
        <span className={cn("uppercase", condensed ? "tracking-wide" : "text-sm")}>
          Gruppe
        </span>
      );
      void cn;
    `);
    expect(output).toContain(RULE);
  });

  it("reports a class chain in a constant once", async () => {
    const { output } = await lintProbe(`
      const LABEL = "text-xs tracking-wide uppercase";
      export const Probe = () => <p className={LABEL}>Gruppe</p>;
    `);
    expect(output.split(RULE).length - 1).toBe(1);
  });

  it("reports a class chain split across a className constant", async () => {
    const { output } = await lintProbe(`
      import { cn } from "~/lib/utils";
      const classes = cn("uppercase", "tracking-wide");
      export const Probe = () => <span className={classes}>Gruppe</span>;
      void cn;
    `);
    expect(output.split(RULE).length - 1).toBe(1);
  });

  it.each([
    ["the kit", "src/components/ui/section-card.tsx"],
    ["the /start page", "src/app/start/page.tsx"],
    ["the NFC print guide", "src/app/help/nfc/erste-schritte/page.tsx"],
  ])("leaves %s alone", async (_name, file) => {
    const { output } = await lintProbe(
      'export const Probe = () => <p className="text-xs tracking-wide uppercase">Kicker</p>;',
      file,
    );
    expect(output).not.toContain(RULE);
  });

  it("fails one finding more than a file's baseline", async () => {
    const [file, allowed] = Object.entries(
      uppercaseLabelBaseline["no-uppercase-label"]!,
    )[0]!;
    const labels = Array.from(
      { length: allowed + 1 },
      () => '<p className="text-xs tracking-wide uppercase">A</p>',
    ).join("");
    const result = await lintSource(
      `export const Probe = () => <>${labels}</>;`,
      file,
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain(RULE);
  });
});

describe("ui-kit/no-uppercase-label baseline is current (#5590)", () => {
  // Die Zahl darf nur sinken: Nach einem Abbau muss die Baseline mitsinken.
  it("records exactly the findings each file still has", () => {
    const result = spawnSync(
      process.execPath,
      [
        resolve("scripts/ui-kit-uppercase-label-baseline-update.mjs"),
        "--check",
      ],
      { encoding: "utf8" },
    );

    expect(result.status, `${result.stdout}${result.stderr}`).toBe(0);
  }, 120_000);
});

describe.concurrent("ui-kit/no-hand-rolled-inset (#5592)", () => {
  const RULE = "ui-kit(no-hand-rolled-inset)";
  const lintProbe = (source: string, file = "src/components/probe-panel.tsx") =>
    lintSource(source, file);

  it.each([
    '<div className="rounded-xl bg-gray-50 p-3">A</div>',
    '<div className="rounded-lg border border-gray-200 bg-gray-50/70 px-3 py-2">A</div>',
    '<p className="rounded-md bg-gray-50/50 py-1 text-sm">A</p>',
    '<div className="rounded-xl bg-gray-50 pt-2">A</div>',
    '<section className="space-y-2 rounded-2xl border border-gray-100 bg-gray-50 p-4">A</section>',
    '<div className={open ? "rounded-xl bg-gray-50 p-3" : "hidden"}>A</div>',
  ])("reports %s", async (element) => {
    const { output } = await lintProbe(
      `const open = true; export const Probe = () => ${element};`,
    );
    expect(output).toContain(RULE);
  });

  it.each([
    '<div className="rounded-full bg-gray-50 p-3">Pille</div>',
    '<div className="rounded-xl bg-gray-100 p-3">Anderer Ton</div>',
    '<div className="rounded-xl hover:bg-gray-50 p-3">Nur beim Darüberfahren</div>',
    '<div className="rounded-xl bg-gray-50">Ohne Polster</div>',
    '<div className="bg-gray-50 p-3">Ohne Radius</div>',
    '<button type="button" className="rounded-xl bg-gray-50 p-3">Knopf</button>',
    '<a href="/x" className="rounded-xl bg-gray-50 p-3">Link</a>',
  ])("accepts %s", async (element) => {
    const { output } = await lintProbe(
      `export const Probe = () => ${element};`,
    );
    expect(output).not.toContain(RULE);
  });

  it("follows a local constant into the className", async () => {
    const { output } = await lintProbe(`
      export function Probe() {
        const surface = "rounded-xl bg-gray-50 p-3";
        return <div className={surface}>A</div>;
      }
    `);
    expect(output).toContain(RULE);
  });

  it.each([
    'const SURFACE = "moto-content-surface rounded-2xl border p-5";',
    'export const SURFACE = "rounded-xl border border-gray-200";',
    "const SURFACE = `rounded-lg bg-white p-3`;",
    'const SURFACE = "rounded-xl bg-moto-red/5 p-3";',
    'const SURFACE = "rounded-xl border" as const;',
  ])("reports the module constant %s", async (declaration) => {
    const { output } = await lintProbe(`${declaration}\nvoid SURFACE;`);
    expect(output).toContain(RULE);
  });

  it.each([
    'const VALUE = "rounded-full border px-2";',
    'const VALUE = "mt-4 min-h-0 flex-1 overflow-hidden";',
    'const VALUE = "rounded-xl sind Karten";',
  ])("accepts the module constant %s", async (declaration) => {
    const { output } = await lintProbe(`${declaration}\nvoid VALUE;`);
    expect(output).not.toContain(RULE);
  });

  it("does not treat a constant inside a function as a module constant", async () => {
    const { output } = await lintProbe(`
      export function probe() {
        const unused = "rounded-xl border border-gray-200";
        return unused;
      }
    `);
    expect(output).not.toContain(RULE);
  });

  it("leaves the kit alone", async () => {
    const { output } = await lintProbe(
      'export const Probe = () => <div className="rounded-xl bg-gray-50 p-3">A</div>;',
      "src/components/ui/inset-panel.tsx",
    );
    expect(output).not.toContain(RULE);
  });

  it("fails a single finding in a file outside the exceptions", async () => {
    const result = await lintProbe(
      'export const Probe = () => <div className="rounded-xl bg-gray-50 p-3">A</div>;',
      "src/components/parent/child-care.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain(RULE);
  });

  it.each([
    "src/components/parent/child/child-day-card.tsx",
    "src/components/parent/child/child-switcher.tsx",
    "src/components/parent/start/parent-start-page.tsx",
  ])("leaves the #2619 parent exception %s alone", async (file) => {
    const { output } = await lintProbe(
      'export const Probe = () => <div className="rounded-xl bg-gray-50 p-3">A</div>;',
      file,
    );
    expect(output).not.toContain(RULE);
  });
});

describe.concurrent("ui-kit/no-hand-rolled-notice (#5591)", () => {
  const RULE = "ui-kit(no-hand-rolled-notice)";

  it.each([
    '<div className="border-moto-amber/50 bg-moto-amber/10 text-moto-amber-strong rounded-lg border px-3 py-2 text-xs">A</div>',
    '<p className="border-moto-blue/20 bg-moto-blue-soft rounded-xl border p-3">A</p>',
    '<ul className="border-moto-red/20 bg-moto-red-soft/50 rounded-2xl border p-4"><li>A</li></ul>',
    '<section className="border-moto-orange/30 bg-moto-orange-soft mt-6 rounded-lg border-2 p-4">A</section>',
    '<aside className="border-moto-orange/30 bg-moto-orange-soft rounded-lg border pt-3">A</aside>',
    '<details className="border-moto-blue/30 bg-moto-blue-soft rounded-lg border pb-3">A</details>',
    '<div className="border-moto-red/30 bg-moto-red-soft rounded-lg border pl-3">A</div>',
    '<div className="border-moto-green/30 bg-moto-green-soft rounded-lg border pr-3">A</div>',
    '<div className="border-moto-blue/30 bg-moto-blue-soft rounded-lg border ps-3">A</div>',
    '<div className="border-moto-orange/30 bg-moto-orange-soft rounded-lg border pe-3">A</div>',
  ])("reports %s", async (element) => {
    const { output } = await lintSource(
      `export const Probe = () => ${element};`,
      "src/components/probe-notice.tsx",
    );
    expect(output).toContain(RULE);
  });

  it.each([
    '<div className="border-moto-green/40 bg-moto-green/10 rounded-full border px-2 py-0.5">Pille</div>',
    '<div className="border-moto-blue/30 bg-moto-red-soft rounded-lg border p-3">Zwei Töne</div>',
    '<div className="bg-moto-blue/10 rounded-lg p-3">Ohne Rand</div>',
    '<div className="border-moto-amber/30 bg-moto-amber/10 rounded-lg border">Ohne Abstand</div>',
    '<span className="border-moto-amber/30 bg-moto-amber/10 rounded-lg border px-2">Inline</span>',
  ])("accepts %s", async (element) => {
    const { output } = await lintSource(
      `export const Probe = () => ${element};`,
      "src/components/probe-notice.tsx",
    );
    expect(output).not.toContain(RULE);
  });

  it("allows the tinted box in the Alert itself", async () => {
    const { output } = await lintSource(
      'export const Probe = () => <div className="border-moto-amber/30 bg-moto-amber/10 rounded-lg border p-3">A</div>;',
      "src/components/ui/alert.tsx",
    );
    expect(output).not.toContain(RULE);
  });

  it("fails a single finding anywhere outside the kit", async () => {
    const result = await lintProbe(
      'export const Probe = () => <div className="border-moto-amber/30 bg-moto-amber/10 rounded-lg border p-3">A</div>;',
      "src/components/parent/child-care.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain(RULE);
  });

  // Amber ist kein Ton von Alert: auch ohne Rand ein handgebauter Hinweis.
  it.each([
    '<div className="bg-moto-amber/10 text-moto-amber-strong rounded-lg px-3 py-2 text-sm">A</div>',
    '<p className="bg-moto-amber-soft text-moto-amber-strong mt-1 rounded px-2 py-1 text-xs">A</p>',
  ])("fails the borderless amber box %s", async (element) => {
    const result = await lintProbe(
      `export const Probe = () => ${element};`,
      "src/components/probe-notice.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain(RULE);
  });

  it.each([
    '<span className="bg-moto-amber-soft text-moto-amber-strong rounded px-1.5 py-0.5 text-xs">Inline</span>',
    '<div className="bg-moto-amber/20 text-moto-amber-strong inline-flex h-7 w-7 rounded-full">!</div>',
  ])("accepts %s", async (element) => {
    const { output } = await lintProbe(
      `export const Probe = () => ${element};`,
      "src/components/probe-notice.tsx",
    );
    expect(output).not.toContain(RULE);
  });
});

describe.concurrent("ui-kit surface baseline (#5594)", () => {
  it.each([
    [
      "no-hand-rolled-page-card",
      '<div className="moto-content-surface rounded-2xl border p-5">A</div>',
    ],
    [
      "no-hand-rolled-surface",
      '<div className="rounded-lg border border-gray-200 bg-white p-3">A</div>',
    ],
  ])("fails one %s finding more than a file's baseline", async (rule, card) => {
    const [file, allowed] = Object.entries(surfaceBaseline[rule]!)[0]!;
    const cards = Array.from({ length: allowed + 1 }, () => card).join("");
    const result = await lintSource(
      `export const Probe = () => <>${cards}</>;`,
      file,
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain(`ui-kit(${rule})`);
  });

  it("keeps the classic surface chain hard-zero in a baselined file", async () => {
    const [file, allowed] = Object.entries(
      surfaceBaseline["no-hand-rolled-surface"]!,
    )[0]!;
    expect(allowed).toBeGreaterThan(0);
    const result = await lintSource(
      'export const Probe = () => <div className="rounded-2xl border bg-white p-4">A</div>;',
      file,
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-hand-rolled-surface)");
  });

  it("keeps a composed legacy surface constant hard-zero in a baselined file", async () => {
    const [file, allowed] = Object.entries(
      surfaceBaseline["no-hand-rolled-surface"]!,
    )[0]!;
    expect(allowed).toBeGreaterThan(0);
    const result = await lintSource(
      `declare function cn(...classes: string[]): string;
      const card = cn("rounded-xl", "border", "bg-white", "p-4");
      export const Probe = () => <div className={card}>A</div>;`,
      file,
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-hand-rolled-surface)");
  });

  it("does not count a finding behind its disable comment", async () => {
    const [file, allowed] = Object.entries(
      surfaceBaseline["no-hand-rolled-surface"]!,
    )[0]!;
    const cards = Array.from(
      { length: allowed },
      () => '<div className="rounded-lg border bg-white p-3">A</div>',
    ).join("");
    const result = await lintSource(
      `export const Probe = () => (
        <>
          {/* oxlint-disable-next-line ui-kit/no-hand-rolled-surface -- Probe */}
          <div className="rounded-lg border bg-white p-3">B</div>
          ${cards}
        </>
      );`,
      file,
    );

    expect(result.output).not.toContain("no-hand-rolled-surface");
  });

  it("tolerates exactly the recorded findings", async () => {
    const [file, allowed] = Object.entries(
      surfaceBaseline["no-hand-rolled-page-card"]!,
    )[0]!;
    const cards = Array.from(
      { length: allowed },
      () =>
        '<div className="moto-content-surface rounded-2xl border p-5">A</div>',
    ).join("");
    const result = await lintSource(
      `export const Probe = () => <>${cards}</>;`,
      file,
    );

    expect(result.output).not.toContain("no-hand-rolled-page-card");
  });

  it("excludes hard-zero legacy surfaces from the generated baseline", () => {
    const directory = mkdtempSync(join(tmpdir(), "surface-baseline-"));
    temporaryDirectories.push(directory);
    mkdirSync(join(directory, "scripts"));
    for (const file of [
      "ui-kit-surface-baseline-update.mjs",
      "ui-kit-ratchet-baseline.mjs",
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
          "ui-kit/no-hand-rolled-surface": "error",
          "ui-kit/no-hand-rolled-page-card": "error",
        },
      }),
    );
    mkdirSync(join(directory, "src/components"), { recursive: true });
    writeFileSync(
      join(directory, "src/components/probe.tsx"),
      `export const Probe = () => <>
        <div className="rounded-xl border bg-white p-4">Alt</div>
        <div className="rounded-lg border bg-white p-3">Neu</div>
      </>;`,
    );
    const baselinePath = join(directory, "ui-kit-surface-baseline.json");
    writeFileSync(
      baselinePath,
      `${JSON.stringify(
        {
          "no-hand-rolled-surface": { "src/components/probe.tsx": 1 },
          "no-hand-rolled-page-card": {},
        },
        null,
        2,
      )}\n`,
    );

    const result = spawnSync(
      process.execPath,
      [
        join(directory, "scripts/ui-kit-surface-baseline-update.mjs"),
        "--check",
      ],
      {
        encoding: "utf8",
        env: {
          ...process.env,
          UI_KIT_SURFACE_BASELINE_PATH: baselinePath,
        },
      },
    );

    expect(result.status, `${result.stdout}${result.stderr}`).toBe(0);
    expect(`${result.stdout}${result.stderr}`).toContain(
      "no-hand-rolled-surface: 1 Funde in 1 Dateien",
    );
  }, 120_000);
});

describe("ui-kit surface baseline is current (#5594)", () => {
  // Die Zahl darf nur sinken: Nach einem Abbau muss die Baseline mitsinken.
  it("records exactly the findings each file still has", () => {
    const result = spawnSync(
      process.execPath,
      [resolve("scripts/ui-kit-surface-baseline-update.mjs"), "--check"],
      { encoding: "utf8" },
    );

    expect(result.status, `${result.stdout}${result.stderr}`).toBe(0);
  }, 120_000);
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

describe.concurrent("ui-kit/no-radix-outside-kit", () => {
  it("reports a Radix import in a portal page", async () => {
    const result = await lintSource(
      `import * as TabsPrimitive from "@radix-ui/react-tabs";
      void TabsPrimitive;
      void import("@radix-ui/react-dialog");`,
      "src/app/operator/organizations/probe/page.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output.match(/ui-kit\(no-radix-outside-kit\)/g)).toHaveLength(
      2,
    );
  });

  it("allows a Radix import inside the kit", async () => {
    const result = await lintSource(
      `import * as TabsPrimitive from "@radix-ui/react-tabs";
      void TabsPrimitive;`,
      "src/components/ui/probe.tsx",
    );

    expect(result.output).not.toContain("ui-kit(no-radix-outside-kit)");
  });
});

describe.concurrent("ui-kit/no-hand-rolled-page-card in other portals", () => {
  it.each(["src/app/school/probe/page.tsx", "src/app/operator/probe/page.tsx"])(
    "reports a hand-rolled card in %s",
    async (path) => {
      const result = await lintSource(
        `export default function Probe() {
        return <div className="moto-content-surface rounded-2xl border p-5" />;
      }`,
        path,
      );

      expect(result.status).toBe(1);
      expect(result.output).toContain("ui-kit(no-hand-rolled-page-card)");
    },
  );

  // Seit #5594 auch das Elternportal; nur die drei #2619-Dateien sind
  // ausgenommen (siehe oben).
  it("reports a hand-rolled card in the parent portal", async () => {
    const result = await lintSource(
      `export default function Probe() {
        return <div className="moto-content-surface rounded-2xl border p-5" />;
      }`,
      "src/app/parents/probe/page.tsx",
    );

    expect(result.output).toContain("ui-kit(no-hand-rolled-page-card)");
  });
});

describe.concurrent("ui-kit/no-kit-domain-import", () => {
  it("reports a kit file that imports a domain component", async () => {
    const result = await lintSource(
      `import { Thing } from "~/components/dashboard/thing";
      void Thing;`,
      "src/components/ui/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("ui-kit(no-kit-domain-import)");
  });

  it("reports a relative path out of the kit and a dynamic import", async () => {
    const result = await lintSource(
      `import { Thing } from "../../help/thing";
      void Thing;
      void import("../../auth/other");`,
      "src/components/ui/page-header/probe.tsx",
    );

    expect(result.output.match(/ui-kit\(no-kit-domain-import\)/g)).toHaveLength(
      2,
    );
  });

  it("reports a re-export out of the kit", async () => {
    const result = await lintSource(
      `export { Thing } from "~/components/auth/thing";
      export * from "~/components/database/other";`,
      "src/components/ui/probe.tsx",
    );

    expect(result.output.match(/ui-kit\(no-kit-domain-import\)/g)).toHaveLength(
      2,
    );
  });

  it("reports fetch and useSWR in the kit", async () => {
    const result = await lintSource(
      `import useSWR from "swr";
      export function useProbe() {
        void fetch("/api/x");
        void window.fetch("/api/y");
        return useSWR("k");
      }`,
      "src/components/ui/probe.tsx",
    );

    expect(result.output.match(/ui-kit\(no-kit-domain-import\)/g)).toHaveLength(
      4,
    );
  });

  it("accepts kit-internal imports, lib helpers and files outside the kit", async () => {
    const inKit = await lintSource(
      `import { Button } from "./button";
      import { Alert } from "~/components/ui/alert";
      import { Tile } from "../icon-tile";
      import { cn } from "~/lib/utils";
      void Button; void Alert; void Tile; void cn;`,
      "src/components/ui/planning/probe.tsx",
    );
    const outside = await lintSource(
      `import { Thing } from "~/components/help/thing";
      void Thing;
      void fetch("/api/x");`,
      "src/components/parent/probe.tsx",
    );

    expect(inKit.output).not.toContain("no-kit-domain-import");
    expect(outside.output).not.toContain("no-kit-domain-import");
  });

  it("leaves kit tests alone", async () => {
    const result = await lintSource(
      `import { Thing } from "~/components/home/thing";
      void Thing;`,
      "src/components/ui/probe.test.tsx",
    );

    expect(result.output).not.toContain("no-kit-domain-import");
  });
});

describe.concurrent(
  "kit allowlists for no-raw-input and no-hand-rolled-surface",
  () => {
    const rawInput = `export const Probe = () => <input type="text" />;`;
    const surface = `export const Probe = () => <div className="rounded-2xl border border-gray-200 bg-white p-4" />;`;

    it("reports a raw input in a kit file that is no primitive", async () => {
      const result = await lintSource(rawInput, "src/components/ui/probe.tsx");
      expect(result.output).toContain("ui-kit(no-raw-input)");
    });

    it("accepts a raw input in a real primitive", async () => {
      const result = await lintSource(rawInput, "src/components/ui/input.tsx");
      expect(result.output).not.toContain("no-raw-input");
    });

    it("accepts a surface in a kit control on the allowlist", async () => {
      const result = await lintSource(
        surface,
        "src/components/ui/checkbox-card.tsx",
      );
      expect(result.output).not.toContain("no-hand-rolled-surface");
    });

    it("reports a hand-built surface in a kit file off the allowlist", async () => {
      const result = await lintSource(surface, "src/components/ui/probe.tsx");
      expect(result.output).toContain("ui-kit(no-hand-rolled-surface)");
    });
  },
);

describe.concurrent("ui-kit/no-hand-rolled-menu (#5601)", () => {
  it.each([
    ["a menu", `<div role="menu" />`],
    ["a menu entry", `<button type="button" role="menuitem" />`],
    ["a listbox", `<ul role="listbox" />`],
    ["an option", `<li role="option" aria-selected={false} />`],
    ["a conditional role", `<div role={open ? "menu" : undefined} />`],
  ])("fails %s", async (_name, element) => {
    const result = await lintSource(
      `function Probe({ open }: { open: boolean }) {
        void open;
        return ${element};
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("no-hand-rolled-menu");
  });

  it("accepts other roles and the kit folder", async () => {
    const other = await lintSource(
      `export const Probe = () => <div role="group" />;`,
      "src/components/probe.tsx",
    );
    const kit = await lintSource(
      `export const Probe = () => <div role="menu" />;`,
      "src/components/ui/probe.tsx",
    );

    expect(other.output).not.toContain("no-hand-rolled-menu");
    expect(kit.output).not.toContain("no-hand-rolled-menu");
  });
});

describe.concurrent("ui-kit/no-hand-rolled-popover (#5601)", () => {
  it.each([
    [
      "a class string",
      `<div className="moto-popover-surface rounded-xl border" />`,
    ],
    [
      "a computed class",
      `<div className={\`moto-popover-surface \${extra}\`} />`,
    ],
  ])("fails %s", async (_name, element) => {
    const result = await lintSource(
      `function Probe({ extra }: { extra: string }) {
        void extra;
        return ${element};
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("no-hand-rolled-popover");
  });

  it("accepts the kit folder and other surfaces", async () => {
    const kit = await lintSource(
      `export const Probe = () => <div className="moto-popover-surface rounded-xl border" />;`,
      "src/components/ui/probe.tsx",
    );
    const other = await lintSource(
      `export const Probe = () => <div className="moto-content-surface rounded-2xl border" />;`,
      "src/components/probe.tsx",
    );

    expect(kit.output).not.toContain("no-hand-rolled-popover");
    expect(other.output).not.toContain("no-hand-rolled-popover");
  });
});

describe.concurrent("Listen-Bausteine (#5593)", () => {
  it.each([
    [
      "no-arbitrary-shadow",
      `<div className="shadow-[0_1px_0_rgba(17,24,39,0.04)]" />`,
    ],
    ["no-arbitrary-shadow", `<div className="hover:shadow-[0_0_4px_red]" />`],
    [
      "no-hand-rolled-list-row",
      `<div className="flex rounded-xl bg-gray-50/50 px-3 py-2" />`,
    ],
    [
      "no-hand-rolled-framed-list",
      `<ul className="divide-y divide-gray-100 rounded-lg border border-gray-200" />`,
    ],
    [
      "no-hand-rolled-nav-row",
      `<button type="button" className="flex w-full">Anna<ChevronRight /></button>`,
    ],
    [
      "no-hand-rolled-nav-row",
      `<button type="button" className="flex justify-between">Anna<span aria-hidden="true">›</span></button>`,
    ],
  ])("%s fails %s", async (rule, element) => {
    const result = await lintSource(
      `const ChevronRight = () => null;
      const cn = (...parts: string[]) => parts.join(" ");
      const on = true;
      export const Probe = () => ${element};
      void [ChevronRight, cn, on];`,
      "src/components/probe.tsx",
    );

    expect(result.status, result.output).toBe(1);
    expect(result.output).toContain(rule);
  });

  it.each([
    ["no-arbitrary-shadow", `<div className="shadow-sm ring-4" />`],
    [
      "no-hand-rolled-list-row",
      `<div className="rounded-xl border bg-gray-50/50 p-3" />`,
    ],
    [
      "no-hand-rolled-framed-list",
      `<ul className="divide-y divide-gray-100" />`,
    ],
    ["no-hand-rolled-nav-row", `<a href="/x" className="flex w-full">Anna</a>`],
    [
      "no-hand-rolled-nav-row",
      `<a href="/x" className="flex gap-1">Heute öffnen<ChevronRight /></a>`,
    ],
  ])("%s accepts %s", async (rule, element) => {
    const result = await lintSource(
      `export const Probe = () => ${element};`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain(rule);
  });

  it("leaves the kit folder alone", async () => {
    const result = await lintSource(
      `export const Probe = () => <div className="shadow-[0_0_4px_red] rounded-xl bg-gray-50/50 px-3" />;`,
      "src/components/ui/probe.tsx",
    );

    expect(result.output).not.toContain("no-arbitrary-shadow");
    expect(result.output).not.toContain("no-hand-rolled-list-row");
  });
});

describe.concurrent("ui-kit/no-own-spinner (#5588)", () => {
  it.each([
    [
      "a border spinner",
      `<div className="h-4 w-4 animate-spin rounded-full border-2 border-t-transparent" />`,
    ],
    [
      "animate-spin on an icon",
      `<svg className="h-4 w-4 animate-spin text-white" />`,
    ],
    [
      "a computed class",
      `<span className={\`h-3 w-3 animate-spin \${tone}\`} />`,
    ],
  ])("fails %s", async (_name, element) => {
    const result = await lintSource(
      `function Probe({ tone }: { tone: string }) {
        void tone;
        return ${element};
      }
      void Probe;`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("no-own-spinner");
  });

  it.each([
    ["lucide-react", "Loader2"],
    ["lucide-react", "LoaderCircle"],
    ["@phosphor-icons/react/ssr", "CircleNotchIcon"],
  ])("fails the %s icon %s", async (source, name) => {
    const result = await lintSource(
      `import { ${name} } from "${source}";
      void ${name};`,
      "src/components/probe.tsx",
    );

    expect(result.status).toBe(1);
    expect(result.output).toContain("no-own-spinner");
  });

  it("accepts the kit pieces, other icons and the kit folder", async () => {
    const accepted = await lintSource(
      `import { Check } from "lucide-react";
      import { SpinnerIcon } from "~/components/ui/icons";
      import { Loading } from "~/components/ui/loading";
      function Probe() {
        return (
          <>
            <SpinnerIcon className="h-4 w-4" />
            <Loading fullPage={false} message="Wird geladen" />
            <Check className="h-4 w-4" />
          </>
        );
      }
      void Probe;`,
      "src/components/probe.tsx",
    );
    const kit = await lintSource(
      `function Probe() {
        return <svg className="h-4 w-4 animate-spin" />;
      }
      void Probe;`,
      "src/components/ui/probe.tsx",
    );

    expect(accepted.output).not.toContain("no-own-spinner");
    expect(kit.output).not.toContain("no-own-spinner");
  });
});

describe.concurrent("ui-kit Eingabe-Bausteine (#5596)", () => {
  it.each([
    [
      "no-hand-rolled-removable-chip",
      `<span className="inline-flex items-center rounded-full bg-gray-50">
        {label}
        <button type="button" onClick={remove} aria-label="Weg"><X /></button>
      </span>`,
    ],
    [
      "no-hand-rolled-removable-chip",
      `<span className="inline-flex rounded-full bg-gray-50">
        {label}
        <button type="button" onClick={remove} aria-label="Weg">×</button>
      </span>`,
    ],
    [
      "no-hand-rolled-number-stepper",
      `<div className="flex">
        <Button onClick={remove} aria-label="Weniger"><Minus /></Button>
        <span>{label}</span>
        <Button onClick={remove} aria-label="Mehr"><Plus /></Button>
      </div>`,
    ],
    [
      "no-hand-rolled-weekday-toggle",
      `<div>{weekdays.map((day) => (
        <label key={day}><Checkbox checked onChange={remove} />{day}</label>
      ))}</div>`,
    ],
    [
      "no-hand-rolled-weekday-toggle",
      `<div>{WEEKDAYS.map((day) => (
        <Button key={day} aria-pressed onClick={remove}>{day}</Button>
      ))}</div>`,
    ],
  ])("%s fails the hand-built form", async (rule, element) => {
    const result = await lintSource(
      `import { Minus, Plus, X } from "lucide-react";
      import { Button } from "~/components/ui/button";
      import { Checkbox } from "~/components/ui/checkbox";
      const weekdays = ["Mo"];
      const WEEKDAYS = ["Mo"];
      function Probe({ label, remove }: { label: string; remove: () => void }) {
        return ${element};
      }
      void Probe; void Minus; void Plus; void X; void Button; void Checkbox;
      void weekdays; void WEEKDAYS;`,
      "src/components/probe.tsx",
    );

    expect(result.output).toContain(rule);
  });

  it.each([
    [
      "no-hand-rolled-removable-chip",
      `<RemovableChip label={label} onRemove={remove} removeLabel="Weg" />`,
    ],
    [
      "no-hand-rolled-removable-chip",
      // Eine Zeile mit Schließen-Kreuz ist kein Chip.
      `<div className="flex rounded-xl border">
        {label}
        <button type="button" onClick={remove} aria-label="Weg"><X /></button>
      </div>`,
    ],
    [
      "no-hand-rolled-number-stepper",
      `<Button onClick={remove} aria-label="Mehr"><Plus /></Button>`,
    ],
    [
      "no-hand-rolled-weekday-toggle",
      `<div>{weekdays.map((day) => (
        <ToggleChip key={day} pressed onPressedChange={remove}>{day}</ToggleChip>
      ))}</div>`,
    ],
  ])("%s accepts the kit form", async (rule, element) => {
    const result = await lintSource(
      `import { Plus, X } from "lucide-react";
      import { Button } from "~/components/ui/button";
      import { RemovableChip } from "~/components/ui/removable-chip";
      import { ToggleChip } from "~/components/ui/toggle-chip";
      const weekdays = ["Mo"];
      function Probe({ label, remove }: { label: string; remove: () => void }) {
        return ${element};
      }
      void Probe; void Plus; void X; void Button; void RemovableChip;
      void ToggleChip; void weekdays;`,
      "src/components/probe.tsx",
    );

    expect(result.output).not.toContain(rule);
  });
});

describe.concurrent("ui-kit sticky bars and slide-over layout (#5602)", () => {
  async function outputOf(jsx: string, path = "src/probe.tsx") {
    const result = await lintSource(
      `function Probe() { return ${jsx}; }\nvoid Probe;`,
      path,
    );
    return result.output;
  }

  it("reports a hand-built sticky bottom surface", async () => {
    const output = await outputOf(
      `<div className="sticky bottom-4 z-10 rounded-2xl border bg-white/95 shadow-lg" />`,
    );
    expect(output).toContain("ui-kit(no-hand-rolled-sticky-bar)");
  });

  it("reports a hand-built sticky top selection surface", async () => {
    const output = await outputOf(
      `<div className="moto-content-surface sticky top-2 z-30 rounded-xl border" />`,
    );
    expect(output).toContain("ui-kit(no-hand-rolled-sticky-bar)");
  });

  it("reports a sticky bar split over several class strings", async () => {
    const output = await outputOf(
      `<div className={cn("sticky bottom-4", "border-t bg-gray-50")} />`,
    );
    expect(output).toContain("ui-kit(no-hand-rolled-sticky-bar)");
  });

  it("allows a sticky table head and the kit itself", async () => {
    const head = await outputOf(
      `<div className="sticky top-0 z-30 border-b bg-white" />`,
    );
    expect(head).not.toContain("no-hand-rolled-sticky-bar");
    const kit = await outputOf(
      `<div className="sticky bottom-4 rounded-2xl border bg-white/95 shadow-lg" />`,
      "src/components/ui/probe.tsx",
    );
    expect(kit).not.toContain("no-hand-rolled-sticky-bar");
  });

  it("reports a free slide-over width, as prop and as class", async () => {
    const prop = await outputOf(
      `<SlideOverContent widthClass="sm:w-[640px]" />`,
    );
    expect(prop).toContain("ui-kit(no-arbitrary-slide-over-width)");
    const klass = await outputOf(
      `<SlideOverContent className="sm:w-[640px]" />`,
    );
    expect(klass).toContain("ui-kit(no-arbitrary-slide-over-width)");
  });

  it("allows size and the modal width", async () => {
    const output = await outputOf(
      `<><SlideOverContent size="lg" /><Modal widthClass="sm:w-[640px]" /></>`,
    );
    expect(output).not.toContain("no-arbitrary-slide-over-width");
  });

  it("reports flex-row on a slide-over head, allows sm:flex-row on the foot", async () => {
    const importLine = `import { SlideOverFooter, SlideOverHeader } from "~/components/ui/slide-over";`;
    const bad = await lintSource(
      `${importLine}\nfunction Probe() { return <SlideOverHeader className="flex-row gap-3" />; }\nvoid Probe;`,
      "src/probe.tsx",
    );
    expect(bad.output).toContain("ui-kit(no-slide-over-row-override)");
    const good = await lintSource(
      `${importLine}\nfunction Probe() { return <SlideOverFooter className="flex-col sm:flex-row" />; }\nvoid Probe;`,
      "src/probe.tsx",
    );
    expect(good.output).not.toContain("no-slide-over-row-override");
  });
});
