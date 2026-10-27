import { expect, test, type Page } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import {
  gotoTolerant,
  loadAccess,
  loadRoleAccess,
  login,
  tenantBaseUrl,
} from "./access";
import { RequestRecorder } from "./recorder";
import {
  INTENT_HOPS,
  NAVIGATION_HOPS,
  REVISIT_TARGETS,
  type IntentHop,
  type NavigationHop,
  type PerfRole,
  type RevisitTarget,
} from "./targets";

// Seitenwechsel-Messung (#2828). Der kalte Aufruf ist in measure.perf.ts
// gemessen; hier geht es um das, was eine Betreuungskraft den ganzen Tag tut:
// in der Seitenleiste auf die nächste Seite klicken.
//
// Was pro Wechsel festgehalten wird und warum:
//   documents       — ein Dokument-Request wäre ein echter Neuaufbau der
//                     Seite. Muss 0 sein, sonst ist es ein Full-Reload.
//   rsc             — die RSC-Nutzlast des Zielsegments (`?_rsc=`).
//   api             — Seitendaten. Die Hülle darf hier nicht auftauchen.
//   shellApi        — Aufrufe der Hüllen-Endpunkte. Bis auf die bewusst beim
//                     Seitenwechsel erneuerten Ankündigungen und das beim
//                     ersten Öffnen von Einstellungen geladene Schema muss
//                     die Liste leer sein (#2973).
//   shellSurvived   — die Hülle wurde vor dem Klick markiert; ist die Marke
//                     danach weg, hat React sie neu aufgebaut.
//   fallbacks       — welche Ladehüllen zwischendurch sichtbar waren.
//   toContentMs     — Klick bis der Zielinhalt steht.

const OUT_DIR = join(process.cwd(), "perf-results", "navigation");
const REPEATS = 3;
const CPU_THROTTLE = 4;
/** Schul-WLAN gegen einen entfernten Server: 150 ms RTT, 3 Mbit/s. */
const NETWORK = {
  latency: 150,
  downloadThroughput: (3 * 1024 * 1024) / 8,
  uploadThroughput: (1 * 1024 * 1024) / 8,
} as const;
const QUIET = { quietMs: 1500, capMs: 25_000 } as const;

const access = loadAccess();
const base = tenantBaseUrl(access);

/** Endpunkte, die die Hülle beim vollen Seitenaufruf einmal lädt (#2973). */
const SHELL_ENDPOINTS = [
  "/api/user-context",
  "/api/me/profile",
  "/api/me/navigation",
  "/api/settings/schema",
  "/api/reminders",
  "/api/platform/announcements/unread",
  "/api/messages/unread-count",
  "/api/staff-messages/unread-count",
  "/api/staff-notices/today",
  "/api/staff/absences/pending",
  "/api/students/change-requests/pending-count",
  "/api/enrollment/admin/change-requests/pending-count",
  "/api/students/care-withdrawals",
  "/api/groups/context",
  "/api/active/supervisors/all",
  "/api/active/schulhof/status",
  "/api/auth/account-tenants",
  "/api/auth/session",
];
const ALLOWED_NAVIGATION_SHELL_ENDPOINTS = new Set([
  // Eine neue Ankündigung erscheint bewusst beim Seitenwechsel statt mitten
  // in einer Eingabe. Das Schema wird beim ersten Öffnen von Einstellungen
  // nachgeladen und danach im Cache gehalten.
  "/api/platform/announcements/unread",
  "/api/settings/schema",
]);

interface NavProbeWindow {
  __navProbe?: { fallbacks: string[]; observer: MutationObserver | null };
}

interface HopResult {
  from: string;
  to: string;
  label: string;
  toContentMs: number;
  settleMs: number;
  shellSurvived: boolean;
  documents: number;
  rsc: number;
  api: number;
  shellApi: string[];
  fallbacks: string[];
  paths: string[];
}

function expectNavigationInvariants(result: HopResult): void {
  const transition = `${result.from} → ${result.to}`;
  expect(result.documents, `${transition}: kein Dokument-Reload`).toBe(0);
  expect(
    result.shellSurvived,
    `${transition}: Portalhülle bleibt erhalten`,
  ).toBe(true);
  expect(
    result.shellApi.filter(
      (endpoint) => !ALLOWED_NAVIGATION_SHELL_ENDPOINTS.has(endpoint),
    ),
    `${transition}: keine wiederholten Hüllen-Endpunkte`,
  ).toEqual([]);
  expect(
    result.fallbacks.filter((fallback) => fallback.startsWith("loading:")),
    `${transition}: kein allgemeiner Ladeplatzhalter`,
  ).toEqual([]);
}

/**
 * Markiert die Hülle und beobachtet, welche Ladehüllen auftauchen. Beides
 * direkt vor dem Klick; die Marke ist ein Attribut auf einem DOM-Knoten, den
 * React nur bei einem Neuaufbau ersetzt.
 */
async function armProbe(page: Page): Promise<void> {
  await page.evaluate(() => {
    const probeWindow = window as NavProbeWindow;
    probeWindow.__navProbe?.observer?.disconnect();
    const state = {
      fallbacks: [] as string[],
      observer: null as MutationObserver | null,
    };
    probeWindow.__navProbe = state;

    document
      .querySelector("[data-portal-background]")
      ?.setAttribute("data-nav-probe", "armed");

    const note = (element: Element) => {
      if (element.matches('output[aria-busy="true"]')) {
        state.fallbacks.push(
          `loading:${element.getAttribute("aria-label") ?? ""}`,
        );
      } else if (element.matches(".animate-pulse")) {
        state.fallbacks.push("skeleton");
      } else if (element.matches(".moto-nav-progress")) {
        state.fallbacks.push("progress-bar");
      } else {
        return;
      }
    };

    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        for (const node of mutation.addedNodes) {
          if (!(node instanceof Element)) continue;
          note(node);
          for (const child of node.querySelectorAll(
            'output[aria-busy="true"], .animate-pulse, .moto-nav-progress',
          )) {
            note(child);
          }
        }
      }
    });
    observer.observe(document.body, { childList: true, subtree: true });
    state.observer = observer;
  });
}

async function readProbe(
  page: Page,
): Promise<{ shellSurvived: boolean; fallbacks: string[] }> {
  return page.evaluate(() => {
    const probeWindow = window as NavProbeWindow;
    const state = probeWindow.__navProbe;
    state?.observer?.disconnect();
    const shellSurvived =
      document
        .querySelector("[data-portal-background]")
        ?.getAttribute("data-nav-probe") === "armed";
    // Mehrfach dasselbe Skelett zählt einmal; interessant ist, WAS zu sehen war.
    const fallbacks = [...new Set(state?.fallbacks ?? [])];
    return { shellSurvived, fallbacks };
  });
}

/** „3x /api/…“ je Pfad, absteigend — die Rohliste eines Wechsels. */
function countByPath(records: readonly { path: string }[]): string[] {
  const counts = new Map<string, number>();
  for (const record of records) {
    const path = record.path.replace(/\?.*$/, "");
    counts.set(path, (counts.get(path) ?? 0) + 1);
  }
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([path, count]) => `${count}x ${path}`);
}

/** Der Seitenleisten-Link; liegt er in einer zugeklappten Gruppe, aufklappen. */
async function sidebarLink(page: Page, hop: NavigationHop) {
  if (hop.group) {
    // Ein zugeklappter Bereich behält seine Links im DOM. Sie gelten als
    // sichtbar, den Klick fängt aber der Bereichskopf ab.
    const group = page
      .getByRole("button", { name: hop.group, exact: true })
      .first();
    if ((await group.getAttribute("aria-expanded")) === "false") {
      await group.click();
    }
  }
  const sidebar = page.locator("nav").filter({ has: page.getByRole("link") });
  const link = page
    .getByRole("link", { name: hop.linkName, exact: true })
    .filter({ visible: true })
    .first();
  if (await link.count()) return link;
  return sidebar.getByRole("link", { name: hop.linkName }).first();
}

async function measureHop(
  page: Page,
  recorder: RequestRecorder,
  from: string,
  hop: NavigationHop,
): Promise<HopResult> {
  const link = await sidebarLink(page, hop);
  await expect(link).toBeVisible({ timeout: 15_000 });

  await armProbe(page);
  recorder.start();
  const started = performance.now();
  await link.click();
  // PERF_NAV_SCREENSHOTS=1 hält den Moment kurz nach dem Klick fest — dort
  // ist zu sehen, was während des Wechsels auf dem Schirm steht.
  if (process.env.PERF_NAV_SCREENSHOTS === "1") {
    // 250 ms: der Balken blendet sich nach 150 ms ein, vorher ist absichtlich
    // nichts zu sehen.
    await page.waitForTimeout(250);
    await page.screenshot({
      path: join(
        OUT_DIR,
        `waehrend-wechsel${hop.path.replaceAll("/", "-")}.png`,
      ),
    });
  }
  await expect(hop.ready(page)).toBeVisible({ timeout: 30_000 });
  const toContentMs = performance.now() - started;
  await recorder.waitForQuiet(QUIET);
  const settleMs = performance.now() - started;
  const probe = await readProbe(page);

  const records = recorder.records.filter((record) => !record.noise);
  const apiRecords = records.filter((record) =>
    record.path.startsWith("/api/"),
  );
  return {
    from,
    to: hop.path,
    label: hop.linkName,
    toContentMs: Math.round(toContentMs),
    settleMs: Math.round(settleMs),
    shellSurvived: probe.shellSurvived,
    fallbacks: probe.fallbacks,
    documents: records.filter((record) => record.resourceType === "document")
      .length,
    rsc: records.filter((record) => record.path.includes("_rsc=")).length,
    api: apiRecords.length,
    shellApi: apiRecords
      .map((record) => record.path.split("?")[0] ?? record.path)
      .filter((path) => SHELL_ENDPOINTS.includes(path)),
    paths: countByPath(records),
  };
}

test.describe.configure({ mode: "serial" });

test.beforeAll(() => {
  mkdirSync(OUT_DIR, { recursive: true });
});

test("login", async ({ page }) => {
  await login(page, access);
  await page
    .context()
    .storageState({ path: join(OUT_DIR, "storage-state.json") });
});

test("measure sidebar navigation", async ({ browser }) => {
  test.setTimeout(REPEATS * 180_000);
  const storageState = join(OUT_DIR, "storage-state.json");

  for (let repeat = 0; repeat < REPEATS; repeat += 1) {
    const context = await browser.newContext({
      storageState,
      serviceWorkers: "block",
    });
    const page = await context.newPage();
    const cdp = await context.newCDPSession(page);
    await cdp.send("Emulation.setCPUThrottlingRate", { rate: CPU_THROTTLE });
    // Ohne Leitungsbremse ist jeder Wechsel gegen ein lokales Backend fertig,
    // bevor irgendeine Ladehülle sichtbar wird — gemessen würde dann etwas,
    // das keine Schule je erlebt. NETWORK entspricht grob einem Schul-WLAN
    // gegen einen entfernten Server.
    await cdp.send("Network.emulateNetworkConditions", {
      offline: false,
      ...NETWORK,
    });
    const recorder = new RequestRecorder(page);
    const results: HopResult[] = [];
    try {
      // Erster Aufruf normal laden; gemessen wird erst ab dem zweiten
      // Schritt. Das Warten auf Ruhe ist Pflicht: sonst laufen die Requests
      // des Seitenaufrufs noch, wenn der erste Wechsel startet, und werden
      // ihm zugerechnet.
      await gotoTolerant(page, `${base}${NAVIGATION_HOPS[0]!.path}`);
      await expect(NAVIGATION_HOPS[0]!.ready(page)).toBeVisible({
        timeout: 30_000,
      });
      recorder.start();
      await recorder.waitForQuiet(QUIET);
      let from = NAVIGATION_HOPS[0]!.path;
      for (const hop of NAVIGATION_HOPS.slice(1)) {
        const result = await measureHop(page, recorder, from, hop);
        expectNavigationInvariants(result);
        results.push(result);
        from = hop.path;
      }
    } finally {
      writeFileSync(
        join(OUT_DIR, `navigation.${repeat}.json`),
        JSON.stringify(results, null, 2),
      );
      await context.close();
    }
  }
});

// Zeigen vor dem Klick (#4987). Je Ziel ein frischer Kontext, damit der Klick
// kalt ist: kein Modul der Zielseite im Cache, kein Eintrag im Router.
// Beide Varianten warten INTENT_WINDOW_MS vor dem Klick; „hover“ hält die Maus
// dabei auf dem Link, „click“ lässt sie liegen. Was im Fenster angefragt
// wird, sind die Kosten des Vorladens — bei „click“ muss es leer sein.
const INTENT_WINDOW_MS = 300;
const INTENT_MODES = ["click", "hover"] as const;
/**
 * Vor dem Fenster muss die Ausgangsseite wirklich ruhen: unter 4x CPU liegen
 * zwischen zwei Daten-Stufen der Startseite mehr als 1,5 s ohne Request.
 */
const INTENT_SETTLE = { quietMs: 3000, capMs: 30_000 } as const;
/** Nach dem Klick reicht ein kürzeres Fenster: gezählt wird nur der Wechsel. */
const INTENT_QUIET = { quietMs: 1500, capMs: 10_000 } as const;

interface RequestCost {
  requests: number;
  rsc: number;
  scripts: number;
  api: number;
  kB: number;
}

interface IntentResult {
  name: string;
  mode: (typeof INTENT_MODES)[number];
  repeat: number;
  toContentMs: number;
  /** Klick bis zum ersten Daten-Request der Zielseite. */
  firstApiMs: number | null;
  settleMs: number;
  /** Requests im Fenster vor dem Klick. */
  window: RequestCost;
  /** Requests ab dem Klick bis zur Ruhe. */
  afterClick: RequestCost;
  documents: number;
  shellSurvived: boolean;
  /** Hüllen-Endpunkte, die der Wechsel erneut abruft. */
  shellApi: string[];
  fallbacks: string[];
  windowPaths: string[];
  afterClickPaths: string[];
  /** RSC und JS relativ zum Klick: „start..ende pfad“ in ms. */
  timeline: string[];
}

function requestCost(
  records: readonly {
    path: string;
    resourceType: string;
    bytes: number | null;
  }[],
): RequestCost {
  return {
    requests: records.length,
    rsc: records.filter((record) => record.path.includes("_rsc=")).length,
    scripts: records.filter((record) => record.resourceType === "script")
      .length,
    api: records.filter((record) => record.path.startsWith("/api/")).length,
    kB:
      Math.round(
        records.reduce((sum, record) => sum + (record.bytes ?? 0), 0) / 102.4,
      ) / 10,
  };
}

function median(values: readonly number[]): number {
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2
    ? sorted[middle]!
    : Math.round((sorted[middle - 1]! + sorted[middle]!) / 2);
}

async function measureIntent(
  page: Page,
  recorder: RequestRecorder,
  hop: IntentHop,
  mode: IntentResult["mode"],
  repeat: number,
): Promise<IntentResult> {
  if (hop.accordion) {
    await page.addInitScript((section) => {
      localStorage.setItem("sidebar-accordion-expanded", section);
    }, hop.accordion);
  }
  await gotoTolerant(page, `${base}${hop.from}`);
  await expect(hop.fromReady(page)).toBeVisible({ timeout: 30_000 });
  const link = hop.link(page);
  recorder.start();
  await recorder.waitForQuiet(INTENT_SETTLE);
  await expect(link).toBeVisible({ timeout: 30_000 });

  await armProbe(page);
  recorder.start();
  const windowStarted = performance.now();
  if (mode === "hover") await link.hover({ timeout: 15_000 });
  await page.waitForTimeout(INTENT_WINDOW_MS);
  const clickOffset = performance.now() - windowStarted;
  const started = performance.now();
  await link.click({ timeout: 15_000 });
  // Manche Marken stehen auch auf der Ausgangsseite; erst die Adresse zählt.
  await page.waitForURL((url) => url.pathname === hop.path, {
    timeout: 30_000,
    waitUntil: "commit",
  });
  await expect(hop.ready(page)).toBeVisible({ timeout: 30_000 });
  const toContentMs = performance.now() - started;
  await recorder.waitForQuiet(INTENT_QUIET);
  const settleMs = performance.now() - started;
  const probe = await readProbe(page);

  const records = recorder.records.filter((record) => !record.noise);
  const inWindow = records.filter((record) => record.startMs < clickOffset);
  const afterClick = records.filter((record) => record.startMs >= clickOffset);
  const firstApi = afterClick.find((record) => record.path.startsWith("/api/"));
  return {
    name: hop.name,
    mode,
    repeat,
    toContentMs: Math.round(toContentMs),
    firstApiMs: firstApi ? Math.round(firstApi.startMs - clickOffset) : null,
    settleMs: Math.round(settleMs),
    window: requestCost(inWindow),
    afterClick: requestCost(afterClick),
    documents: afterClick.filter((record) => record.resourceType === "document")
      .length,
    shellSurvived: probe.shellSurvived,
    shellApi: afterClick
      .map((record) => record.path.split("?")[0] ?? record.path)
      .filter((path) => SHELL_ENDPOINTS.includes(path)),
    fallbacks: probe.fallbacks,
    windowPaths: countByPath(inWindow),
    afterClickPaths: countByPath(afterClick),
    timeline: records
      .filter(
        (record) =>
          record.path.includes("_rsc=") || record.resourceType === "script",
      )
      .map(
        (record) =>
          `${Math.round(record.startMs - clickOffset)}..${
            record.endMs === null
              ? "offen"
              : Math.round(record.endMs - clickOffset)
          } ${record.path.replace(/\?.*$/, "")}`,
      ),
  };
}

test("measure cold click after intent", async ({ browser }) => {
  test.setTimeout(REPEATS * INTENT_HOPS.length * INTENT_MODES.length * 180_000);
  const loginState = join(OUT_DIR, "storage-state.json");
  const storageState = join(OUT_DIR, "storage-state-intent.json");

  // Der Hinweis zu Browser-Benachrichtigungen legt sich sonst über die
  // Seitenleiste. Er merkt sich das Schließen im localStorage.
  const setup = await browser.newContext({ storageState: loginState });
  const setupPage = await setup.newPage();
  await gotoTolerant(setupPage, `${base}/home`);
  const dismiss = setupPage.getByRole("button", { name: "Verstanden" });
  await dismiss.waitFor({ timeout: 10_000 }).catch(() => undefined);
  if (await dismiss.isVisible()) await dismiss.click();
  await setup.storageState({ path: storageState });
  await setup.close();

  const results: IntentResult[] = [];
  for (let repeat = 0; repeat < REPEATS; repeat += 1) {
    for (const hop of INTENT_HOPS) {
      for (const mode of INTENT_MODES) {
        const context = await browser.newContext({
          storageState,
          serviceWorkers: "block",
        });
        const page = await context.newPage();
        const cdp = await context.newCDPSession(page);
        await cdp.send("Emulation.setCPUThrottlingRate", {
          rate: CPU_THROTTLE,
        });
        await cdp.send("Network.emulateNetworkConditions", {
          offline: false,
          ...NETWORK,
        });
        try {
          const result = await measureIntent(
            page,
            new RequestRecorder(page),
            hop,
            mode,
            repeat,
          );
          expect(result.documents, `${hop.name}: kein Dokument-Reload`).toBe(0);
          expect(
            result.shellSurvived,
            `${hop.name}: Portalhülle bleibt erhalten`,
          ).toBe(true);
          results.push(result);
          // Laufend sichern: ein Abbruch kostet sonst alle bisherigen Läufe.
          writeFileSync(
            join(OUT_DIR, "intent.json"),
            JSON.stringify(results, null, 2),
          );
          console.log(
            `${hop.name} ${mode} #${repeat}: ${result.toContentMs} ms, Fenster ${result.window.requests} Requests / ${result.window.kB} kB`,
          );
        } finally {
          await context.close();
        }
      }
    }
  }
  const rows = INTENT_HOPS.flatMap((hop) =>
    INTENT_MODES.map((mode) => {
      const runs = results.filter(
        (result) => result.name === hop.name && result.mode === mode,
      );
      return {
        ziel: hop.name,
        variante: mode,
        klickBisInhaltMs: median(runs.map((run) => run.toContentMs)),
        ersterDatenRequestMs: median(
          runs.flatMap((run) =>
            run.firstApiMs === null ? [] : [run.firstApiMs],
          ),
        ),
        fensterRequests: median(runs.map((run) => run.window.requests)),
        fensterRsc: median(runs.map((run) => run.window.rsc)),
        fensterJs: median(runs.map((run) => run.window.scripts)),
        fensterKB: median(runs.map((run) => run.window.kB)),
        nachKlickRsc: median(runs.map((run) => run.afterClick.rsc)),
        nachKlickJs: median(runs.map((run) => run.afterClick.scripts)),
        nachKlickKB: median(runs.map((run) => run.afterClick.kB)),
        laeufe: runs.map((run) => run.toContentMs).join("/"),
        ladehuellen: [...new Set(runs.flatMap((run) => run.fallbacks))].join(
          ",",
        ),
      };
    }),
  );
  console.table(rows);
  writeFileSync(
    join(OUT_DIR, "intent.summary.json"),
    JSON.stringify(rows, null, 2),
  );
});

// Rückweg und erneuter Besuch (#4988). Je Ziel und Lauf ein frischer Kontext:
// Ziel öffnen („kalt“), Zwischenseite, Ziel erneut über die Seitenleiste
// („warm“), Zwischenseite, Browser-Zurück („rueckweg“). Liegen die Daten im
// Cache, darf weder warm noch Rückweg auf denselben Abruf warten.
// PERF_REVISIT=name1,name2 begrenzt die Ziele.
const REVISIT_STEPS = ["kalt", "warm", "rueckweg"] as const;

interface RevisitResult {
  name: string;
  step: (typeof REVISIT_STEPS)[number];
  repeat: number;
  toContentMs: number;
  settleMs: number;
  documents: number;
  fallbacks: string[];
  paths: string[];
}

async function revisitStep(
  page: Page,
  recorder: RecorderLike,
  target: RevisitTarget,
  action: () => Promise<unknown>,
): Promise<Omit<RevisitResult, "name" | "step" | "repeat">> {
  await armProbe(page);
  recorder.start();
  const started = performance.now();
  await action();
  await page.waitForURL((url) => url.pathname === target.path, {
    timeout: 30_000,
    waitUntil: "commit",
  });
  await expect(target.ready(page)).toBeVisible({ timeout: 30_000 });
  const toContentMs = performance.now() - started;
  await recorder.waitForQuiet(QUIET);
  const settleMs = performance.now() - started;
  const probe = await readProbe(page);
  const records = recorder.records.filter((record) => !record.noise);
  return {
    toContentMs: Math.round(toContentMs),
    settleMs: Math.round(settleMs),
    documents: records.filter((record) => record.resourceType === "document")
      .length,
    fallbacks: probe.fallbacks,
    paths: countByPath(records),
  };
}

type RecorderLike = Pick<RequestRecorder, "start" | "waitForQuiet" | "records">;

async function openVia(
  page: Page,
  recorder: RecorderLike,
  target: RevisitTarget,
): Promise<void> {
  await target.via.link(page).click({ timeout: 15_000 });
  await page.waitForURL((url) => url.pathname === target.via.path, {
    timeout: 30_000,
    waitUntil: "commit",
  });
  await expect(target.via.ready(page)).toBeVisible({ timeout: 30_000 });
  recorder.start();
  await recorder.waitForQuiet(QUIET);
}

test("measure revisit", async ({ browser }) => {
  const only = process.env.PERF_REVISIT?.split(",").filter(Boolean);
  const targets = REVISIT_TARGETS.filter(
    (target) => !only || only.includes(target.name),
  );
  test.setTimeout(REPEATS * targets.length * 180_000 + 120_000);

  // Je Rolle einmal anmelden; der Hinweis zu Benachrichtigungen legt sich
  // sonst über die Seitenleiste und merkt sich das Schließen im localStorage.
  const states = new Map<PerfRole, { storageState: string; baseUrl: string }>();
  for (const role of new Set(targets.map((target) => target.role))) {
    const roleAccess = loadRoleAccess(role);
    const baseUrl = tenantBaseUrl(roleAccess);
    const state = join(OUT_DIR, `storage-state-revisit-${role}.json`);
    const setup = await browser.newContext();
    const setupPage = await setup.newPage();
    await login(setupPage, roleAccess);
    await gotoTolerant(setupPage, `${baseUrl}/home`);
    const dismiss = setupPage.getByRole("button", { name: "Verstanden" });
    await dismiss.waitFor({ timeout: 10_000 }).catch(() => undefined);
    if (await dismiss.isVisible()) await dismiss.click();
    await setup.storageState({ path: state });
    await setup.close();
    states.set(role, { storageState: state, baseUrl });
  }

  const results: RevisitResult[] = [];
  for (let repeat = 0; repeat < REPEATS; repeat += 1) {
    for (const target of targets) {
      const roleState = states.get(target.role)!;
      const context = await browser.newContext({
        storageState: roleState.storageState,
        serviceWorkers: "block",
      });
      const page = await context.newPage();
      const cdp = await context.newCDPSession(page);
      await cdp.send("Emulation.setCPUThrottlingRate", { rate: CPU_THROTTLE });
      await cdp.send("Network.emulateNetworkConditions", {
        offline: false,
        ...NETWORK,
      });
      const recorder = new RequestRecorder(page);
      try {
        await gotoTolerant(page, `${roleState.baseUrl}${target.via.path}`);
        await expect(target.via.ready(page)).toBeVisible({ timeout: 30_000 });
        recorder.start();
        await recorder.waitForQuiet(INTENT_SETTLE);

        const steps: Record<RevisitResult["step"], () => Promise<unknown>> = {
          kalt: () => target.link(page).click({ timeout: 15_000 }),
          warm: () => target.link(page).click({ timeout: 15_000 }),
          rueckweg: () => page.goBack({ waitUntil: "commit" }),
        };
        for (const step of REVISIT_STEPS) {
          const result = await revisitStep(page, recorder, target, steps[step]);
          expect(result.documents, `${target.name} ${step}: kein Reload`).toBe(
            0,
          );
          results.push({ name: target.name, step, repeat, ...result });
          console.log(
            `${target.name} ${step} #${repeat}: ${result.toContentMs} ms, Ladehüllen ${result.fallbacks.join(",") || "-"}`,
          );
          if (step !== "rueckweg") await openVia(page, recorder, target);
        }
      } finally {
        writeFileSync(
          join(OUT_DIR, "revisit.json"),
          JSON.stringify(results, null, 2),
        );
        await context.close();
      }
    }
  }

  const rows = targets.flatMap((target) =>
    REVISIT_STEPS.map((step) => {
      const runs = results.filter(
        (result) => result.name === target.name && result.step === step,
      );
      return {
        ziel: target.name,
        schritt: step,
        bisInhaltMs: median(runs.map((run) => run.toContentMs)),
        laeufe: runs.map((run) => run.toContentMs).join("/"),
        ladehuellen: [...new Set(runs.flatMap((run) => run.fallbacks))].join(
          ",",
        ),
        api: [
          ...new Set(
            runs.flatMap((run) =>
              run.paths.filter((path) => path.includes("/api/")),
            ),
          ),
        ].join(" "),
      };
    }),
  );
  console.table(rows.map(({ api: _api, ...row }) => row));
  writeFileSync(
    join(OUT_DIR, "revisit.summary.json"),
    JSON.stringify(rows, null, 2),
  );
});
