import { expect, test, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { compileGlobalsCss, render } from "./harness";

// Das Geburtsdatum der Anmeldeformulare (#5333): Auf dem Handy stand in der
// Monatsauswahl nur „Mo…“. Nachgebaut sind die drei Auslöser von
// `DateOfBirthPicker` (Tag / Monat / Jahr) mit den Klassen von `CustomSelect`;
// Playwright importiert kein JSX aus `src`. Gemessen wird mit Inter, der
// Schrift der App.

// Die Klassen stehen im Klartext, damit Tailwinds Quellensuche sie findet.
const TRIGGER =
  "flex items-center justify-between gap-2 rounded-lg border px-3 text-left text-sm shadow-sm moto-content-surface h-10 w-full";
const CONTAINER = "mt-1 @container";
const GRID_FIXED =
  "grid grid-cols-1 gap-2 @sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)_minmax(0,1fr)]";
const GRID_BEFORE = "grid grid-cols-3 gap-2";

// Platzhalter und längste Werte aller Sprachen der Anmeldung.
const LABELS = {
  day: ["Tag", "31", "Day", "Dzień", "День", "Dita", "Gün"],
  month: [
    "Monat",
    "September",
    "Dezember",
    "Month",
    "Miesiąc",
    "Październik",
    "Wrzesień",
    "Месяц",
    "Сентябрь",
    "Muaji",
    "Qershor",
    "Ağustos",
    "Місяць",
    "Листопад",
  ],
  year: ["Jahr", "2026", "Year", "Rok", "Год", "Viti", "Yıl", "Рік"],
} as const;

// Feldbreite in der Karte eines Kindes der Ferienanmeldung, gemessen auf der
// Seite: Seitenrand, äußere und innere Karte. 384 und 520 stehen für breitere
// Felder (Tablet, Computer), dort liegen die drei Auswahlen nebeneinander.
const FIELDS = [
  { viewport: 360, field: 252, columns: 1 },
  { viewport: 390, field: 282, columns: 1 },
  { viewport: 768, field: 384, columns: 3 },
  { viewport: 1280, field: 520, columns: 3 },
] as const;

let compiledCss = "";

test.beforeAll(async () => {
  const inter = await readFile("src/fonts/inter-latin-variable.woff2");
  compiledCss = `@font-face { font-family: Inter; src: url(data:font/woff2;base64,${inter.toString("base64")}); }
    :root { --font-inter: Inter; }
    ${await compileGlobalsCss()}`;
});

function trigger(field: string, label: string): string {
  return `<button type="button" data-testid="${field}" class="${TRIGGER}">
    <span class="min-w-0 flex-1 truncate">${label}</span>
    <span style="width:16px;height:16px;flex:none"></span>
  </button>`;
}

function picker(
  width: number,
  labels: { day: string; month: string; year: string },
  fixed: boolean,
): string {
  return `<div style="width:${width}px" class="font-sans">
    <div class="${CONTAINER}">
      <div class="${fixed ? GRID_FIXED : GRID_BEFORE}">
        ${trigger("day", labels.day)}${trigger("month", labels.month)}${trigger("year", labels.year)}
      </div>
    </div>
  </div>`;
}

async function cutLabels(page: Page): Promise<string[]> {
  await page.evaluate(() => document.fonts.ready);
  return page.evaluate(() =>
    Array.from(document.querySelectorAll("button span.truncate"))
      .filter((span) => span.scrollWidth > span.clientWidth)
      .map((span) => span.textContent ?? ""),
  );
}

async function rowCount(page: Page): Promise<number> {
  const tops = await Promise.all(
    ["day", "month", "year"].map(
      async (id) => (await page.getByTestId(id).boundingBox())!.y,
    ),
  );
  return new Set(tops).size;
}

for (const { viewport, field, columns } of FIELDS) {
  test(`a ${field}px field at ${viewport}px keeps every label whole`, async ({
    page,
  }) => {
    await page.setViewportSize({ width: viewport, height: 800 });
    const longest = Math.max(...Object.values(LABELS).map((l) => l.length));
    const cut: string[] = [];
    for (let i = 0; i < longest; i++) {
      await render(
        page,
        compiledCss,
        picker(
          field,
          {
            day: LABELS.day[i % LABELS.day.length]!,
            month: LABELS.month[i % LABELS.month.length]!,
            year: LABELS.year[i % LABELS.year.length]!,
          },
          true,
        ),
      );
      cut.push(...(await cutLabels(page)));
      expect(await rowCount(page)).toBe(columns === 1 ? 3 : 1);
    }
    expect(cut).toEqual([]);
  });
}

test("three equal columns cut the month placeholder on a phone", async ({
  page,
}) => {
  // Gegenprobe: der Zustand vor #5333.
  await page.setViewportSize({ width: 360, height: 800 });
  await render(
    page,
    compiledCss,
    picker(252, { day: "Tag", month: "Monat", year: "Jahr" }, false),
  );

  expect(await cutLabels(page)).toContain("Monat");
});
