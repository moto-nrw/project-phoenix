import { expect, test } from "@playwright/test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { GroupHeader } from "../../src/components/database/group-header";
import { compileGlobalsCss, render } from "./harness";

// Der Klapp-Knopf eines Gruppenkopfs (#6098): Der Knopf ist so breit wie die
// Zeile; der Zähler soll direkt nach dem Namen stehen, nicht am rechten Rand,
// und der Name nicht mittig. Das Produktionsmarkup kommt aus `GroupHeader`
// und damit aus `ListGroupHeader`; nur die feste Breite der Testumgebung und
// die rechte Sammelaktion gehören zum Testgerüst.

function header(width: number, title: string): string {
  const groupHeader = renderToStaticMarkup(
    createElement(GroupHeader, {
      title,
      count: 13,
      isOpen: true,
      onToggle: () => undefined,
      bulkAction: createElement(
        "button",
        { type: "button", style: { width: 32, height: 32 } },
        "⋮",
      ),
    }),
  );
  return `
    <div style="width:${width}px">${groupHeader}</div>`;
}

/** Linke und rechte Kante des Textes im Titel, nicht seiner Box. */
async function textEdges(page: import("@playwright/test").Page, title: string) {
  return page.getByText(title, { exact: true }).evaluate((el) => {
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
    const title = "Klasse 1a";
    await render(page, compiledCss, header(width, title));
    const toggle = await page
      .getByRole("button", { name: `${title} 13` })
      .boundingBox();
    const count = await page.getByText("13", { exact: true }).boundingBox();
    if (!toggle || !count) throw new Error("header is not rendered");
    const text = await textEdges(page, title);

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
  const toggle = await page
    .getByRole("button", { name: `${long} 13` })
    .boundingBox();
  const count = await page.getByText("13", { exact: true }).boundingBox();
  if (!toggle || !count) throw new Error("header is not rendered");

  const truncated = await page
    .getByText(long, { exact: true })
    .evaluate((el) => el.scrollWidth > el.clientWidth);
  expect(truncated).toBe(true);
  expect(count.width).toBeGreaterThan(20);
  expect(count.x + count.width).toBeLessThanOrEqual(toggle.x + toggle.width);
});
