import { test, expect } from "../fixtures";
import { TENANT_NAME, E2E_FRONTEND_PORT } from "../helpers/seed-data";
import { getSecondTenant } from "../helpers/seed-state";

test.describe("Tenant switch", () => {
  test("admin with access to two tenants switches without re-login", async ({
    authenticatedPage: page,
  }) => {
    // Slug + name come from .seed-state-e2e.json (written by the Go
    // seeder's --with-second-tenant step) — never hardcoded here, so a
    // future rename only touches the seeder defaults.
    const second = getSecondTenant();

    await page.goto("/");

    // Wait until the post-login redirect chain has settled on the actual
    // dashboard before looking for the switcher. Asserting on the
    // switcher straight after `goto("/")` races the "Einrichtung wechseln"
    // interstitial under default parallel load — the page is still on the
    // redirect/loading flow and the protected shell hasn't mounted yet.
    await page.waitForURL(/\/dashboard(\/|$)/, { timeout: 20000 });

    // The TenantSwitcher only renders when the user has access to more than
    // one tenant. `scripts/seed-e2e.sh` always invokes the seeder with
    // --with-second-tenant for exactly this reason, so the dropdown MUST
    // be present here. If it isn't, the seed is broken — fail loudly
    // rather than skip silently.
    const switcherTrigger = page.getByRole("button", { name: TENANT_NAME });
    await expect(switcherTrigger.first()).toBeVisible({ timeout: 20000 });

    // Open the dropdown and pick the second tenant.
    await switcherTrigger.first().click();
    await page.getByRole("button", { name: second.name }).click();

    // The switch flow does a hard navigation to the second subdomain.
    await page.waitForURL(
      new RegExp(
        `^http://${second.slug}\\.localtest\\.me:${E2E_FRONTEND_PORT}`,
      ),
      { timeout: 15000 },
    );

    // The whole point of using localtest.me instead of *.localhost: the
    // session cookie is scoped to .localtest.me and is sent to the new
    // subdomain, so we land authenticated. If cookies were host-only the
    // login form would be visible here.
    await expect(page.locator('input[name="email"]')).toHaveCount(0);

    // The switcher trigger now reflects the new current tenant.
    await expect(
      page.getByRole("button", { name: second.name }).first(),
    ).toBeVisible({ timeout: 10000 });
  });
});
