import { expect, test } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { join } from "node:path";

import { gotoTolerant, loadAccess, login, tenantBaseUrl } from "./access";
import { RequestRecorder } from "./recorder";
import { PERF_TARGETS } from "./targets";

// Render-Zählung mit react-scan gegen den DEV-Server (#2938). Läuft NUR über
// playwright.perf-dev.config.ts. react-scan wird per Init-Script injiziert,
// die App bleibt unverändert. Zähler sind auf Prod übertragbar, Render-Zeiten
// nicht (Dev-React ist deutlich langsamer).
//
// react-scan 0.5.7 liefert weder `unnecessary` noch gefüllte `changes` über
// den onRender-Callback; gezählt werden deshalb Renders und Updates je
// Komponente. Was ein Update auslöst, muss der Quelltext beantworten.

const OUT_DIR = join(process.cwd(), "perf-results", "react-scan");
const QUIET = { quietMs: 2000, capMs: 40_000 } as const;
const REACT_SCAN_BUNDLE = createRequire(import.meta.url).resolve(
  "react-scan/dist/auto.global.js",
);

const access = loadAccess();
const base = tenantBaseUrl(access);

interface RenderEntry {
  component: string;
  /** Ein Render-Objekt = ein Render; `Render.count` von react-scan ist kumulativ je Fiber. */
  renders: number;
  /** RenderPhase.Update (2): alles außer Mount/Unmount. */
  updates: number;
  timeMs: number;
}

interface RenderReport {
  components: number;
  renders: number;
  updates: number;
  timeMs: number;
  /** Sekunden zwischen Start der Zählung und Auslesen. */
  windowSeconds: number;
  top: RenderEntry[];
}

interface ScanWindow {
  reactScan?: (options: Record<string, unknown>) => void;
  __perfRenders?: {
    reset: () => void;
    dump: () => RenderEntry[];
  };
}

test.beforeAll(() => {
  mkdirSync(OUT_DIR, { recursive: true });
});

function aggregate(entries: RenderEntry[], windowMs: number): RenderReport {
  const sorted = [...entries].sort((a, b) => b.renders - a.renders);
  return {
    components: entries.length,
    renders: entries.reduce((s, e) => s + e.renders, 0),
    updates: entries.reduce((s, e) => s + e.updates, 0),
    timeMs: Math.round(entries.reduce((s, e) => s + e.timeMs, 0)),
    windowSeconds: Math.round(windowMs / 100) / 10,
    top: sorted
      .slice(0, 25)
      .map((e) => ({ ...e, timeMs: Math.round(e.timeMs) })),
  };
}

for (const target of PERF_TARGETS) {
  test(`render count ${target.name}`, async ({ browser }) => {
    test.setTimeout(240_000);
    // bypassCSP: proxy.ts setzt eine CSP; das Init-Script läuft im Hauptkontext.
    const context = await browser.newContext({ bypassCSP: true });
    const page = await context.newPage();
    await page.addInitScript({ path: REACT_SCAN_BUNDLE });
    await page.addInitScript(() => {
      const agg = new Map<string, RenderEntry>();
      const w = window as ScanWindow;
      w.__perfRenders = {
        reset: () => agg.clear(),
        dump: () => [...agg.values()],
      };
      w.reactScan?.({
        enabled: true,
        showToolbar: false,
        onRender: (
          _fiber: unknown,
          renders: Array<{
            componentName: string | null;
            phase: number;
            time: number | null;
          }>,
        ) => {
          for (const render of renders) {
            const component = render.componentName ?? "(anonym)";
            const entry = agg.get(component) ?? {
              component,
              renders: 0,
              updates: 0,
              timeMs: 0,
            };
            entry.renders += 1;
            if (render.phase === 2) entry.updates += 1;
            entry.timeMs += render.time ?? 0;
            agg.set(component, entry);
          }
        },
      });
    });
    const recorder = new RequestRecorder(page);
    const dump = () =>
      page.evaluate(() => (window as ScanWindow).__perfRenders!.dump());
    try {
      await login(page, access);
      recorder.start();
      const mountStart = performance.now();
      await gotoTolerant(page, `${base}${target.path}`);
      await expect(target.ready(page)).toBeVisible({ timeout: 60_000 });
      await recorder.waitForQuiet(QUIET);
      const hooked = await page.evaluate(
        () => typeof (window as ScanWindow).reactScan === "function",
      );
      expect(hooked, "react-scan global must be installed").toBe(true);
      const mount = aggregate(await dump(), performance.now() - mountStart);

      // Leerlauf: die Seite steht, nichts wird angefasst. Renders in diesem
      // Fenster sind Timer-, SSE- oder Effekt-Schleifen.
      await page.evaluate(() => (window as ScanWindow).__perfRenders!.reset());
      const idleStart = performance.now();
      await page.waitForTimeout(10_000);
      const idle = aggregate(await dump(), performance.now() - idleStart);

      let interaction: (RenderReport & { label: string }) | null = null;
      if (target.interaction) {
        await page.evaluate(() =>
          (window as ScanWindow).__perfRenders!.reset(),
        );
        recorder.start();
        const start = performance.now();
        await target.interaction.run(page);
        await recorder.waitForQuiet(QUIET);
        interaction = {
          label: target.interaction.label,
          ...aggregate(await dump(), performance.now() - start),
        };
      }
      writeFileSync(
        join(OUT_DIR, `${target.name}.json`),
        JSON.stringify(
          { target: target.name, path: target.path, mount, idle, interaction },
          null,
          2,
        ),
      );
    } finally {
      await context.close();
    }
  });
}
