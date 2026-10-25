import {
  chromium,
  type Browser,
  type BrowserContext,
  type Page,
} from "@playwright/test";
import { loginToLocalTenant } from "../../e2e/local-tenant-login";

import { accessForShot, type Credentials, type StackAccess } from "./access";
import {
  DEVICES,
  nativeSize,
  type DeviceId,
  type DeviceSpec,
  type Portal,
} from "./devices";
import {
  MAX_WAIT_MS,
  trackNetworkActivity,
  waitExpired,
  type NetworkActivity,
} from "./network-activity";
import {
  BrokenShotError,
  findBrokenReasons,
  isIgnoredResponse,
  type FailedResponse,
} from "./quality";
import { referenceInstant } from "./reference-time";
import { devicesFor, type Shot, type Step } from "./shot-list";

// Capture der Rohbilder (#3759): pro Shot und Gerät ein Browser-Kontext mit
// dem nativen Viewport, fester Uhr, Deutsch und hellem Modus.

export interface RawShot {
  readonly shotId: string;
  readonly device: DeviceId;
  readonly png: Buffer;
}

export interface CaptureOptions {
  readonly access: StackAccess;
  readonly shots: readonly Shot[];
  readonly now?: Date;
  /**
   * Wie lange Ladeanzeigen stehen dürfen, während die Seite nichts mehr lädt,
   * bevor der Shot als kaputt gilt (network-activity.ts).
   */
  readonly loadTimeoutMs?: number;
  readonly log?: (message: string) => void;
}

const LOAD_TIMEOUT_MS = 20_000;
/** Dasselbe für `warten_auf`: so lange darf die Seite ohne Laden still stehen. */
const STEP_IDLE_MS = 30_000;
const SETTLE_MS = 600;

/**
 * Räumt die Seite für das Bild auf: keine Animationen, kein Caret, keine
 * Scrollbalken, kein Dev-Overlay und keine Toasts. Die Toast-Host-Elemente
 * (contexts/ToastContext.tsx) tragen nur die Ebene z-[9000] als Kennzeichen.
 */
const CLEAN_CSS = `
*, *::before, *::after {
  animation: none !important;
  transition: none !important;
  caret-color: transparent !important;
  scroll-behavior: auto !important;
}
html { scrollbar-width: none !important; }
*::-webkit-scrollbar { display: none !important; }
nextjs-portal { display: none !important; }
[class*="z-[9000]"] { display: none !important; }
`;

/**
 * Die Benachrichtigungs-Einrichtung (components/notifications) öffnet beim
 * ersten Besuch einen Dialog, solange der Browser keine gespeicherte
 * Entscheidung hat. Der Schlüssel enthält die Konto-ID, die vorher niemand
 * kennt; deshalb beantwortet dieses Skript die Frage für jeden Schlüssel dieser
 * Form mit "erledigt". Dasselbe gilt für die Erste-Schritte-Liste der Eltern
 * (components/parent/onboarding), die sich beim ersten Besuch über den Inhalt
 * legt: Sie gilt als ausgeblendet. Die Erste-Schritte-Liste der Betreuer
 * (components/staff-onboarding) geht einmal je Sitzung von selbst auf; sie
 * gilt als schon geöffnet und bleibt damit zugeklappt. Ein Produkt-Screenshot
 * zeigt die App im Alltag, nicht ihren Erststart.
 */
const PRESET_DECISIONS = `
(() => {
  const setupKey = /^moto\\.[a-z]+\\.notification-setup\\.v1\\./;
  const parentFirstStepsKey = /^parent-first-steps:v1:/;
  const staffOnboardingOpenedKey = /^staff-onboarding-opened:/;
  const original = Storage.prototype.getItem;
  Storage.prototype.getItem = function (key) {
    if (setupKey.test(key)) return '{"done":true}';
    if (parentFirstStepsKey.test(key)) return '{"dismissed":true}';
    if (staffOnboardingOpenedKey.test(key)) return "1";
    return original.call(this, key);
  };
})();
`;

const LOADING_SELECTOR = '[aria-busy="true"], .animate-pulse';

/**
 * Pulsierende Status-Punkte ("Belegt", "eingestempelt") tragen dieselbe
 * Klasse wie Skelette, laden aber nichts. Sie sind höchstens so groß.
 */
const STATUS_DOT_MAX_PX = 8;

/**
 * Sichtbare Ladeanzeigen auf der Seite, ohne Status-Punkte und ohne
 * pulsierende Symbole: Ein Symbol (svg) pulsiert, wenn eine Ankunft oder
 * Abholung bald fällig ist; Skelette sind Flächen, keine Symbole.
 */
function countLoadingIndicators(page: Page): Promise<number> {
  return page.evaluate(
    ({ selector, dotMax }) =>
      Array.from(document.querySelectorAll(selector)).filter((element) => {
        if (element.matches('[aria-busy="true"]')) return true;
        if (element instanceof SVGElement) return false;
        const { width, height } = element.getBoundingClientRect();
        return width > dotMax || height > dotMax;
      }).length,
    { selector: LOADING_SELECTOR, dotMax: STATUS_DOT_MAX_PX },
  );
}

async function login(
  context: BrowserContext,
  portal: Portal,
  origin: string,
  credentials: Credentials,
): Promise<void> {
  const page = await context.newPage();
  try {
    if (portal === "eltern") {
      await page.goto(`${origin}/login`, { waitUntil: "domcontentloaded" });
      await page.fill("#parent-email", credentials.email);
      await page.fill("#parent-password", credentials.password);
      await page.click('button[type="submit"]');
      await page.waitForURL((url) => !url.pathname.startsWith("/login"), {
        timeout: 30_000,
      });
      return;
    }
    // `domcontentloaded`: der Dev-Server hält SSE-Verbindungen offen, das
    // `load`-Ereignis kann dadurch hängen.
    await loginToLocalTenant(page, origin, credentials);
  } finally {
    await page.close();
  }
}

/** Meldet einmal je Portal und Rolle an; die Geräte teilen die Sitzung. */
async function sessionFor(
  browser: Browser,
  cache: Map<string, Awaited<ReturnType<BrowserContext["storageState"]>>>,
  access: StackAccess,
  shot: Shot,
) {
  const key = `${shot.portal}:${shot.rolle}`;
  const cached = cache.get(key);
  if (cached) return cached;
  const { origin, credentials } = accessForShot(access, shot);
  const context = await browser.newContext({ locale: "de-DE" });
  try {
    await login(context, shot.portal, origin, credentials);
    const state = await context.storageState();
    cache.set(key, state);
    return state;
  } finally {
    await context.close();
  }
}

/**
 * Bilder im Ausschnitt, die noch nicht geladen sind. Bilder laden über
 * next/image mit loading="lazy"; ohne dieses Warten zeigte ein Shot leere
 * Flächen. Ein Avatarbild, das nicht lädt, ersetzt die Avatar-Komponente
 * durch Initialen; es zählt hier also nur, solange es unterwegs ist.
 */
function countUnloadedImages(page: Page): Promise<number> {
  return page.evaluate(
    () =>
      Array.from(document.images).filter((image) => {
        const { top, bottom, left, right, width, height } =
          image.getBoundingClientRect();
        const inView =
          width > 0 &&
          height > 0 &&
          bottom > 0 &&
          right > 0 &&
          top < window.innerHeight &&
          left < window.innerWidth;
        return inView && (!image.complete || image.naturalWidth === 0);
      }).length,
  );
}

interface Settled {
  readonly loadingIndicators: number;
  readonly unloadedImages: number;
}

/**
 * Prüft `done`, bis es zutrifft. Gibt auf, wenn die Seite `idleMs` lang keine
 * Anfrage an den Stack offen hatte, spätestens nach MAX_WAIT_MS. Liefert, ob
 * `done` eingetreten ist.
 */
async function waitWhileLoading(
  page: Page,
  network: NetworkActivity,
  idleMs: number,
  done: () => Promise<boolean>,
): Promise<boolean> {
  const startedAt = Date.now();
  const limits = { idleMs, maxMs: MAX_WAIT_MS };
  while (!(await done())) {
    if (waitExpired(Date.now(), startedAt, network.state(), limits)) {
      return false;
    }
    await page.waitForTimeout(200);
  }
  return true;
}

async function waitForSettled(
  page: Page,
  network: NetworkActivity,
  idleMs: number,
): Promise<Settled> {
  await waitWhileLoading(
    page,
    network,
    idleMs,
    // Erst nach den Ladeanzeigen: vorher stehen die Bilder noch nicht im DOM.
    async () =>
      (await countLoadingIndicators(page)) === 0 &&
      (await countUnloadedImages(page)) === 0,
  );
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(SETTLE_MS);
  return {
    loadingIndicators: await countLoadingIndicators(page),
    unloadedImages: await countUnloadedImages(page),
  };
}

async function runStep(
  page: Page,
  network: NetworkActivity,
  step: Step,
): Promise<void> {
  if ("klicken" in step) {
    await page.locator(step.klicken).first().click({ timeout: 10_000 });
  } else if ("warten_auf" in step) {
    // Nach einem Klick auf eine andere Seite kompiliert der Dev-Server deren
    // Route und Datenabfragen beim ersten Aufruf; solange die Seite lädt,
    // wartet der Schritt weiter.
    const target = page.locator(step.warten_auf).first();
    const startedAt = Date.now();
    const visible = await waitWhileLoading(page, network, STEP_IDLE_MS, () =>
      target.isVisible(),
    );
    if (!visible) {
      const seconds = Math.round((Date.now() - startedAt) / 1000);
      throw new Error(
        `${step.warten_auf} nach ${seconds} s nicht sichtbar (${network.state().inFlight} Anfragen offen)`,
      );
    }
  } else if (typeof step.scrollen === "number") {
    const offset = step.scrollen;
    await page.evaluate((top) => window.scrollTo(0, top), offset);
  } else {
    await page
      .locator(step.scrollen)
      .first()
      .scrollIntoViewIfNeeded({ timeout: 10_000 });
  }
}

async function captureOne(
  browser: Browser,
  storageState: Awaited<ReturnType<BrowserContext["storageState"]>>,
  device: DeviceSpec,
  shot: Shot,
  origin: string,
  now: Date,
  loadTimeoutMs: number,
): Promise<Buffer> {
  const context = await browser.newContext({
    storageState,
    viewport: device.viewport,
    deviceScaleFactor: device.deviceScaleFactor,
    isMobile: device.isMobile,
    hasTouch: device.hasTouch,
    locale: "de-DE",
    timezoneId: "Europe/Berlin",
    colorScheme: "light",
    // Kein reducedMotion: framer-motion liest die Einstellung erst im Browser
    // (useReducedMotion ist auf dem Server null). Mit "reduce" weichen die
    // Startstile animierter Elemente vom Server-HTML ab, und die Kindersuche
    // bricht mit einem Hydration-Fehler ab. CLEAN_CSS und
    // `animations: "disabled"` halten das Bild trotzdem ruhig.
  });
  try {
    await context.addInitScript(PRESET_DECISIONS);
    // Das Skript läuft vor dem Dokument: das Element, an dem das <style>
    // hängt, gibt es erst später.
    await context.addInitScript((css) => {
      const add = () => {
        const style = document.createElement("style");
        style.textContent = css;
        document.documentElement.appendChild(style);
      };
      if (document.documentElement) {
        add();
        return;
      }
      const observer = new MutationObserver(() => {
        if (!document.documentElement) return;
        observer.disconnect();
        add();
      });
      observer.observe(document, { childList: true });
    }, CLEAN_CSS);
    // Feste Uhr: Date.now() und new Date() stehen auf dem Referenzzeitpunkt,
    // Timer laufen weiter.
    await context.clock.setFixedTime(referenceInstant(now));

    const page = await context.newPage();
    const failedResponses: FailedResponse[] = [];
    const consoleErrors: string[] = [];
    const pageErrors: string[] = [];
    const ownHost = (url: string) => {
      const host = new URL(url).hostname;
      return host === "localhost" || host.endsWith(".localhost");
    };
    const network = trackNetworkActivity(page, ownHost);
    const ignoredUrls = new Set<string>();
    page.on("response", (response) => {
      if (response.status() < 400 || !ownHost(response.url())) return;
      const failed = {
        method: response.request().method(),
        url: response.url(),
        status: response.status(),
      };
      if (isIgnoredResponse(failed)) ignoredUrls.add(failed.url);
      else failedResponses.push(failed);
    });
    page.on("console", (message) => {
      if (message.type() !== "error") return;
      if (ignoredUrls.has(message.location().url)) return;
      consoleErrors.push(message.text());
    });
    page.on("pageerror", (error) => pageErrors.push(error.message));

    const response = await page.goto(`${origin}${shot.pfad}`, {
      waitUntil: "domcontentloaded",
    });
    const requestedPath = new URL(shot.pfad, origin).pathname;
    const stepFailures: string[] = [];
    const opensDialog = (shot.vorbereitung ?? []).some(
      (step) => "klicken" in step,
    );
    let settled = await waitForSettled(page, network, loadTimeoutMs);
    // Die Umleitungsprüfung gilt der Seite, auf der der Shot landet. Ein Klick
    // der Vorbereitung darf danach weiterführen, etwa von der Kindersuche auf
    // ein Kind, dessen ID erst der Seed vergibt.
    const landedPath = new URL(page.url()).pathname;
    for (const step of shot.vorbereitung ?? []) {
      try {
        await runStep(page, network, step);
      } catch (error) {
        // Mit dem Anfang des Call-Logs: er sagt, ob der Selektor nichts fand
        // oder ein Element, das sich nicht klicken ließ.
        stepFailures.push(
          `Vorbereitung ${JSON.stringify(step)} fehlgeschlagen: ${error instanceof Error ? error.message.split("\n").slice(0, 4).join(" ").replace(/\s+/g, " ") : String(error)}`,
        );
        break;
      }
      settled = await waitForSettled(page, network, loadTimeoutMs);
    }

    const reasons = [
      ...stepFailures,
      ...findBrokenReasons({
        requestedPath,
        finalPath: landedPath,
        documentStatus: response?.status() ?? null,
        failedResponses,
        consoleErrors,
        pageErrors,
        bodyText: await page.evaluate(() => document.body.innerText),
        loginFormVisible:
          (await page.locator('input[type="password"]:visible').count()) > 0,
        devErrorOverlayPresent:
          (await page
            .locator("nextjs-portal")
            .locator("[data-nextjs-dialog], [data-nextjs-dialog-overlay]")
            .count()) > 0,
        loadingIndicators: settled.loadingIndicators,
        unloadedImages: settled.unloadedImages,
        unexpectedDialogs: opensDialog
          ? 0
          : await page.locator('[role="dialog"]:visible, dialog[open]').count(),
      }),
    ];
    if (reasons.length > 0) {
      throw new BrokenShotError(shot.id, device.id, reasons);
    }

    const png = await page.screenshot({
      type: "png",
      animations: "disabled",
      caret: "hide",
      scale: "device",
    });
    assertNativeSize(png, device, shot.id);
    return png;
  } finally {
    await context.close();
  }
}

/** Das PNG-Kopfstück (IHDR) trägt Breite und Höhe an fester Stelle. */
function pngSize(png: Buffer): { width: number; height: number } {
  return { width: png.readUInt32BE(16), height: png.readUInt32BE(20) };
}

function assertNativeSize(
  png: Buffer,
  device: DeviceSpec,
  shotId: string,
): void {
  const actual = pngSize(png);
  const expected = nativeSize(device);
  if (actual.width !== expected.width || actual.height !== expected.height) {
    throw new BrokenShotError(shotId, device.id, [
      `Das Rohbild misst ${actual.width}×${actual.height}, erwartet ${expected.width}×${expected.height}`,
    ]);
  }
}

/**
 * Fotografiert alle Shots auf allen ihren Geräten. Der erste kaputte Shot wirft
 * (BrokenShotError) und beendet den Lauf; es gibt keine Teilergebnisse.
 */
export async function captureShots(
  options: CaptureOptions,
): Promise<RawShot[]> {
  const log = options.log ?? (() => undefined);
  const now = options.now ?? new Date();
  const loadTimeoutMs = options.loadTimeoutMs ?? LOAD_TIMEOUT_MS;
  const browser = await chromium.launch();
  try {
    const sessions = new Map<
      string,
      Awaited<ReturnType<BrowserContext["storageState"]>>
    >();
    const raws: RawShot[] = [];
    for (const shot of options.shots) {
      const { origin } = accessForShot(options.access, shot);
      const storageState = await sessionFor(
        browser,
        sessions,
        options.access,
        shot,
      );
      for (const deviceId of devicesFor(shot)) {
        log(`Fotografiere ${shot.id} auf ${deviceId}`);
        const png = await captureOne(
          browser,
          storageState,
          DEVICES[deviceId],
          shot,
          origin,
          now,
          loadTimeoutMs,
        );
        raws.push({ shotId: shot.id, device: deviceId, png });
      }
    }
    return raws;
  } finally {
    await browser.close();
  }
}
