import { expect, test } from "@playwright/test";
import { compileGlobalsCss, render } from "./harness";

// iOS vergrößert die ganze Seite, sobald jemand in ein Feld mit Schrift unter
// 16 px tippt, und lässt sie danach vergrößert stehen. Im Chat der
// Mitarbeitenden verschwand das Schreibfeld dabei aus dem Bild. `globals.css`
// hebt Felder auf Touch-Geräten deshalb auf 16 px. Den Zoom selbst zeigt nur
// ein iPhone; die Schriftgröße lässt sich hier prüfen.

// Die Klassen stehen im Klartext, damit Tailwinds Quellensuche sie findet.
const FIELDS = `
  <textarea data-testid="textarea" class="text-sm"></textarea>
  <input data-testid="input" class="h-10 px-3 py-2 text-sm" />
  <input data-testid="plain" />
  <select data-testid="select" class="text-xs"><option>A</option></select>
  <input data-testid="large" class="text-2xl" />
  <p data-testid="text" class="text-sm">Text</p>`;

let compiledCss = "";

test.beforeAll(async () => {
  compiledCss = await compileGlobalsCss();
});

const fontSize = (element: Element) =>
  parseFloat(getComputedStyle(element).fontSize);

test.describe("auf einem Touch-Gerät", () => {
  test.use({ hasTouch: true, isMobile: true });

  test("schreiben Felder mit mindestens 16 px", async ({ page }) => {
    await render(page, compiledCss, FIELDS);

    for (const testId of ["textarea", "input", "plain", "select"]) {
      const size = await page.getByTestId(testId).evaluate(fontSize);
      expect(size, testId).toBe(16);
    }
    // Größere Schrift bleibt, Fließtext ebenfalls.
    expect(await page.getByTestId("large").evaluate(fontSize)).toBe(24);
    expect(await page.getByTestId("text").evaluate(fontSize)).toBe(14);
  });
});

test("mit der Maus bleiben Felder bei ihrer Größe", async ({ page }) => {
  await render(page, compiledCss, FIELDS);

  expect(await page.getByTestId("textarea").evaluate(fontSize)).toBe(14);
});
