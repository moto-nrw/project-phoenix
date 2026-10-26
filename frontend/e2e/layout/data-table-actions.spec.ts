import { expect, test, type Page } from "@playwright/test";
import { compileGlobalsCss, render } from "./harness";

// Das Drei-Punkte-Menü einer Liste (#4975): Auf iPad und Handy lag es in
// `Dateien` rechts außerhalb des Bildes, erreichbar nur durch Querscrollen.
// Nachgebaut ist das Markup von `DataTable` (Tabelle und einzeilige
// Handyliste) mit den Spalten der Dateiliste; Playwright importiert kein JSX
// aus `src`. Die Unit-Tests von `DataTable` und `FilesPage` prüfen, dass die
// Komponenten diese Klassen setzen.

const LONG_NAME =
  "Elternbrief_Herbstferien_2026_Betreuungszeiten_und_Anmeldung_zur_Ferienbetreuung_final_v3.pdf";

// Die Klassen stehen im Klartext, damit Tailwinds Quellensuche sie findet.
const SURFACE =
  "moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md grow";
const CELL = "px-5 py-3 align-middle max-sm:px-3 text-gray-900";
const PINNED = "sticky right-0 bg-white";
const FILL = "w-full max-w-0 min-w-40";

let compiledCss = "";

test.beforeAll(async () => {
  compiledCss = await compileGlobalsCss();
});

function fileName(name: string): string {
  return `<span class="flex min-w-0 items-center gap-2">
    <span style="width:16px;height:16px;flex:none"></span>
    <a href="#" title="${name}" class="truncate text-sm font-medium text-gray-900">${name}</a>
  </span>`;
}

const MENU = `<button data-testid="menu" type="button" style="width:32px;height:32px">⋮</button>`;

/** Die Tabelle einer Liste in einer Karte der Breite `width`. */
function table(
  width: number,
  { fill, pinned }: { fill: boolean; pinned: boolean },
): string {
  return `
    <div style="width:${width}px" class="flex flex-col">
      <div class="${SURFACE}">
        <div data-testid="scroller" class="overflow-x-auto">
          <table class="w-full border-collapse text-left text-sm">
            <thead><tr>
              <th class="${CELL}">Datei</th>
              <th class="${CELL} whitespace-nowrap">Größe</th>
              <th class="${CELL} whitespace-nowrap">Hochgeladen</th>
              <th class="${CELL} ${pinned ? PINNED : ""}"></th>
            </tr></thead>
            <tbody><tr>
              <td class="${CELL} ${fill ? FILL : ""}">${fileName(LONG_NAME)}</td>
              <td class="${CELL} whitespace-nowrap">1,2 MB</td>
              <td class="${CELL} whitespace-nowrap">26.10.2026</td>
              <td class="${CELL} text-right ${pinned ? PINNED : ""}">${MENU}</td>
            </tr></tbody>
          </table>
        </div>
      </div>
    </div>`;
}

/** Die einzeilige Handyliste (`stackedLayout="row"`) mit dem Menü als Meta. */
function phoneRow(width: number): string {
  return `
    <div style="width:${width}px">
      <div class="${SURFACE}">
        <ul class="divide-y divide-gray-100"><li class="px-4 py-2">
          <div class="flex min-h-8 items-center gap-3">
            <div class="min-w-20 flex-1">
              <span class="line-clamp-2 break-words">${fileName(LONG_NAME)}</span>
              <div class="mt-0.5 flex min-w-0 items-center gap-1 text-sm">
                <span class="min-w-0">1,2 MB · 26.10.2026</span>
              </div>
            </div>
            <span class="flex max-w-[9.5rem] min-w-0 justify-end">${MENU}</span>
          </div>
        </li></ul>
      </div>
    </div>`;
}

/** Wie weit der Knopf über den sichtbaren Teil der Karte hinausragt. */
async function menuOverhang(page: Page, scrollerTestId: string) {
  return page.evaluate((id) => {
    const scroller = document.querySelector(`[data-testid="${id}"]`)!;
    const menu = document.querySelector('[data-testid="menu"]')!;
    return (
      menu.getBoundingClientRect().right -
      scroller.getBoundingClientRect().right
    );
  }, scrollerTestId);
}

async function sidewaysScroll(page: Page): Promise<number> {
  return page
    .getByTestId("scroller")
    .evaluate((el) => el.scrollWidth - el.clientWidth);
}

// Kartenbreiten: iPad hochkant und quer neben der Seitenleiste, Computer.
for (const width of [480, 736, 900]) {
  test(`a long file name is cut at ${width}px and the menu stays in view`, async ({
    page,
  }) => {
    await render(page, compiledCss, table(width, { fill: true, pinned: true }));

    expect(await sidewaysScroll(page)).toBe(0);
    expect(await menuOverhang(page, "scroller")).toBeLessThanOrEqual(0);
    const name = page.getByRole("link");
    expect(await name.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(
      true,
    );
  });
}

test("on a phone the cut file name keeps room beside the pinned menu", async ({
  page,
}) => {
  // Die Dokumente von Kind und Mitarbeitenden bleiben auf dem Handy eine
  // Tabelle. Die gekürzte Spalte darf dort nicht auf null schrumpfen.
  await page.setViewportSize({ width: 390, height: 844 });
  await render(page, compiledCss, table(320, { fill: true, pinned: true }));

  expect(await menuOverhang(page, "scroller")).toBeLessThanOrEqual(0);
  const name = await page.getByRole("link").boundingBox();
  expect(name!.width).toBeGreaterThanOrEqual(96);
});

test("without the fill column a long file name pushes the menu out of view", async ({
  page,
}) => {
  // Gegenprobe: der Zustand vor #4975.
  await render(page, compiledCss, table(480, { fill: false, pinned: false }));

  expect(await sidewaysScroll(page)).toBeGreaterThan(0);
  expect(await menuOverhang(page, "scroller")).toBeGreaterThan(0);
});

test("the pinned action column stays in view while a wide table scrolls", async ({
  page,
}) => {
  // Jede andere Liste ohne gekürzte Spalte: die Tabelle scrollt weiter quer,
  // das Menü bleibt am rechten Rand stehen.
  await render(page, compiledCss, table(480, { fill: false, pinned: true }));

  expect(await sidewaysScroll(page)).toBeGreaterThan(0);
  expect(await menuOverhang(page, "scroller")).toBeLessThanOrEqual(0);
});

test("the phone list shows the menu beside a long file name", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await render(
    page,
    compiledCss,
    phoneRow(358).replace("<ul", '<ul data-testid="scroller"'),
  );

  expect(await menuOverhang(page, "scroller")).toBeLessThanOrEqual(0);
  expect(
    await page.evaluate(
      () => document.scrollingElement!.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});
