import { expect, test, type Browser, type Page } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { gotoTolerant, loadAccess, login, tenantBaseUrl } from "./access";
import { metricsBearerToken } from "./env.mjs";
import {
  installVitalsObserver,
  readVitals,
  RequestRecorder,
  summarizeRequests,
  type RequestSummary,
  type Vitals,
} from "./recorder";
import { PERF_TARGETS, type PerfTarget } from "./targets";

// Prod-Server-Messung (#2938): pro Screen ein kalter und ein warmer Aufruf
// plus eine Interaktion, jeweils REPEATS-mal, alles in perf-results/.
// Die Datei läuft NUR über playwright.perf.config.ts (webServer = build+start).

const OUT_DIR = join(process.cwd(), "perf-results");
const REPEATS = 3;
/** CDP-CPU-Drosselung (4 = viermal langsamer als die Messmaschine). */
const CPU_THROTTLE = 4;
const QUIET = { quietMs: 1500, capMs: 25_000 } as const;

const access = loadAccess();
const base = tenantBaseUrl(access);

interface RunResult {
  settledBy: "quiet" | "cap";
  settleMs: number;
  vitals: Vitals;
  requests: RequestSummary;
}

interface InteractionResult {
  label: string;
  settledBy: "quiet" | "cap";
  durationMs: number;
  longTasksMs: number;
  requests: RequestSummary;
}

interface TargetResult {
  target: string;
  path: string;
  repeat: number;
  cold: RunResult;
  warm: RunResult;
  interaction: InteractionResult | null;
}

test.describe.configure({ mode: "serial" });

async function scrapeMetrics(page: Page, file: string): Promise<void> {
  const response = await page.request.get(`${base}/api/internal/metrics`, {
    headers: { Authorization: `Bearer ${metricsBearerToken()}` },
  });
  expect(response.status(), "metrics endpoint must answer 200").toBe(200);
  writeFileSync(join(OUT_DIR, file), await response.text());
}

async function assertAuthenticated(page: Page, where: string): Promise<void> {
  await expect(
    page.locator('input[type="email"]'),
    `${where}: login form visible, session lost`,
  ).toHaveCount(0);
}

async function navigate(
  page: Page,
  recorder: RequestRecorder,
  target: PerfTarget,
): Promise<RunResult> {
  recorder.start();
  const started = performance.now();
  await gotoTolerant(page, `${base}${target.path}`);
  await assertAuthenticated(page, target.path);
  await expect(target.ready(page)).toBeVisible({ timeout: 30_000 });
  const settledBy = await recorder.waitForQuiet(QUIET);
  const settleMs = performance.now() - started;
  const vitals = await readVitals(page);
  return {
    settledBy,
    settleMs: Math.round(settleMs),
    vitals,
    requests: summarizeRequests(recorder.records),
  };
}

async function interact(
  page: Page,
  recorder: RequestRecorder,
  target: PerfTarget,
): Promise<InteractionResult | null> {
  if (!target.interaction) return null;
  const before = await readVitals(page);
  const mark = recorder.mark();
  const started = performance.now();
  await target.interaction.run(page);
  const settledBy = await recorder.waitForQuiet(QUIET);
  const durationMs = performance.now() - started;
  const after = await readVitals(page);
  return {
    label: target.interaction.label,
    settledBy,
    durationMs: Math.round(durationMs),
    longTasksMs: Math.round(after.longTaskTotalMs - before.longTaskTotalMs),
    requests: summarizeRequests(recorder.since(mark)),
  };
}

async function measureOnce(
  browser: Browser,
  storageState: string,
  target: PerfTarget,
  repeat: number,
): Promise<TargetResult> {
  // Frischer Context = kalter HTTP-Cache; Service Worker aus, damit der
  // PWA-Cache die Messung nicht verfälscht.
  const context = await browser.newContext({
    storageState,
    serviceWorkers: "block",
  });
  const page = await context.newPage();
  // Schul-Hardware ist deutlich schwächer als ein Entwickler-Mac; ohne
  // Drosselung liefert der Long-Task-Beobachter hier schlicht Nullen.
  const cdp = await context.newCDPSession(page);
  await cdp.send("Emulation.setCPUThrottlingRate", { rate: CPU_THROTTLE });
  const recorder = new RequestRecorder(page);
  await installVitalsObserver(page);
  await context.tracing.start({
    screenshots: true,
    snapshots: true,
    sources: false,
  });
  try {
    const cold = await navigate(page, recorder, target);
    const interaction = await interact(page, recorder, target);
    const warm = await navigate(page, recorder, target);
    return {
      target: target.name,
      path: target.path,
      repeat,
      cold,
      warm,
      interaction,
    };
  } finally {
    await context.tracing.stop({
      path: join(OUT_DIR, "traces", `${target.name}-${repeat}.zip`),
    });
    await context.close();
  }
}

test.beforeAll(() => {
  mkdirSync(join(OUT_DIR, "traces"), { recursive: true });
});

test("login and metrics baseline", async ({ page }) => {
  await login(page, access);
  await page.context().storageState({
    path: join(OUT_DIR, "storage-state.json"),
  });
  await scrapeMetrics(page, "metrics-baseline.txt");
});

for (const target of PERF_TARGETS) {
  test(`measure ${target.name}`, async ({ browser }) => {
    test.setTimeout(REPEATS * 120_000);
    const storageState = join(OUT_DIR, "storage-state.json");
    for (let repeat = 0; repeat < REPEATS; repeat += 1) {
      const result = await measureOnce(browser, storageState, target, repeat);
      writeFileSync(
        join(OUT_DIR, `${target.name}.${repeat}.json`),
        JSON.stringify(result, null, 2),
      );
    }
  });
}

test("metrics final", async ({ page }) => {
  await page.goto(`${base}/`, { waitUntil: "domcontentloaded" });
  await scrapeMetrics(page, "metrics-final.txt");
});
