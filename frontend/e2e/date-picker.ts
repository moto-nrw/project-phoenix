import { expect, type Locator, type Page } from "@playwright/test";

import { formatDate } from "../src/lib/date-helpers";

/**
 * Wählt ein Datum im Kit-Datumsfeld (ISODatePicker, seit c21828f2f8 statt
 * `<input type="date">`): Feld öffnen, bei Bedarf einen Monat weiterblättern,
 * den Tag anklicken. Der Kalender rendert per Portal, daher page-weit greifen.
 * Das Tagesraster lädt nach (lazy react-day-picker), und ein Klick während der
 * Hydration öffnet das Feld nicht, und ein Feld rendert nach der Auswahl im
 * Nachbarfeld neu; dann wird erneut geklickt.
 */
export async function pickDate(page: Page, trigger: Locator, iso: string) {
  const grid = page.locator("td[data-day]").first();
  const day = page.locator(`td[data-day="${iso}"]:not([data-outside]) button`);
  // A rerender may close the picker after opening but before the day click.
  // Bound each action so the whole interaction can reopen and retry.
  await expect(async () => {
    if ((await trigger.getAttribute("aria-expanded")) !== "true") {
      await trigger.click({ timeout: 2000 });
    }
    await expect(grid).toBeVisible({ timeout: 2000 });
    if ((await day.count()) === 0) {
      await page
        .getByRole("button", { name: "Nächster Monat" })
        .click({ timeout: 2000 });
    }
    await day.click({ timeout: 2000 });
    await expect(trigger).toContainText(formatDate(iso), { timeout: 2000 });
    await expect(trigger).toHaveAttribute("aria-expanded", "false", {
      timeout: 2000,
    });
  }).toPass({ timeout: 20000 });
}
