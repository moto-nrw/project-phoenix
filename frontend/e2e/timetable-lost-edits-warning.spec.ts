import { expect, type Page, test } from "@playwright/test";

// #1875: before a series edit ("Alle Termine" / "Ab jetzt dauerhaft") discards
// single-occurrence edits, the planner warns and lists the affected dates.
//
// This spec drives the REAL Betreuungsplan flow (open a series instance →
// Bearbeiten → Speichern → scope dialog) but STUBS the feature-specific probe
// (GET /api/timetable/instances/edited-in-window) so the assertion is
// deterministic regardless of what single-occurrence edits happen to exist in
// the seeded data. It always cancels the warning, so no series data is mutated.
//
// Like the sibling specs it needs E2E_TEST_EMAIL / E2E_TEST_PASSWORD (skips
// otherwise) and a seeded, template-backed instance in the current week (skips
// with a clear message if none is found).

const TEST_EMAIL = process.env.E2E_TEST_EMAIL;
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD;

// The deterministic probe payload the stub returns for every window.
const STUB_OCCURRENCES = [
  {
    instance_id: 900001,
    date: "2026-05-11",
    start_time: "15:00:00",
    title: "Testtermin",
    changes: ["room", "title"],
  },
  {
    instance_id: 900002,
    date: "2026-05-18",
    start_time: "15:00:00",
    title: "Testtermin",
    changes: ["staff"],
  },
];

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

async function stubProbe(page: Page) {
  await page.route(
    "**/api/timetable/instances/edited-in-window*",
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "success",
          data: {
            count: STUB_OCCURRENCES.length,
            occurrences: STUB_OCCURRENCES,
          },
        }),
      });
    },
  );
}

/**
 * Opens the first planned, template-backed instance of the current week in the
 * edit modal and clicks Speichern. Returns true when the series scope dialog
 * ("Wiederholenden Termin ändern") appears (i.e. the instance belongs to a
 * series), false otherwise. Mutates nothing — the caller cancels afterwards.
 */
async function openSeriesScopeDialog(page: Page): Promise<boolean> {
  await page.locator("aside").getByText("Betreuungsplan").click();
  await page.waitForURL((url) => url.pathname.endsWith("/betreuungsplan"));

  // Instance blocks render as buttons showing a HH:MM time. Try the first few.
  const blocks = page.getByRole("button").filter({ hasText: /\d{1,2}:\d{2}/ });
  const count = await blocks.count();
  for (let i = 0; i < Math.min(count, 5); i++) {
    await blocks.nth(i).click();
    // The read-only detail slide-over offers "Bearbeiten" for planned rows.
    const edit = page.getByRole("button", { name: "Bearbeiten" });
    if (await edit.isVisible().catch(() => false)) {
      await edit.click();
      const save = page.getByRole("button", { name: "Speichern" });
      await save.click();
      const scopeDialog = page.getByText("Wiederholenden Termin ändern");
      if (await scopeDialog.isVisible({ timeout: 3000 }).catch(() => false)) {
        return true;
      }
      // Not a series instance (or validation blocked): back out and try next.
      await page.keyboard.press("Escape");
    } else {
      await page.keyboard.press("Escape");
    }
  }
  return false;
}

test.describe("#1875 lost single-occurrence edits warning", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(
      !TEST_EMAIL || !TEST_PASSWORD,
      "E2E_TEST_EMAIL and E2E_TEST_PASSWORD must be set",
    );
    if (!TEST_EMAIL || !TEST_PASSWORD) return;
    await stubProbe(page);
    await login(page, TEST_EMAIL, TEST_PASSWORD);
  });

  test("warns before 'Alle Termine der Serie' and cancels without saving", async ({
    page,
  }) => {
    const opened = await openSeriesScopeDialog(page);
    test.skip(
      !opened,
      "no template-backed instance available in the seeded week",
    );

    await page.getByRole("button", { name: /Alle Termine der Serie/ }).click();

    // Deterministic warning from the stubbed probe, listing the concrete dates.
    await expect(
      page.getByText("Einzelanpassungen gehen verloren"),
    ).toBeVisible();
    await expect(page.getByText("11.05.2026")).toBeVisible();
    await expect(page.getByText("18.05.2026")).toBeVisible();

    // Cancel — nothing is written.
    await page.getByRole("button", { name: "Abbrechen" }).click();
    await expect(
      page.getByText("Einzelanpassungen gehen verloren"),
    ).toBeHidden();
  });

  test("warns before 'Ab jetzt dauerhaft' and cancels without saving", async ({
    page,
  }) => {
    const opened = await openSeriesScopeDialog(page);
    test.skip(
      !opened,
      "no template-backed instance available in the seeded week",
    );

    await page.getByRole("button", { name: /Ab jetzt dauerhaft/ }).click();

    await expect(
      page.getByText("Einzelanpassungen gehen verloren"),
    ).toBeVisible();
    await expect(page.getByText("11.05.2026")).toBeVisible();

    await page.getByRole("button", { name: "Abbrechen" }).click();
    await expect(
      page.getByText("Einzelanpassungen gehen verloren"),
    ).toBeHidden();
  });
});
