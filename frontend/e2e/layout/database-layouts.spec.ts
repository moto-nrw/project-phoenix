import { expect, test, type Page } from "@playwright/test";
import { box, compileGlobalsCss, render, tenantPage } from "./harness";

// Die Hüllen der Datenverwaltung im Browser (#3330): `DatabaseListLayout`
// (Kinder, Mitarbeitende, Räume) und `MasterDetailLayout` (Gruppen,
// Aktivitäten, Rollen …). Am Computer bekommen sie ihre Höhe von
// `useFillHeight` und scrollen in der Karte; die Wachs-Regel aus globals.css
// darf sie nicht aufblähen. Diese Datei prüft CSS-Geometrie mit nachgebautem
// Markup, nicht die Breakpoint-Auswahl der Komponenten: Playwright importiert
// kein JSX aus `src`. Die Unit-Tests der beiden Komponenten prüfen dafür
// `useIsMobile` mit beiden Rückgabewerten. Auf dem Telefon scrollt die ganze
// Seite, dort gilt die Regel.

const FRAME_HEIGHT = 500;
const ROW_COUNT = 60;

// Die Klassen stehen im Klartext, damit Tailwinds Quellensuche sie findet.
const CARD =
  "moto-content-surface moto-scroll-surface overflow-hidden rounded-2xl border shadow-sm";

let compiledCss = "";

test.beforeAll(async () => {
  compiledCss = await compileGlobalsCss();
});

function scrollList(testId: string, prefix: string, extra = ""): string {
  const rows = Array.from(
    { length: ROW_COUNT },
    (_, i) => `<div style="height:40px">${prefix} ${i + 1}</div>`,
  ).join("");
  return `<div data-testid="${testId}" class="min-h-0 flex-1 overflow-auto">${rows}${extra}</div>`;
}

/** `DatabaseListLayout` am Computer. */
function listLayout(cardClass: string): string {
  return tenantPage(`
    <div data-testid="frame" class="flex w-full flex-col" style="height:${FRAME_HEIGHT}px">
      <div data-testid="card" class="${cardClass} min-h-0 flex-1">
        <div class="flex h-full flex-col">${scrollList("list", "Kind")}</div>
      </div>
    </div>`);
}

/** `MasterDetailLayout` am Computer, mit ausgewähltem Objekt. */
function masterDetail(cardClass: string): string {
  // Eine Karte in der Objektansicht, wie sie jedes Register hat. Sie darf
  // die Zeile nicht wieder zur Spalte machen.
  const innerCard = `<section class="moto-content-surface rounded-xl border p-4">Notizen</section>`;
  return tenantPage(`
    <div data-testid="frame" class="flex w-full gap-4" style="height:${FRAME_HEIGHT}px">
      <div data-testid="list-card" class="${cardClass} shrink-0" style="width:440px">
        <div class="flex h-full flex-col">${scrollList("list", "Gruppe")}</div>
      </div>
      <div data-testid="detail-card" class="${cardClass} min-w-0 flex-1">
        <div class="flex h-full flex-col">${scrollList("detail", "Feld", innerCard)}</div>
      </div>
    </div>`);
}

/** CSS-Fixture der mobilen `DatabaseListLayout`-Ausgabe. */
function mobileListLayout(): string {
  return tenantPage(`
    <div class="flex w-full flex-col">
      <div data-testid="card" class="moto-content-surface min-h-0 flex-1 overflow-hidden rounded-2xl border shadow-sm">
        <div class="flex h-full flex-col">${scrollList("list", "Kind")}</div>
      </div>
    </div>`);
}

/** CSS-Fixture der mobilen `MasterDetailLayout`-Ausgabe; Details liegen im Drawer. */
function mobileMasterDetail(): string {
  return tenantPage(`
    <div class="flex w-full flex-col">
      <div data-testid="card" class="moto-content-surface min-h-0 flex-1 overflow-hidden rounded-2xl border shadow-sm">
        <div data-testid="list-container" class="h-full overflow-auto">${scrollList("list", "Gruppe")}</div>
      </div>
    </div>`);
}

async function overflowOf(page: Page, testId: string): Promise<number> {
  return page
    .getByTestId(testId)
    .evaluate((el) => el.scrollHeight - el.clientHeight);
}

async function expectPageScrollInsteadOfList(
  page: Page,
  listTestId: string,
): Promise<void> {
  expect(await overflowOf(page, listTestId)).toBe(0);
  expect(
    await page.evaluate(
      () => document.scrollingElement!.scrollHeight > window.innerHeight,
    ),
  ).toBe(true);
}

test("the single-column collection keeps its height and scrolls inside the card", async ({
  page,
}) => {
  await render(page, compiledCss, listLayout(CARD));

  const frame = await box(page, "frame");
  const card = await box(page, "card");
  expect(Math.round(card.height)).toBe(FRAME_HEIGHT);
  expect(card.y + card.height).toBeLessThanOrEqual(frame.y + frame.height + 1);
  expect(await overflowOf(page, "list")).toBeGreaterThan(0);
});

test("without the exit the grow rule bloats the collection card past its frame", async ({
  page,
}) => {
  // Gegenprobe: derselbe Nachbau ohne Ausstieg trifft den Fehler aus #3330.
  await render(
    page,
    compiledCss,
    listLayout(CARD.replace(" moto-scroll-surface", "")),
  );

  const frame = await box(page, "frame");
  const card = await box(page, "card");
  expect(card.height).toBeGreaterThan(frame.height);
  expect(await overflowOf(page, "list")).toBe(0);
});

test("master-detail keeps list and object side by side, each scrolling in its card", async ({
  page,
}) => {
  await render(page, compiledCss, masterDetail(CARD));

  const frame = await box(page, "frame");
  const list = await box(page, "list-card");
  const detail = await box(page, "detail-card");

  expect(detail.y).toBe(list.y);
  expect(detail.x).toBeGreaterThan(list.x + list.width);
  expect(detail.x + detail.width).toBeLessThanOrEqual(
    frame.x + frame.width + 1,
  );
  expect(Math.round(list.height)).toBe(FRAME_HEIGHT);
  expect(Math.round(detail.height)).toBe(FRAME_HEIGHT);
  expect(await overflowOf(page, "list")).toBeGreaterThan(0);
  expect(await overflowOf(page, "detail")).toBeGreaterThan(0);
});

test("without the exit master-detail stacks list and object and bloats them", async ({
  page,
}) => {
  // Gegenprobe: ohne Ausstieg macht die Regel die Zeile zur Spalte und gibt
  // der Objektansicht `flex: 1 0 auto`.
  await render(
    page,
    compiledCss,
    masterDetail(CARD.replace(" moto-scroll-surface", "")),
  );

  const list = await box(page, "list-card");
  const detail = await box(page, "detail-card");
  expect(detail.y).toBeGreaterThan(list.y);
  expect(await overflowOf(page, "detail")).toBe(0);
});

test("the single-column mobile fixture grows with its list", async ({
  page,
}) => {
  await render(page, compiledCss, mobileListLayout());

  await expectPageScrollInsteadOfList(page, "list");
});

test("the master-detail mobile fixture grows with its list", async ({
  page,
}) => {
  await render(page, compiledCss, mobileMasterDetail());

  await expectPageScrollInsteadOfList(page, "list");
  expect(await overflowOf(page, "list-container")).toBe(0);
});
