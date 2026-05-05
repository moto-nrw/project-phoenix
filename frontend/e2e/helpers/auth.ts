import type { Page } from "@playwright/test";

/**
 * Drive the tenant login form. Used only by the auth setup project — every
 * other test inherits a pre-authenticated `storageState` and never sees the
 * login UI.
 */
export async function loginViaUI(
  page: Page,
  credentials: { email: string; password: string },
): Promise<void> {
  await page.goto("/");

  await page.waitForSelector('input[name="email"]', { timeout: 15000 });

  await page.fill('input[name="email"]', credentials.email);
  await page.fill('input[name="password"]', credentials.password);

  await page.click('button:has-text("Anmelden")');

  // After successful login, NextAuth sets the session cookie and the client
  // navigates away from the login page. The exact target depends on role
  // (admins land on /, staff on /myroom, etc.), so we just assert "no longer
  // on the login form".
  await page.waitForURL((url) => !/\/login(\/|$)/.test(url.pathname), {
    timeout: 15000,
  });
  await page.waitForSelector('input[name="email"]', {
    state: "detached",
    timeout: 15000,
  });
}
