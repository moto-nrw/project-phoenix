import { expect, test, type Route } from "@playwright/test";
import { existsSync, readFileSync, statSync } from "node:fs";
import path from "node:path";

import { TEST_CLOCK_INSTANT } from "../../src/test/clock-instant";
import { selectStories } from "../../scripts/kit-pixel-lib.mjs";

// Ein Test je Story und Breite: Story im Storybook-Build öffnen, warten, bis
// Render und `play` fertig sind, ganze Seite gegen die Baseline vergleichen.
// Start nur über `pnpm run kit-pixel` (#6562).

const storybookDir = path.resolve(process.env.KIT_PIXEL_STORYBOOK_DIR ?? "");
if (!process.env.KIT_PIXEL_STORYBOOK_DIR) {
  throw new Error(
    'KIT_PIXEL_STORYBOOK_DIR fehlt. Mit "pnpm run kit-pixel baseline|compare" starten.',
  );
}

/** Erfundene Adresse: Der Build kommt per `route` von der Platte, kein Port, kein Netz. */
const ORIGIN = "http://storybook.kit-pixel.test";

const stories = selectStories(
  JSON.parse(readFileSync(path.join(storybookDir, "index.json"), "utf8")),
  {
    filter: process.env.KIT_PIXEL_FILTER || undefined,
    allStories: process.env.KIT_PIXEL_ALL_STORIES === "1",
  },
);
if (stories.length === 0) {
  throw new Error(
    `Keine Story ausgewählt (Filter: ${process.env.KIT_PIXEL_FILTER || "–"}).`,
  );
}

/**
 * Keine Übergänge, jede CSS-Animation sofort im Endzustand, schon vor dem
 * ersten Render. Ein laufender Übergang hebt ein Element auf eine eigene
 * Compositing-Ebene; je nach Zeitpunkt glättet Chrome Kanten dann anders
 * (gemessen: Schließen-X der Modals, Farbwerte ±5 zwischen zwei Läufen).
 * Playwrights `animations: "disabled"` greift erst beim Bild, zu spät dafür.
 */
const FREEZE_MOTION_STYLE = `<style data-kit-pixel>
*, *::before, *::after {
  transition: none !important;
  animation-duration: 0s !important;
  animation-delay: 0s !important;
  animation-iteration-count: 1 !important;
}
</style>`;

async function serveStorybook(route: Route): Promise<void> {
  const url = new URL(route.request().url());
  if (url.origin !== ORIGIN) {
    // Alles Fremde (Fonts, Tracking, APIs) bleibt draußen: Bilder hängen nur am Build.
    await route.abort("blockedbyclient");
    return;
  }
  const pathname = decodeURIComponent(url.pathname);
  const file = path.join(
    storybookDir,
    pathname.endsWith("/") ? `${pathname}index.html` : pathname,
  );
  if (
    !file.startsWith(storybookDir + path.sep) ||
    !existsSync(file) ||
    !statSync(file).isFile()
  ) {
    await route.fulfill({ status: 404, body: "" });
    return;
  }
  if (pathname === "/iframe.html") {
    const html = readFileSync(file, "utf8");
    await route.fulfill({
      contentType: "text/html",
      body: html.replace("</head>", `${FREEZE_MOTION_STYLE}</head>`),
    });
    return;
  }
  await route.fulfill({ path: file });
}

type StoryOutcome = { status: string; errors: string[] };
type KitPixelWindow = Window & {
  __KIT_PIXEL_OUTCOME__?: Record<string, StoryOutcome>;
};

/**
 * Hört auf Storybooks Kanal, bevor die Vorschau startet. `storyFinished`
 * markiert das Ende von Render, `play` und `afterEach`; Fehler kommen als
 * eigene Ereignisse davor. Die Phase des Renders hilft nicht: Sie steht auch
 * nach einem Fehler in `play` am Ende auf `finished`.
 */
function listenForStoryOutcome(): void {
  const outcomes: Record<string, StoryOutcome> = {};
  const errors: string[] = [];
  (window as KitPixelWindow).__KIT_PIXEL_OUTCOME__ = outcomes;
  type Channel = {
    on: (event: string, listener: (payload: never) => void) => void;
  };
  const listened = new WeakSet<Channel>();
  let current: Channel | undefined;
  Object.defineProperty(globalThis, "__STORYBOOK_ADDONS_CHANNEL__", {
    configurable: true,
    get: () => current,
    set: (next: Channel | undefined) => {
      current = next;
      if (!next || listened.has(next)) return;
      listened.add(next);
      const remember = (payload: { message?: string } | undefined) => {
        errors.push(payload?.message ?? JSON.stringify(payload));
      };
      for (const event of [
        "storyErrored",
        "storyThrewException",
        "playFunctionThrewException",
        "unhandledErrorsWhilePlaying",
        "storyMissing",
      ]) {
        next.on(event, remember);
      }
      next.on(
        "storyFinished",
        (payload: { storyId: string; status: string }) => {
          outcomes[payload.storyId] = {
            status: payload.status,
            errors: [...errors],
          };
        },
      );
    },
  });
}

test.beforeEach(async ({ context, page }) => {
  await context.route("**/*", serveStorybook);
  await page.addInitScript(listenForStoryOutcome);
  // Wie Vitest (frontend/CLAUDE.md): nur `Date` steht, Timer laufen weiter.
  await page.clock.setFixedTime(TEST_CLOCK_INSTANT);
});

for (const story of stories) {
  test(story.id, async ({ page }) => {
    await page.goto(
      `${ORIGIN}/iframe.html?id=${encodeURIComponent(story.id)}&viewMode=story`,
    );
    // `storyFinished` folgt auf Render, `play` und `afterEach`; offene
    // Zustände (Menü, Popover) entstehen in `play` und sind dann sichtbar.
    const outcome = (await page
      .waitForFunction(
        (id) => (window as KitPixelWindow).__KIT_PIXEL_OUTCOME__?.[id] ?? false,
        story.id,
      )
      .then((handle) => handle.jsonValue())) as StoryOutcome;
    // Ein Fehler in `play` meldet Storybook nur als eigenes Ereignis;
    // `storyFinished` sagt danach trotzdem „success“.
    if (outcome.status !== "success" || outcome.errors.length > 0) {
      throw new Error(
        `Story ${story.id} ist fehlgeschlagen (storyFinished: ${outcome.status}): ${outcome.errors.join(" | ") || "ohne Meldung"}`,
      );
    }

    await page.evaluate(async () => {
      await document.fonts.ready;
      await Promise.all(
        Array.from(document.images, (image) =>
          image.complete ? null : image.decode().catch(() => null),
        ),
      );
    });
    await expect(page).toHaveScreenshot(`${story.id}.png`, { fullPage: true });
  });
}
