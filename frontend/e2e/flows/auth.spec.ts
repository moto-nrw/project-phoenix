import {
  test,
  expect,
  assertSessionReady,
  waitForLoginFormReady,
} from "../fixtures";

test.describe("Login", () => {
  // These tests must start unauthenticated, so override the project's
  // storageState with an empty one. Without this, the chromium-admin project
  // would already have a session cookie and the login form would not render.
  test.use({ storageState: { cookies: [], origins: [] } });

  test("admin signs in with valid credentials and lands on the dashboard", async ({
    page,
    authSessions,
  }) => {
    const adminSession = authSessions.admin;

    // The positive login flow is exercised by `auth.setup.ts` for every
    // suite run, but only as a setup step — a regression there fails with
    // the cryptic "setup project failed" rather than a clearly named test.
    // This spec asserts the same path as a first-class test so a broken
    // login surfaces with the right name in the report.
    await page.goto(adminSession.appRoot);
    await waitForLoginFormReady(page);

    await page
      .getByRole("textbox", { name: "E-Mail-Adresse" })
      .fill(adminSession.email);
    await page
      .getByRole("textbox", { name: "Passwort" })
      .fill(adminSession.password);
    await page.getByRole("button", { name: "Anmelden" }).click();

    // After successful login NextAuth redirects away from /login. Wait for
    // the authenticated shell — the admin display-name button — as the
    // single canonical "we are past login" signal. Asserting on URL or
    // email-input absence first races the "Sie werden weitergeleitet…"
    // interstitial under parallel load: the URL has already left /login
    // but the protected shell is not mounted yet, so input[name="email"]
    // can still be present on the interstitial page. The display-name
    // button only renders inside the protected layout, so its visibility
    // is the real gate.
    await assertSessionReady(page, adminSession);
    await expect(page).not.toHaveURL(/\/login(\/|$)/);
  });

  test("rejects wrong password and stays on the login form", async ({
    page,
    authSessions,
  }) => {
    const adminSession = authSessions.admin;
    await page.goto(adminSession.appRoot);
    await waitForLoginFormReady(page);

    await page
      .getByRole("textbox", { name: "E-Mail-Adresse" })
      .fill(adminSession.email);
    await page
      .getByRole("textbox", { name: "Passwort" })
      .fill("definitely-not-the-real-password");
    await page.getByRole("button", { name: "Anmelden" }).click();

    // Wait for the login request to settle. Under parallel load this can
    // take a moment, so assert directly on the inline error instead of the
    // transient loading copy. The loading text can remain mounted briefly in
    // the DOM even after the form has already returned to its idle state.

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
    authSessions,
  }) => {
    const adminSession = authSessions.admin;
    await page.goto(adminSession.appRoot);
    await assertSessionReady(page, adminSession);

    // Find the profile trigger button. Selectors:
    //   - It is a <button> in the header that contains the user's display
    //     name on desktop breakpoints.
    //   - Two characters of the name (e.g. "AM") also live in the avatar,
    //     but the desktop trigger renders the full name in a sibling div.
    const profileTrigger = page
      .getByRole("banner")
      .getByRole("button", {
        name: new RegExp(
          adminSession.displayName.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"),
        ),
      })
      .first();
    await profileTrigger.waitFor({ state: "visible", timeout: 15000 });
    await profileTrigger.click();

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
    await waitForLoginFormReady(page);
    await expect(
      page
        .getByRole("banner")
        .getByRole("button", { name: adminSession.displayName }),
    ).toHaveCount(0, { timeout: 10000 });
  });
});

// Tenant switch is covered in flows/tenant-switch.spec.ts (requires the
// multi-tenant seed setup; gracefully skips when only one tenant exists).
