import { expect, test } from "@playwright/test";

import { runStep } from "./capture";
import { trackNetworkActivity } from "./network-activity";

// Die Vorbereitungsschritte `tippen`, `zeichnen`, `klicken` auf einen Reiter
// und die Scroll-Schritte gegen eine feste Seite, ohne Stack:
//   pnpm exec playwright test -c playwright.screenshots.config.ts steps\.pw

const PAGE = `
<style>
  body { margin: 0; }
  header { position: sticky; top: 0; height: 64px; background: gray; }
  .spacer { height: 1500px; }
  #area { height: 200px; overflow-y: auto; }
  #area li { height: 80px; }
</style>
<header>Kopfleiste</header>
<main>
  <label>E-Mail <input id="email" /></label>
  <canvas id="pad" width="400" height="160" style="width: 400px; height: 160px"></canvas>
  <div role="tablist">
    <button id="tab" role="tab" aria-selected="false">Notizen</button>
  </div>
  <ul id="area">
    <li>Eins</li><li>Zwei</li><li>Drei</li><li>Vier</li><li>Fünf</li><li>Sechs</li>
  </ul>
  <div class="spacer"></div>
  <h2>Ziel</h2>
  <div class="spacer"></div>
</main>
<script>
  window.strokes = 0;
  window.points = [];
  const pad = document.getElementById("pad");
  pad.addEventListener("pointerdown", () => { window.strokes += 1; });
  pad.addEventListener("pointermove", (event) => {
    if (event.buttons !== 1) return;
    const rect = pad.getBoundingClientRect();
    window.points.push([event.clientX - rect.left, event.clientY - rect.top]);
  });
  // Der erste Klick wählt den Reiter nur kurz aus, dann dreht die Seite ihn
  // zurück; erst der zweite Klick hält.
  window.tabClicks = 0;
  const tab = document.getElementById("tab");
  tab.addEventListener("click", () => {
    window.tabClicks += 1;
    tab.setAttribute("aria-selected", "true");
    if (window.tabClicks === 1) {
      setTimeout(() => tab.setAttribute("aria-selected", "false"), 300);
    }
  });
</script>
`;

interface Drawn {
  strokes: number;
  points: [number, number][];
}

const drawn = (page: import("@playwright/test").Page) =>
  page.evaluate(() => {
    const { strokes, points } = window as unknown as Drawn;
    return { strokes, points };
  });

test.beforeEach(async ({ page }) => {
  await page.setContent(PAGE);
});

test("tippen füllt das Feld, ohne abzuschicken", async ({ page }) => {
  const network = trackNetworkActivity(page, () => false);
  await runStep(page, network, {
    tippen: { feld: "#email", text: "jonas@example.org" },
  });
  await expect(page.locator("#email")).toHaveValue("jonas@example.org");
});

test("zeichnen schreibt mehrere Züge in die Fläche", async ({ page }) => {
  const network = trackNetworkActivity(page, () => false);
  await runStep(page, network, { zeichnen: "#pad" });
  const { strokes, points } = await drawn(page);
  // Ein Schriftzug mit abgesetztem Stift, keine einzelne Linie.
  expect(strokes).toBeGreaterThan(3);
  expect(points.length).toBeGreaterThan(40);
  const xs = points.map(([x]) => x);
  const ys = points.map(([, y]) => y);
  // Ganz in der Fläche und über den größten Teil ihrer Breite.
  expect(Math.min(...xs)).toBeGreaterThanOrEqual(0);
  expect(Math.max(...xs)).toBeLessThanOrEqual(400);
  expect(Math.min(...ys)).toBeGreaterThanOrEqual(0);
  expect(Math.max(...ys)).toBeLessThanOrEqual(160);
  expect(Math.max(...xs) - Math.min(...xs)).toBeGreaterThan(200);
});

test("zeichnen ergibt auf jedem Lauf dieselbe Form", async ({ page }) => {
  const network = trackNetworkActivity(page, () => false);
  await runStep(page, network, { zeichnen: "#pad" });
  const first = await drawn(page);
  await page.setContent(PAGE);
  await runStep(page, network, { zeichnen: "#pad" });
  expect(await drawn(page)).toEqual(first);
});

test.describe("auf einem Touch-Gerät", () => {
  test.use({ hasTouch: true, isMobile: true });

  test("zeichnen erreicht die Fläche auch dort", async ({ page }) => {
    const network = trackNetworkActivity(page, () => false);
    await runStep(page, network, { zeichnen: "#pad" });
    const { strokes, points } = await drawn(page);
    expect(strokes).toBeGreaterThan(3);
    expect(points.length).toBeGreaterThan(40);
  });
});

test("klicken wiederholt einen Reiter, den die Seite zurückdreht", async ({
  page,
}) => {
  const network = trackNetworkActivity(page, () => false);
  await runStep(page, network, { klicken: "#tab" });
  await expect(page.locator("#tab")).toHaveAttribute("aria-selected", "true");
  expect(
    await page.evaluate(
      () => (window as unknown as { tabClicks: number }).tabClicks,
    ),
  ).toBe(2);
});

test("scrollen legt das Element unter die feste Kopfleiste", async ({
  page,
}) => {
  const network = trackNetworkActivity(page, () => false);
  await runStep(page, network, { scrollen: 'role=heading[name="Ziel"s]' });
  const top = await page
    .getByRole("heading", { name: "Ziel" })
    .evaluate((element) => element.getBoundingClientRect().top);
  // 64 px Kopfleiste und 16 px Abstand
  expect(Math.round(top)).toBe(80);
});

test("scrollen_im_bereich bewegt nur den Bereich, nicht die Seite", async ({
  page,
}) => {
  const network = trackNetworkActivity(page, () => false);
  await runStep(page, network, { scrollen_im_bereich: "text=Vier" });
  const { areaTop, itemTop, pageY } = await page.evaluate(() => ({
    areaTop: document.getElementById("area")!.getBoundingClientRect().top,
    itemTop: Array.from(document.querySelectorAll("#area li"))
      .find((item) => item.textContent === "Vier")!
      .getBoundingClientRect().top,
    pageY: window.scrollY,
  }));
  expect(pageY).toBe(0);
  // 8 px Abstand zum oberen Rand des Bereichs
  expect(Math.round(itemTop - areaTop)).toBe(8);
});

test("einblenden legt die Unterkante an den Rand des Bereichs", async ({
  page,
}) => {
  const network = trackNetworkActivity(page, () => false);
  await runStep(page, network, { einblenden: "text=Fünf" });
  const { areaBottom, itemBottom } = await page.evaluate(() => ({
    areaBottom: document.getElementById("area")!.getBoundingClientRect().bottom,
    itemBottom: Array.from(document.querySelectorAll("#area li"))
      .find((item) => item.textContent === "Fünf")!
      .getBoundingClientRect().bottom,
  }));
  expect(Math.round(areaBottom - itemBottom)).toBe(8);
});
