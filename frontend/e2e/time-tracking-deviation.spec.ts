import { expect, type Page, test } from "@playwright/test";
import { todayISO } from "../src/lib/date-helpers";

// F9 (Planung-Redesign Inkrement 6A): stamping outside the tolerance window
// around the planned shift window requires a reason that lands in the
// session's audit log. The flow under test mirrors docs/08-zeiterfassung.md
// section 13: enable the setting, plan a shift, check out too late, fill the
// reason dialog, verify the audit entry; then disable the setting and verify
// the same stamp passes without a dialog.

// Test credentials from environment variables (never commit real credentials!).
// No fallback defaults per the repo no-fallback policy — the suite skips when
// either is unset (see beforeEach) rather than substituting an empty string.
const TEST_EMAIL = process.env.E2E_TEST_EMAIL;
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD;

const REQUIRE_KEY = "operations.time_tracking_require_deviation_reason";
const TOLERANCE_KEY = "operations.time_tracking_deviation_tolerance_minutes";

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

async function setSetting(page: Page, key: string, value: unknown) {
  const response = await page.request.put(`/api/settings/values/${key}`, {
    data: { value },
  });
  expect(response.ok(), `setting ${key} should update`).toBeTruthy();
}

async function resetSetting(page: Page, key: string) {
  // DELETE removes the tenant override so later runs (and the school) fall
  // back to the registry default.
  await page.request.delete(`/api/settings/values/${key}`);
}

// Clicks "Einstempeln" and confirms the optional "Trotzdem einstempeln"
// follow-up (shown when today's session was manually edited or an absence
// exists) so the stamp actually happens.
async function checkIn(page: Page) {
  await page.getByText("In der OGS").first().click();
  await page.getByLabel("Einstempeln").click();
  const confirm = page.getByRole("button", { name: "Trotzdem einstempeln" });
  try {
    await confirm.waitFor({ state: "visible", timeout: 2000 });
    await confirm.click();
  } catch {
    // No confirmation modal — the stamp went through directly.
  }
  await expect(page.getByLabel("Ausstempeln")).toBeVisible({ timeout: 10000 });
}

test.describe("Time tracking deviation reason (F9)", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(
      !TEST_EMAIL || !TEST_PASSWORD,
      "E2E_TEST_EMAIL and E2E_TEST_PASSWORD must be set",
    );
    if (!TEST_EMAIL || !TEST_PASSWORD) return;
    await login(page, TEST_EMAIL, TEST_PASSWORD);
  });

  test("late check-out demands a reason that lands in the audit log; disabled setting stays silent", async ({
    page,
  }) => {
    // The planned shift ends at 00:02; a check-out before ~00:03 would not
    // deviate yet. Skip the sub-3-minute window right after midnight instead
    // of producing a false red.
    const berlinNow = new Date(
      new Date().toLocaleString("en-US", { timeZone: "Europe/Berlin" }),
    );
    test.skip(
      berlinNow.getHours() === 0 && berlinNow.getMinutes() < 5,
      "planned shift end 00:02 is not in the past yet",
    );

    const reason = `Playwright F9 Begründung ${Date.now()}`;
    let createdShiftId: string | null = null;

    try {
      // Baseline: gate off, so setup stamps never trigger the dialog.
      await setSetting(page, REQUIRE_KEY, false);

      await page.goto("/time-tracking");
      // Wait until the stamp card resolved the current session before
      // branching, otherwise a slow session fetch reads as "not checked in".
      await expect(
        page.getByLabel("Einstempeln").or(page.getByLabel("Ausstempeln")),
      ).toBeVisible({ timeout: 15000 });
      // A leftover active session from a previous run is stamped out first
      // (setting is off, so this cannot open the dialog).
      if (await page.getByLabel("Ausstempeln").isVisible()) {
        await page.getByLabel("Ausstempeln").click();
        await expect(page.getByLabel("Einstempeln")).toBeVisible({
          timeout: 10000,
        });
      }

      // Check in without a planned shift (or long after its start — late
      // arrivals are not gated), then read the session for staff id and
      // audit lookups.
      await checkIn(page);
      const currentResponse = await page.request.get(
        "/api/time-tracking/current",
      );
      expect(currentResponse.ok()).toBeTruthy();
      const current = (await currentResponse.json()) as {
        data: { id: number; staff_id: number };
      };
      const sessionId = current.data.id;
      const staffId = current.data.staff_id;

      // Plan a shift that ended just after midnight, so "now" is past the
      // planned end. A 409 means a suitable shift is left over from an
      // earlier run — equally fine.
      const shiftResponse = await page.request.post("/api/staff/shifts", {
        data: {
          staff_id: staffId,
          date: todayISO(),
          start_time: "00:01",
          end_time: "00:02",
          break_minutes: 0,
          shift_type_id: null,
        },
      });
      expect(
        shiftResponse.ok() || shiftResponse.status() === 409,
        "shift creation should succeed or already exist",
      ).toBeTruthy();
      if (shiftResponse.ok()) {
        const shift = (await shiftResponse.json()) as {
          data: { id: number };
        };
        createdShiftId = String(shift.data.id);
      }

      // Arm the gate with zero tolerance.
      await setSetting(page, REQUIRE_KEY, true);
      await setSetting(page, TOLERANCE_KEY, 0);

      // Late check-out → reason dialog with the deviation facts.
      await page.getByLabel("Ausstempeln").click();
      await expect(page.getByText("Abweichung vom Dienstplan")).toBeVisible();
      await expect(
        page.getByText(/nach deinem geplanten Dienstende/),
      ).toBeVisible();
      const confirmButton = page.getByRole("button", {
        name: "Mit Begründung ausstempeln",
      });
      await expect(confirmButton).toBeDisabled();

      await page.getByLabel("Grund").fill(reason);
      await confirmButton.click();
      await expect(page.getByText("Abweichung vom Dienstplan")).toBeHidden({
        timeout: 10000,
      });
      await expect(page.getByLabel("Einstempeln")).toBeVisible({
        timeout: 10000,
      });

      // The reason is readable in the session's audit log.
      const editsResponse = await page.request.get(
        `/api/time-tracking/${sessionId}/edits`,
      );
      expect(editsResponse.ok()).toBeTruthy();
      const edits = (await editsResponse.json()) as {
        data: Array<{ field_name: string; notes: string | null }>;
      };
      const deviationEdit = edits.data.find(
        (edit) => edit.field_name === "deviation_reason",
      );
      expect(deviationEdit, "deviation_reason audit entry").toBeTruthy();
      expect(deviationEdit?.notes).toBe(reason);

      // Second case: with the setting off, the same stamp passes silently.
      await setSetting(page, REQUIRE_KEY, false);
      await checkIn(page); // reopens today's session
      await page.getByLabel("Ausstempeln").click();
      await expect(page.getByLabel("Einstempeln")).toBeVisible({
        timeout: 10000,
      });
      await expect(page.getByText("Abweichung vom Dienstplan")).toBeHidden();
    } finally {
      // Leave the tenant in its default state for reruns and other suites.
      if (createdShiftId) {
        await page.request.delete(`/api/staff/shifts/${createdShiftId}`);
      }
      await resetSetting(page, REQUIRE_KEY);
      await resetSetting(page, TOLERANCE_KEY);
    }
  });
});
