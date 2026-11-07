import { expect, test } from "@playwright/test";
import { compileGlobalsCss, render } from "./harness";

// Der Klapp-Knopf eines Gruppenkopfs (#6098): Der Knopf ist so breit wie die
// Zeile; der Zähler soll direkt nach dem Namen stehen, nicht am rechten Rand,
// und der Name nicht mittig. Nachgebaut ist das Markup von `ListGroupHeader`
// mit `collapsible` und `titleAddon`, wie `GroupHeader` es in den Listen der
// Datenverwaltung rendert. Der Unit-Test von `ListGroupHeader` prüft, dass
// die Komponente diese Klassen setzt.

// Die Klassen stehen im Klartext, damit Tailwinds Quellensuche sie findet.
const BUTTON =
  "inline-flex items-center font-medium h-8 gap-1.5 rounded-md px-2.5 text-xs -ml-2 w-full min-w-0 justify-start";
const TITLE = "min-w-0 truncate text-left text-sm font-semibold text-gray-600";

function header(width: number, title: string): string {
  return `
    <div style="width:${width}px" class="flex items-center justify-between gap-2 bg-gray-50 px-4 py-2">
      <div class="flex min-w-0 flex-1 flex-wrap items-center gap-2">
        <h3 class="text-sm font-semibold text-gray-600 min-w-0 flex-1">
          <button type="button" data-testid="toggle" class="${BUTTON}">
            <svg class="h-3.5 w-3.5 shrink-0" aria-hidden="true"></svg>
            <span data-testid="title" class="${TITLE}">${title}</span>
            <span data-testid="count" class="shrink-0"><span class="inline-flex items-center rounded-full text-xs font-medium whitespace-nowrap px-2 py-0.5">13</span></span>
          </button>
        </h3>
      </div>
      <div class="shrink-0 text-xs"><button type="button" style="width:32px;height:32px">⋮</button></div>
    </div>`;
}

/** Linke und rechte Kante des Textes im Titel, nicht seiner Box. */
async function textEdges(page: import("@playwright/test").Page) {
  return page.getByTestId("title").evaluate((el) => {
    const range = document.createRange();
    range.selectNodeContents(el);
    const r = range.getBoundingClientRect();
    return { left: r.left, right: r.right };
  });
}

let compiledCss = "";

test.beforeAll(async () => {
  compiledCss = await compileGlobalsCss();
});

for (const width of [1080, 358]) {
  test(`puts the count right after a short group name at ${width}px`, async ({
    page,
  }) => {
    await render(page, compiledCss, header(width, "Klasse 1a"));
    const toggle = await page.getByTestId("toggle").boundingBox();
    const count = await page.getByTestId("count").boundingBox();
    if (!toggle || !count) throw new Error("header is not rendered");
    const text = await textEdges(page);

    // Name links im Knopf: nach Polster (10 px) und Pfeil (14 px + 6 px Lücke).
    expect(text.left - toggle.x).toBeLessThan(40);
    // Zähler direkt dahinter: nur die Lücke von 6 px, kein Leerraum.
    expect(count.x - text.right).toBeLessThan(10);
    expect(count.x + count.width).toBeLessThan(toggle.x + toggle.width / 2);
  });
}

test("truncates a long name but keeps the count visible", async ({ page }) => {
  const long =
    "Nachmittagsbetreuung Sonnenblumen und Löwenzahn, Gebäude B, Erdgeschoss links";
  await render(page, compiledCss, header(358, long));
  const toggle = await page.getByTestId("toggle").boundingBox();
  const count = await page.getByTestId("count").boundingBox();
  if (!toggle || !count) throw new Error("header is not rendered");

  const truncated = await page
    .getByTestId("title")
    .evaluate((el) => el.scrollWidth > el.clientWidth);
  expect(truncated).toBe(true);
  expect(count.width).toBeGreaterThan(20);
  expect(count.x + count.width).toBeLessThanOrEqual(toggle.x + toggle.width);
});
