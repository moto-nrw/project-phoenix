import {
  chromium,
  type Browser,
  type BrowserContext,
  type Page,
} from "@playwright/test";
import { localMFACode, loginToLocalTenant } from "../../e2e/local-tenant-login";

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
  brokenShotsError,
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
  return loadingIndicators(page).then((found) => found.length);
}

/**
 * Die sichtbaren Ladeanzeigen als kurze HTML-Ausschnitte. Ein kaputter Shot
 * nennt sie, damit ein CI-Lauf sagt, was noch lädt.
 */
function loadingIndicators(page: Page): Promise<string[]> {
  return page.evaluate(
    ({ selector, dotMax }) =>
      Array.from(document.querySelectorAll(selector))
        .filter((element) => {
          if (element.matches('[aria-busy="true"]')) return true;
          if (element instanceof SVGElement) return false;
          const { width, height } = element.getBoundingClientRect();
          return width > dotMax || height > dotMax;
        })
        .map((element) => element.outerHTML.slice(0, 240)),
    { selector: LOADING_SELECTOR, dotMax: STATUS_DOT_MAX_PX },
  );
}

/** Wohin ein kaputter Shot sein Bild legt; der Workflow lädt es hoch. */
const FAILURE_DIR = "test-results/product-screenshots";

/**
 * Anmeldung im Träger-Portal (app/traeger/login). Gehört das Konto zu mehreren
 * Trägern, fragt die Seite nach dem Träger; dann gilt der vorausgewählte erste.
 * Danach verlangt das Portal den zweiten Faktor (#4651): den Code aus der
 * Mail im lokalen Mailpit, beim ersten Mal mit Einrichtung.
 */
async function loginToTraeger(
  page: Page,
  origin: string,
  credentials: Credentials,
): Promise<void> {
  const leftLogin = (url: URL) => !url.pathname.endsWith("/login");
  const choice = page.locator('input[name="organization"]').first();
  const code = page.locator('input[autocomplete="one-time-code"]').first();
  const sendCode = page.getByRole("button", {
    name: "Code an meine E-Mail senden",
  });
  await page.goto(`${origin}/login`, { waitUntil: "domcontentloaded" });
  await page.fill("#email", credentials.email);
  await page.fill("#password", credentials.password);
  let notBefore = Date.now();
  await page.click('button[type="submit"]');
  // Welcher Schritt der Anmeldung als Nächstes kommt; höchstens 30 s.
  const nextStep = async () => {
    const deadline = Date.now() + 30_000;
    while (Date.now() < deadline) {
      if (leftLogin(new URL(page.url()))) return "fertig";
      if (await choice.isVisible()) return "traeger";
      if (await code.isVisible()) return "code";
      if (await sendCode.isVisible()) return "einrichtung";
      await page.waitForTimeout(250);
    }
    throw new Error("Die Träger-Anmeldung kam nach 30 s nicht weiter.");
  };
  let step = await nextStep();
  if (step === "traeger") {
    notBefore = Date.now();
    await page.click('button[type="submit"]');
    step = await nextStep();
  }
  if (step === "fertig") return;
  if (step === "einrichtung") {
    notBefore = Date.now();
    await sendCode.click();
  }
  await code.waitFor({ timeout: 30_000 });
  await code.fill(await localMFACode(credentials.email, notBefore));
  if (step === "einrichtung") {
    await page.getByRole("button", { name: "Weiter zum Dashboard" }).click();
  }
  await page.waitForURL(leftLogin, { timeout: 30_000 });
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
    if (portal === "traeger") {
      await loginToTraeger(page, origin, credentials);
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

type Point = readonly [number, number];

/**
 * Der Schriftzug „S. Schneider“ als Federzüge: je Zug ein Startpunkt, danach
 * kubische Bézier-Abschnitte zu je drei Punkten (zwei Stützpunkte, Ziel).
 * Einheiten frei gewählt, y wächst nach unten, die Grundlinie liegt bei 28.
 * Zwischen den Zügen hebt der Stift ab: S, Punkt, S, „chneider“, i-Punkt
 * und der Schwung darunter.
 */
// prettier-ignore
const SIGNATURE_STROKES: readonly (readonly Point[])[] = [
  // S
  [[13, 9], [8, 4], [1, 7], [3, 12], [5, 16], [13, 17], [13, 22], [13, 28], [5, 31], [1, 26]],
  // .
  [[16.5, 27.6], [16.8, 27.9], [17.2, 28.2], [17.4, 27.9]],
  // S
  [[31, 10], [27, 5], [20, 8], [23, 13], [26, 17], [32, 19], [30, 24], [28, 29], [21, 29], [21, 25]],
  // chneider
  [
    [34, 26],
    // c
    [35, 22], [38, 20], [38.5, 21.5], [36, 19.5], [33.5, 24], [34.5, 27], [35.5, 29], [38, 28], [39.5, 25.5],
    // h
    [41.5, 20], [43.5, 10], [42.5, 8], [41.5, 6.5], [39.5, 10], [40, 18], [40.3, 24], [40, 28], [40.1, 28],
    [41, 22], [43.5, 20], [44.3, 22.5], [44.7, 25], [44.3, 28], [45.5, 27.5],
    // n
    [45.8, 23], [46.5, 21], [47, 21.5], [47.3, 24], [47, 28], [47.1, 28],
    [48, 22], [49.5, 20.5], [50, 22.5], [50.3, 25], [50, 28], [51.5, 27],
    // e
    [53.5, 25], [56, 23], [55, 21], [54, 20], [51.5, 22], [52, 25.5], [52.5, 28.5], [54.5, 28.5], [56, 26.5],
    // i
    [57, 24], [57.5, 21.5], [57.5, 21], [57.6, 24], [57.4, 28], [59, 27],
    // d
    [60, 25], [62.5, 20], [63, 21.5], [61, 19.5], [58.5, 24], [59.5, 27], [60.5, 29.5], [63, 27], [63.5, 22],
    [64, 15], [64.2, 9], [64, 8], [63.6, 14], [63.5, 24], [63.6, 27.5], [64, 28.5], [65, 28], [65.5, 26.5],
    // e
    [67.5, 25], [70, 23], [69, 21], [68, 20], [65.5, 22], [66, 25.5], [66.5, 28.5], [68.5, 28.5], [70, 26.5],
    // r
    [70.5, 23], [71, 21], [71.5, 21.5], [71.5, 24], [71.5, 28], [71.5, 28], [72, 24], [73, 21.5], [75, 22],
  ],
  // Punkt auf dem i
  [[57.8, 17], [58, 16.8], [58.3, 16.9], [58.4, 17.1]],
  // Schwung
  [[22, 33], [37, 30], [57, 31.5], [75, 29]],
];

/** Neigung nach rechts: oben liegt ein Punkt um so viel je Einheit weiter rechts. */
const SIGNATURE_SLANT = 0.2;
const SIGNATURE_BASELINE = 28;
/** Punkte je Bézier-Abschnitt. */
const SIGNATURE_SAMPLES = 8;
/** Anteil der Fläche, den der Schriftzug höchstens einnimmt. */
const SIGNATURE_FILL = 0.8;

function cubic(a: Point, b: Point, c: Point, d: Point, t: number): Point {
  const u = 1 - t;
  return [
    u * u * u * a[0] +
      3 * u * u * t * b[0] +
      3 * u * t * t * c[0] +
      t * t * t * d[0],
    u * u * u * a[1] +
      3 * u * u * t * b[1] +
      3 * u * t * t * c[1] +
      t * t * t * d[1],
  ];
}

/** Die Züge als Punktfolgen, geneigt, noch in den Einheiten des Schriftzugs. */
function signaturePolylines(): Point[][] {
  return SIGNATURE_STROKES.map(([start, ...rest]) => {
    if (!start) return [];
    const line: Point[] = [start];
    for (let index = 0; index + 2 < rest.length; index += 3) {
      const from = line[line.length - 1] ?? start;
      const [b, c, d] = [rest[index], rest[index + 1], rest[index + 2]];
      if (!b || !c || !d) break;
      for (let step = 1; step <= SIGNATURE_SAMPLES; step += 1) {
        line.push(cubic(from, b, c, d, step / SIGNATURE_SAMPLES));
      }
    }
    return line.map(([x, y]) => [
      x + (SIGNATURE_BASELINE - y) * SIGNATURE_SLANT,
      y,
    ]);
  });
}

/**
 * Schreibt „S. Schneider“ mit der Maus in die Fläche: mehrere Züge mit
 * abgesetztem Stift, leicht geneigt, auf die Fläche skaliert und mittig.
 * Feste Punkte, damit jedes Gerät dieselbe Form bekommt. Die Maus erreicht
 * die Fläche auch in einem Kontext mit Touch (Pointer-Events).
 */
async function drawSignature(page: Page, selector: string): Promise<void> {
  const target = page.locator(selector).first();
  await target.scrollIntoViewIfNeeded({ timeout: 10_000 });
  const box = await target.boundingBox({ timeout: 10_000 });
  if (!box) throw new Error(`${selector} hat keine sichtbare Fläche`);
  const lines = signaturePolylines();
  const all = lines.flat();
  const xs = all.map(([x]) => x);
  const ys = all.map(([, y]) => y);
  const [minX, maxX] = [Math.min(...xs), Math.max(...xs)];
  const [minY, maxY] = [Math.min(...ys), Math.max(...ys)];
  const scale = Math.min(
    (box.width * SIGNATURE_FILL) / (maxX - minX),
    (box.height * SIGNATURE_FILL) / (maxY - minY),
  );
  const left = box.x + (box.width - (maxX - minX) * scale) / 2;
  const top = box.y + (box.height - (maxY - minY) * scale) / 2;
  for (const line of lines) {
    const [first, ...rest] = line.map(([x, y]) => ({
      x: left + (x - minX) * scale,
      y: top + (y - minY) * scale,
    }));
    if (!first) continue;
    await page.mouse.move(first.x, first.y);
    await page.mouse.down();
    for (const point of rest) await page.mouse.move(point.x, point.y);
    await page.mouse.up();
  }
}

/** Wie lange ein angeklickter Reiter ausgewählt bleiben muss. */
const TAB_STABLE_MS = 1_000;
/** So lange wartet ein Klick auf die Auswahl, bevor er wiederholt wird. */
const TAB_SELECT_MS = 5_000;
const TAB_MAX_CLICKS = 3;

/**
 * Klickt das Element. Ist es ein Reiter (role="tab" ohne eigenes Menü),
 * wartet der Schritt, bis er ausgewählt ist und es eine Sekunde bleibt. Ein
 * Klick, der verloren geht oder den die Seite zurückdreht, wird wiederholt,
 * höchstens dreimal insgesamt. Sonst zeigte der Shot still den alten Reiter.
 */
async function clickStep(page: Page, selector: string): Promise<void> {
  const target = page.locator(selector).first();
  await target.click({ timeout: 10_000 });
  const isTab =
    (await target.getAttribute("role")) === "tab" &&
    (await target.getAttribute("aria-haspopup")) === null;
  if (!isTab) return;
  let clicks = 1;
  let nextClickAt = Date.now() + TAB_SELECT_MS;
  let selectedAt: number | null = null;
  for (;;) {
    const now = Date.now();
    if ((await target.getAttribute("aria-selected")) === "true") {
      selectedAt ??= now;
      if (now - selectedAt >= TAB_STABLE_MS) return;
    } else {
      const reverted = selectedAt !== null;
      selectedAt = null;
      if (reverted || now >= nextClickAt) {
        if (clicks >= TAB_MAX_CLICKS) {
          throw new Error(
            `${selector} blieb nach ${clicks} Klicks nicht ausgewählt`,
          );
        }
        await target.click({ timeout: 10_000 });
        clicks += 1;
        nextClickAt = Date.now() + TAB_SELECT_MS;
      }
    }
    await page.waitForTimeout(100);
  }
}

/** Ein Schritt der Vorbereitung; exportiert für steps.pw.ts. */
export async function runStep(
  page: Page,
  network: NetworkActivity,
  step: Step,
): Promise<void> {
  if ("klicken" in step) {
    await clickStep(page, step.klicken);
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
  } else if ("tippen" in step) {
    // Nur ins Feld schreiben: Ein Formular abzuschicken bleibt ein eigener,
    // bewusster Klick (und im Shot verboten, siehe shots.yaml).
    await page
      .locator(step.tippen.feld)
      .first()
      .fill(step.tippen.text, { timeout: 10_000 });
  } else if ("zeichnen" in step) {
    await drawSignature(page, step.zeichnen);
  } else if ("scrollen_im_bereich" in step) {
    await scrollWithinArea(page, step.scrollen_im_bereich, "oben");
  } else if ("einblenden" in step) {
    await scrollWithinArea(page, step.einblenden, "unten");
  } else if (typeof step.scrollen === "number") {
    const offset = step.scrollen;
    await page.evaluate((top) => window.scrollTo(0, top), offset);
  } else {
    await scrollToTop(page, step.scrollen);
  }
}

/**
 * Abstand zwischen fester Kopfleiste und dem Element, zu dem `scrollen`
 * führt: Die Überschrift klebt nicht an der Leiste.
 */
const SCROLL_GAP_PX = 16;

/**
 * Bringt das Element an den oberen Rand, direkt unter die Kopfleisten, die
 * beim Scrollen stehen bleiben (sticky oder fixed, oben, mindestens halb so
 * breit wie das Fenster). `scrollIntoViewIfNeeded` täte bei einem schon
 * angeschnittenen Element nichts, und der Shot zeigte je nach Gerät einen
 * anderen Ausschnitt. Liegt das Element in einem Dialog, scrollt dessen
 * eigener Bereich. Am Seitenende bleibt die Seite stehen, das Element liegt
 * dann tiefer.
 */
async function scrollToTop(page: Page, selector: string): Promise<void> {
  const target = page.locator(selector).first();
  await target.waitFor({ state: "attached", timeout: 10_000 });
  await target.evaluate((element, gap) => {
    // Im Dialog zählen nur dessen eigene Kopfzeilen; die Leiste der Seite
    // liegt dahinter.
    const scope = element.closest('[role="dialog"], dialog') ?? document.body;
    const headerBottom = Array.from(scope.querySelectorAll("*"))
      .filter((candidate) => {
        if (candidate.contains(element)) return false;
        const { position } = getComputedStyle(candidate);
        if (position !== "sticky" && position !== "fixed") return false;
        const rect = candidate.getBoundingClientRect();
        return (
          rect.top <= 1 &&
          rect.height > 0 &&
          rect.height < window.innerHeight / 3 &&
          rect.width >= window.innerWidth / 2
        );
      })
      .reduce(
        (bottom, candidate) =>
          Math.max(bottom, candidate.getBoundingClientRect().bottom),
        0,
      );
    if (element instanceof HTMLElement || element instanceof SVGElement) {
      element.style.scrollMarginTop = `${headerBottom + gap}px`;
    }
    element.scrollIntoView({ block: "start", behavior: "instant" });
  }, SCROLL_GAP_PX);
}

/** Abstand zum Rand des Bereichs bei `scrollen_im_bereich` und `einblenden`. */
const AREA_GAP_PX = 8;

/**
 * Scrollt nur den nächsten scrollbaren Bereich um das Element (Wochenraster,
 * Chatverlauf, Inhalt eines Dialogs), ohne Bereich die Seite: `oben` legt das
 * Element an den oberen Rand dieses Bereichs, `unten` seine Unterkante an den
 * unteren. Die Seite um den Bereich bleibt stehen. Am Ende des Bereichs hält
 * der Browser an, das Element liegt dann weiter innen.
 */
async function scrollWithinArea(
  page: Page,
  selector: string,
  edge: "oben" | "unten",
): Promise<void> {
  const target = page.locator(selector).first();
  await target.waitFor({ state: "attached", timeout: 10_000 });
  await target.evaluate(
    (element, { edge, gap }) => {
      let area = element.parentElement;
      while (area) {
        const { overflowY } = getComputedStyle(area);
        if (
          (overflowY === "auto" || overflowY === "scroll") &&
          area.scrollHeight > area.clientHeight
        ) {
          break;
        }
        area = area.parentElement;
      }
      const scroller = area ?? document.scrollingElement;
      if (!scroller) return;
      const view = area
        ? area.getBoundingClientRect()
        : { top: 0, bottom: window.innerHeight };
      const rect = element.getBoundingClientRect();
      scroller.scrollTop +=
        edge === "oben"
          ? rect.top - view.top - gap
          : rect.bottom - view.bottom + gap;
    },
    { edge, gap: AREA_GAP_PX },
  );
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
      if (settled.loadingIndicators > 0) {
        for (const html of await loadingIndicators(page)) {
          reasons.push(`Ladeanzeige: ${html}`);
        }
      }
      await page
        .screenshot({
          path: `${FAILURE_DIR}/kaputt-${shot.id}-${device.id}.png`,
        })
        .catch(() => undefined);
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
 * Fotografiert alle Shots auf allen ihren Geräten. Ein kaputter Shot überspringt
 * seine übrigen Geräte; am Ende wirft der Lauf alle kaputten Shots zusammen
 * (BrokenShotError) und gibt keine Teilergebnisse aus.
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
    const broken: BrokenShotError[] = [];
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
        try {
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
        } catch (error) {
          if (!(error instanceof BrokenShotError)) throw error;
          log(error.message);
          broken.push(error);
          break;
        }
      }
    }
    if (broken.length > 0) throw brokenShotsError(broken);
    return raws;
  } finally {
    await browser.close();
  }
}
