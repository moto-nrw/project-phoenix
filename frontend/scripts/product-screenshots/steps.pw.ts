import { expect, test } from "@playwright/test";

import { runStep } from "./capture";
import { trackNetworkActivity } from "./network-activity";

// Die Vorbereitungsschritte `tippen`, `zeichnen` und `scrollen` gegen eine
// feste Seite, ohne Stack:
//   pnpm exec playwright test -c playwright.screenshots.config.ts steps\.pw

const PAGE = `
<style>
  body { margin: 0; }
  header { position: sticky; top: 0; height: 64px; background: gray; }
  .spacer { height: 1500px; }
</style>
<header>Kopfleiste</header>
<main>
  <label>E-Mail <input id="email" /></label>
  <canvas id="pad" width="400" height="160" style="width: 400px; height: 160px"></canvas>
  <div class="spacer"></div>
  <h2>Ziel</h2>
  <div class="spacer"></div>
</main>
<script>
  window.strokes = 0;
  window.points = 0;
  const pad = document.getElementById("pad");
  pad.addEventListener("pointerdown", () => { window.strokes += 1; });
  pad.addEventListener("pointermove", (event) => {
    if (event.buttons === 1) window.points += 1;
  });
</script>
`;

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

test("zeichnen zieht einen zusammenhängenden Strich über die Fläche", async ({
  page,
}) => {
  const network = trackNetworkActivity(page, () => false);
  await runStep(page, network, { zeichnen: "#pad" });
  expect(
    await page.evaluate(
      () => (window as unknown as { strokes: number }).strokes,
    ),
  ).toBe(1);
  expect(
    await page.evaluate(() => (window as unknown as { points: number }).points),
  ).toBeGreaterThan(40);
});

test.describe("auf einem Touch-Gerät", () => {
  test.use({ hasTouch: true, isMobile: true });

  test("zeichnen erreicht die Fläche auch dort", async ({ page }) => {
    const network = trackNetworkActivity(page, () => false);
    await runStep(page, network, { zeichnen: "#pad" });
    expect(
      await page.evaluate(
        () => (window as unknown as { strokes: number }).strokes,
      ),
    ).toBe(1);
  });
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
