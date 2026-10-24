import { expect, type Page, test } from "@playwright/test";

import {
  loginToTenant,
  MISSING_SEED_ACCESS,
  openSidebarGroup,
  seedAccess as access,
} from "./tenant-session";

// #1875: before a series edit ("Alle Termine" / "Ab jetzt dauerhaft") discards
// single-occurrence edits, the planner warns and lists the affected dates.
//
// This spec drives the REAL Betreuungsplan flow (open a series instance →
// Bearbeiten → Speichern → scope dialog) but STUBS the feature-specific probe
// (GET /api/timetable/instances/edited-in-window) so the assertion is
// deterministic regardless of what single-occurrence edits happen to exist in
// the seeded data. It always cancels the warning, so no series data is mutated.
//
// Login and tenant origin: e2e/tenant-session.ts. The spec needs a
// seeded, template-backed instance in the current week (skips with a clear
// message if none is found).

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
 * Opens the first planned, template-backed instance of the current week and
 * clicks Bearbeiten. Returns true when the series scope dialog ("Wiederholenden
 * Termin ändern") appears (i.e. the instance belongs to a series), false
 * otherwise. Since 63cee37399 the scope is chosen BEFORE the form opens; the
 * caller picks the scope, then saves the loaded series form (see
 * saveSeriesForm). Mutates nothing — the caller cancels the warning.
 */
async function openSeriesScopeDialog(page: Page): Promise<boolean> {
  // "Betreuungsplan" lives in the collapsible "Planung" sidebar group.
  await openSidebarGroup(page, "Planung");
  await page
    .locator("aside")
    .getByRole("link", { name: "Betreuungsplan", exact: true })
    .click();
  await page.waitForURL((url) => url.pathname.endsWith("/betreuungsplan"));

  // Instance blocks render as buttons showing a HH:MM time. Try the first few.
  // The grid loads after navigation; count() does not wait, so wait for the
  // first block before counting.
  const blocks = page.getByRole("button").filter({ hasText: /\d{1,2}:\d{2}/ });
  await blocks
    .first()
    .waitFor({ state: "visible", timeout: 15000 })
    .catch(() => undefined);
  const count = await blocks.count();
  const appears = (locator: ReturnType<Page["getByRole"]>, timeout: number) =>
    locator.waitFor({ state: "visible", timeout }).then(
      () => true,
      () => false,
    );
  for (let i = 0; i < Math.min(count, 5); i++) {
    await blocks.nth(i).click();
    // The read-only detail slide-over offers "Bearbeiten" for planned rows.
    const edit = page.getByRole("button", { name: "Bearbeiten" });
    if (await appears(edit, 5000)) {
      await edit.click();
      const scopeDialog = page
        .getByRole("dialog")
        .filter({ hasText: "Wiederholenden Termin ändern" });
      if (await appears(scopeDialog, 3000)) {
        return true;
      }
      // Not a series instance: the form opened directly; back out, try next.
      await page.keyboard.press("Escape");
    } else {
      await page.keyboard.press("Escape");
    }
  }
  return false;
}

/**
 * Saves the series form opened by the chosen scope, unchanged. The form is a
 * three-step wizard; Speichern only exists on the last step (#2025,
 * 2139e21072), so walk through with "Weiter" first.
 */
async function saveSeriesForm(page: Page) {
  const form = page
    .getByRole("dialog")
    .filter({ hasText: "Regeltermin bearbeiten" });
  const save = form.getByRole("button", { name: "Speichern", exact: true });
  const next = form.getByRole("button", { name: "Weiter", exact: true });
  await expect(save.or(next)).toBeVisible({ timeout: 15000 });
  for (let step = 0; step < 3 && !(await save.isVisible()); step++) {
    await next.click();
    await expect(save.or(next)).toBeVisible();
  }
  await save.click();
}

function lostEditsWarning(page: Page) {
  return page.getByRole("dialog", { name: "Einzelanpassungen gehen verloren" });
}

test.describe("#1875 lost single-occurrence edits warning", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(access === null, MISSING_SEED_ACCESS);
    if (!access) return;
    await stubProbe(page);
    await loginToTenant(page, access);
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
    await saveSeriesForm(page);

    // Deterministic warning from the stubbed probe, listing the concrete dates.
    await expect(
      page.getByText("Einzelanpassungen gehen verloren"),
    ).toBeVisible();
    await expect(page.getByText("11.05.2026")).toBeVisible();
    await expect(page.getByText("18.05.2026")).toBeVisible();

    // Cancel — nothing is written. Scoped to the warning: the series form
    // underneath has its own "Abbrechen".
    await lostEditsWarning(page)
      .getByRole("button", { name: "Abbrechen" })
      .click();
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
    await saveSeriesForm(page);

    await expect(
      page.getByText("Einzelanpassungen gehen verloren"),
    ).toBeVisible();
    await expect(page.getByText("11.05.2026")).toBeVisible();

    await lostEditsWarning(page)
      .getByRole("button", { name: "Abbrechen" })
      .click();
    await expect(
      page.getByText("Einzelanpassungen gehen verloren"),
    ).toBeHidden();
  });
});
