import { test, expect, assertSessionReady } from "../fixtures";
import * as routes from "../helpers/routes";

test.describe("Tenant switch", () => {
  test("admin with access to two tenants switches without re-login", async ({
    authenticatedPage: page,
    app,
    authSessions,
    tenantSwitchScenario,
  }) => {
    const { primaryTenant, secondaryTenant, actorDisplayName } =
      tenantSwitchScenario;

    await page.goto(app.primary(routes.root));

    // Wait until the post-login redirect chain has settled on the actual
    // dashboard before looking for the switcher. Asserting on the
    // switcher straight after loading the tenant root races the
    // "Einrichtung wechseln"
    // interstitial under default parallel load — the page is still on the
    // redirect/loading flow and the protected shell hasn't mounted yet.
    await page.waitForURL(/\/dashboard(\/|$)/, { timeout: 20000 });
    await expect(
      page.getByRole("button", { name: primaryTenant.name }).first(),
    ).toBeVisible({ timeout: 20000 });
    await expect(
      page.getByRole("button").filter({ hasText: actorDisplayName }).first(),
    ).toBeVisible({ timeout: 10000 });

    // Open the dropdown and pick the second tenant.
    await page
      .getByRole("button", { name: primaryTenant.name })
      .first()
      .click();
    await page.getByRole("button", { name: secondaryTenant.name }).click();

    // The switch flow does a hard navigation to the second subdomain.
    await expect
      .poll(() => {
        const url = new URL(page.url());
        return `${url.origin}${url.pathname}`;
      })
      .toBe(`${app.origin(secondaryTenant)}/dashboard`);
    await expect(page.locator('input[name="email"]')).toHaveCount(0);
    await assertSessionReady(page, authSessions.admin);
  });
});
