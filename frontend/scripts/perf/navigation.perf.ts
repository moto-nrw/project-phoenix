import { expect, test, type Page } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { gotoTolerant, loadAccess, login, tenantBaseUrl } from "./access";
import { RequestRecorder } from "./recorder";
import { NAVIGATION_HOPS, type NavigationHop } from "./targets";

// Seitenwechsel-Messung (#2828). Der kalte Aufruf ist in measure.perf.ts
// gemessen; hier geht es um das, was eine Betreuungskraft den ganzen Tag tut:
// in der Seitenleiste auf die nächste Seite klicken.
//
// Was pro Wechsel festgehalten wird und warum:
//   documents       — ein Dokument-Request wäre ein echter Neuaufbau der
//                     Seite. Muss 0 sein, sonst ist es ein Full-Reload.
//   rsc             — die RSC-Nutzlast des Zielsegments (`?_rsc=`).
//   api             — Seitendaten. Die Hülle darf hier nicht auftauchen.
//   shellApi        — Aufrufe der Hüllen-Endpunkte. Muss 0 sein: die Hüllen-
//                     Daten kommen einmal beim Seitenaufruf (#2973).
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
  const sidebar = page.locator("nav").filter({ has: page.getByRole("link") });
  const link = page
    .getByRole("link", { name: hop.linkName, exact: true })
    .filter({ visible: true })
    .first();
  if (await link.count()) return link;
  if (hop.group) {
    await page
      .getByRole("button", { name: hop.group, exact: false })
      .first()
      .click();
    return page
      .getByRole("link", { name: hop.linkName, exact: true })
      .filter({ visible: true })
      .first();
  }
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
        results.push(await measureHop(page, recorder, from, hop));
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
