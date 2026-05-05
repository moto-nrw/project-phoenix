import { test, expect } from "../fixtures";
import { ADMIN } from "../helpers/seed-data";

test.describe("Login", () => {
  // These tests must start unauthenticated, so override the project's
  // storageState with an empty one. Without this, the chromium-admin project
  // would already have a session cookie and the login form would not render.
  test.use({ storageState: { cookies: [], origins: [] } });

  test("rejects wrong password and stays on the login form", async ({
    page,
  }) => {
    await page.goto("/");
    await page.waitForSelector('input[name="email"]');

    await page.fill('input[name="email"]', ADMIN.email);
    await page.fill(
      'input[name="password"]',
      "definitely-not-the-real-password",
    );
    await page.click('button:has-text("Anmelden")');

    // Wait for the login request to settle. Under parallel load this can
    // take a moment, and the inline error is only rendered after the
    // submit handler returns. Without this guard the error assertion
    // races the in-flight request.
    await expect(page.getByText("Anmeldung läuft...")).toHaveCount(0, {
      timeout: 20000,
    });

    // Inline error rendered for invalid credentials (src/app/[tenant]/page.tsx:165).
    // The catch-block "Anmeldefehler. Bitte versuchen Sie es erneut." is for
    // network/transport failures, not auth rejections.
    await expect(page.getByText("Ungültige E-Mail oder Passwort")).toBeVisible({
      timeout: 10000,
    });

    // No navigation occurred — login form is still visible
    await expect(page.locator('input[name="email"]')).toBeVisible();
  });
});

test.describe("Logout", () => {
  test("user can sign out via the profile dropdown", async ({
    authenticatedPage: page,
  }) => {
    await page.goto("/");

    // Find the profile trigger button. Selectors:
    //   - It is a <button> in the header that contains the user's display
    //     name on desktop breakpoints.
    //   - Two characters of the name (e.g. "AM") also live in the avatar,
    //     but the desktop trigger renders the full name in a sibling div.
    const profileTrigger = page
      .getByRole("button")
      .filter({ hasText: ADMIN.displayName });
    await profileTrigger.first().waitFor({ state: "visible", timeout: 15000 });
    await profileTrigger.first().click();

    // The dropdown's "Abmelden" entry lives inside the page banner and
    // opens the confirmation modal. The dropdown stays mounted briefly
    // while the modal animates in, so scope each click to its container.
    await page
      .getByRole("banner")
      .getByRole("button", { name: "Abmelden" })
      .click();

    // Confirm in modal — the modal copy is unique, so wait for it first.
    await expect(
      page.getByText("Möchten Sie sich wirklich von Ihrem Konto abmelden?"),
    ).toBeVisible({ timeout: 5000 });
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Abmelden" })
      .click();

    // After signOut, NextAuth redirects to / which on a tenant subdomain
    // re-renders the login form. Wait for the form to appear AND for the
    // banner profile trigger to be gone — checking just the input would
    // race a transitional render where both the banner (still showing the
    // name) and the login form are momentarily mounted.
    await page.waitForSelector('input[name="email"]', { timeout: 15000 });
    await expect(page.locator('input[name="email"]')).toBeVisible();
    await expect(
      page.getByRole("banner").getByRole("button", { name: ADMIN.displayName }),
    ).toHaveCount(0, { timeout: 10000 });
  });
});

// Tenant switch is covered in flows/tenant-switch.spec.ts (requires the
// multi-tenant seed setup; gracefully skips when only one tenant exists).
