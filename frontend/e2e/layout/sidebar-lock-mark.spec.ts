import { expect, test, type Page } from "@playwright/test";
import { compileGlobalsCss, render } from "./harness";

// Ein Unterpunkt eines nicht gebuchten Moduls trägt ein Schloss mit
// `sr-only`-Text. Fehlte dem Schloss der Bezugsrahmen, hing der absolut
// positionierte Text am Sticky-Rahmen der Seitenleiste: Die Seite wurde um
// den Überlauf der Navigation länger, Kopf und Leiste scrollten weg.
// Nachgebaut ist das Markup aus `sidebar.tsx`, `sidebar-sub-item.tsx` und
// `module-lock-mark.tsx`; Playwright importiert kein JSX aus `src`.

// Die Klassen stehen im Klartext, damit Tailwinds Quellensuche sie findet.
const LOCK_MARK_FIXED = "relative inline-flex shrink-0 items-center ml-2";
const LOCK_MARK_BEFORE = "inline-flex shrink-0 items-center ml-2";
const SUB_ITEM =
  "flex h-8 items-center justify-between overflow-hidden rounded-lg pr-3 pl-11 text-sm";

let compiledCss = "";

test.beforeAll(async () => {
  compiledCss = await compileGlobalsCss();
});

/** Shell mit Sticky-Seitenleiste, deren Navigation weit überläuft. */
function shell(lockMarkClass: string): string {
  const rows = Array.from(
    { length: 60 },
    (_, i) => `<div class="h-10">Eintrag ${i}</div>`,
  ).join("");
  return `
    <div class="relative flex min-h-screen flex-col">
      <div class="relative z-10 flex flex-1">
        <aside class="w-64">
          <div class="sticky top-0 flex h-screen flex-col">
            <nav class="flex-1 overflow-y-auto">
              ${rows}
              <a href="#" class="${SUB_ITEM}">
                <span class="truncate">Exporte</span>
                <span class="${lockMarkClass}">
                  <span class="sr-only">, nicht gebucht</span>
                </span>
              </a>
            </nav>
          </div>
        </aside>
        <main class="flex-1">Inhalt</main>
      </div>
    </div>`;
}

function documentHeight(page: Page) {
  return page.evaluate(() => document.documentElement.scrollHeight);
}

test("das Schloss verlängert die Seite nicht über die Shell hinaus", async ({
  page,
}) => {
  await render(page, compiledCss, shell(LOCK_MARK_FIXED));
  const viewport = page.viewportSize();
  expect(await documentHeight(page)).toBe(viewport?.height);
});

test("ohne Bezugsrahmen verlängert der Vorlesetext die Seite", async ({
  page,
}) => {
  await render(page, compiledCss, shell(LOCK_MARK_BEFORE));
  const viewport = page.viewportSize();
  expect(await documentHeight(page)).toBeGreaterThan(viewport?.height ?? 0);
});
