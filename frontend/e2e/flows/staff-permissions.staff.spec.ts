import { test, expect } from "../fixtures";
import { STAFF } from "../helpers/seed-data";

/**
 * Staff-role coverage. The `chromium-staff` Playwright project wires this
 * file (matched by `*.staff.spec.ts`) to the storageState produced for
 * Julia Klein (demo11@mail.de, role=user). Two assertions:
 *
 *   1. The staff session is loaded — Julia's display name is in the
 *      header. This validates the auth.setup → storageState pipeline
 *      end-to-end for the staff role, not just the admin role.
 *   2. Staff users hit the admin-only RoleGuard on /database/* — the
 *      explicit message comes from `app/[tenant]/(protected)/database/
 *      layout.tsx`. This is the authorization boundary that prevents
 *      non-admin staff from seeing student PII through the admin views.
 *
 * Both are real product invariants. Without this file, the staff project
 * would only run setup and never assert anything.
 */

test.describe("Staff session + permissions", () => {
  test("staff lands on the dashboard with their session restored", async ({
    authenticatedPage: page,
  }) => {
    await page.goto("/");
    // The header profile trigger renders the display name on desktop
    // breakpoints. If storageState was loaded correctly, the staff is
    // already signed in and the trigger is visible without any login UI.
    await expect(
      page.getByRole("button").filter({ hasText: STAFF.displayName }).first(),
    ).toBeVisible({ timeout: 15000 });

    // The login form must NOT be visible — proves we did not get bounced
    // back to /login because of an expired or missing session cookie.
    await expect(page.locator('input[name="email"]')).toHaveCount(0);
  });

  test("staff hits the admin-only RoleGuard on /database/students", async ({
    authenticatedPage: page,
  }) => {
    await page.goto("/database/students");
    // Exact copy from `app/[tenant]/(protected)/database/layout.tsx`. If
    // someone changes the message, update both places — the wording is
    // user-facing German UI and intentionally part of the contract.
    await expect(
      page.getByText(
        "Du verfügst nicht über die notwendigen Berechtigungen, um die Datenverwaltung aufzurufen.",
      ),
    ).toBeVisible({ timeout: 10000 });
  });
});
