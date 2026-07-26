import { expect, type Page, test } from "@playwright/test";
import { toISODate } from "../src/lib/date-helpers";

// Test credentials from environment variables (never commit real credentials!).
// No fallback defaults per the repo no-fallback policy — the suite skips when
// either is unset (see beforeEach) rather than substituting an empty string.
const TEST_EMAIL = process.env.E2E_TEST_EMAIL;
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD;

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}

async function login(page: Page, email: string, password: string) {
  await page.goto("/");
  await page.waitForSelector('input[type="email"]');
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', password);
  await page.click('button:has-text("Anmelden")');
  await page.waitForURL(
    (url) => !url.pathname.includes("login") && url.pathname !== "/",
    { timeout: 15000 },
  );
}

test.describe("Personal calendar flow", () => {
  test.beforeEach(async ({ page }) => {
    // Skip if no credentials configured
    test.skip(
      !TEST_EMAIL || !TEST_PASSWORD,
      "E2E_TEST_EMAIL and E2E_TEST_PASSWORD must be set",
    );
    // Redundant at runtime (test.skip already aborted) but narrows the
    // string | undefined env reads to string for the login call.
    if (!TEST_EMAIL || !TEST_PASSWORD) return;
    await login(page, TEST_EMAIL, TEST_PASSWORD);
  });

  test("staff creates, accepts, and sees a recurring all-staff appointment", async ({
    page,
  }) => {
    const today = new Date();
    const startDate = toISODate(today);
    const recurrenceEnd = toISODate(addDays(today, 7));
    const title = `Playwright Kalendertermin ${Date.now()}`;

    await page.goto("/calendar");
    await expect(
      page.getByRole("heading", { name: "Mein Kalender" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "Neuer Termin" }).click();
    await expect(
      page.getByRole("heading", { name: "Termin erstellen" }),
    ).toBeVisible();

    await page.getByLabel("Titel").fill(title);
    await page.getByLabel("Ort").fill("E2E Raum");
    await page.getByLabel("Startdatum").fill(startDate);
    await page.getByLabel("Enddatum").fill(startDate);
    await page.getByLabel("Startzeit").fill("09:00");
    await page.getByLabel("Endzeit").fill("10:00");
    await page.getByLabel("Alle Mitarbeitenden", { exact: true }).check();
    await expect(
      page.getByText("Alle Mitarbeitenden: Alle Mitarbeitenden"),
    ).toBeVisible();
    await page.getByLabel("Wiederholung").selectOption("weekly");
    await page.getByLabel("Endet am").fill(recurrenceEnd);

    const createResponse = page.waitForResponse(
      (response) =>
        response.url().includes("/api/calendar/appointments") &&
        response.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Termin speichern" }).click();
    await expect((await createResponse).ok()).toBeTruthy();

    const currentWeekEvent = page.locator("article").filter({ hasText: title });
    await expect(currentWeekEvent).toBeVisible();
    await expect(currentWeekEvent).toContainText("Offen");
    await expect(currentWeekEvent).toContainText("E2E Raum");

    const responseUpdate = page.waitForResponse(
      (response) =>
        response.url().includes("/api/calendar/recipients/") &&
        response.url().includes("/response") &&
        response.request().method() === "POST",
    );
    await currentWeekEvent.getByRole("button", { name: "Zusagen" }).click();
    await expect((await responseUpdate).ok()).toBeTruthy();
    await expect(currentWeekEvent).toContainText("Zugesagt");

    await page.getByRole("button", { name: "Nächste Woche" }).click();
    await expect(
      page.locator("article").filter({ hasText: title }),
    ).toBeVisible();
  });
});
