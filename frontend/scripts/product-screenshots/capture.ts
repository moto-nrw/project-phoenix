import {
  chromium,
  type Browser,
  type BrowserContext,
  type Page,
} from "@playwright/test";

import { accessForShot, type Credentials, type StackAccess } from "./access";
import {
  DEVICES,
  nativeSize,
  type DeviceId,
  type DeviceSpec,
  type Portal,
} from "./devices";
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
  /** Wie lange Ladeanzeigen abklingen dürfen, bevor der Shot als kaputt gilt. */
  readonly loadTimeoutMs?: number;
  readonly log?: (message: string) => void;
}

const LOAD_TIMEOUT_MS = 20_000;
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
 * Form mit "erledigt". Ein Produkt-Screenshot zeigt die App im Alltag, nicht
 * ihren Erststart.
 */
const PRESET_DECISIONS = `
(() => {
  const setupKey = /^moto\\.[a-z]+\\.notification-setup\\.v1\\./;
  const original = Storage.prototype.getItem;
  Storage.prototype.getItem = function (key) {
    return setupKey.test(key) ? '{"done":true}' : original.call(this, key);
  };
})();
`;

const LOADING_SELECTOR = '[aria-busy="true"], .animate-pulse';

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
    await page.goto(`${origin}/`, { waitUntil: "domcontentloaded" });
    await page.waitForSelector('input[type="email"]', { timeout: 30_000 });
    await page.fill('input[type="email"]', credentials.email);
    await page.fill('input[type="password"]', credentials.password);
    await page.click('button:has-text("Anmelden")');
    await page.waitForSelector('input[type="email"]', {
      state: "detached",
      timeout: 30_000,
    });
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

async function waitForSettled(page: Page, timeoutMs: number): Promise<number> {
  try {
    await page.waitForFunction(
      (selector) => document.querySelectorAll(selector).length === 0,
      LOADING_SELECTOR,
      { timeout: timeoutMs },
    );
  } catch {
    // Die Ladeanzeigen sind nach dem Timeout noch da: wird unten gezählt.
  }
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(SETTLE_MS);
  return page.locator(LOADING_SELECTOR).count();
}

async function runStep(page: Page, step: Step): Promise<void> {
  if ("klicken" in step) {
    await page.locator(step.klicken).first().click({ timeout: 10_000 });
  } else if ("warten_auf" in step) {
    await page
      .locator(step.warten_auf)
      .first()
      .waitFor({ state: "visible", timeout: 15_000 });
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
    reducedMotion: "reduce",
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
    let loadingIndicators = await waitForSettled(page, loadTimeoutMs);
    // Vor den Schritten: ein Klick darf navigieren, die Zielseite nicht.
    const finalPath = new URL(page.url()).pathname;
    for (const step of shot.vorbereitung ?? []) {
      try {
        await runStep(page, step);
      } catch (error) {
        stepFailures.push(
          `Vorbereitung ${JSON.stringify(step)} fehlgeschlagen: ${error instanceof Error ? error.message.split("\n")[0] : String(error)}`,
        );
        break;
      }
      loadingIndicators = await waitForSettled(page, loadTimeoutMs);
    }

    const reasons = [
      ...stepFailures,
      ...findBrokenReasons({
        requestedPath,
        finalPath,
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
        loadingIndicators,
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
