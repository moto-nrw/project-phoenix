import { test, expect } from "../fixtures";
import {
  TENANT_NAME,
  SECOND_TENANT_NAME,
  SECOND_TENANT_SLUG,
} from "../helpers/seed-data";

test.describe("Tenant switch", () => {
  test("admin with access to two tenants switches without re-login", async ({
    authenticatedPage: page,
  }) => {
    await page.goto("/");

    // The TenantSwitcher only renders when the user has access to more than
    // one tenant. `scripts/seed-e2e.sh` chains in seed-e2e-multi-tenant.sh
    // for exactly this reason, so the dropdown MUST be present here. If it
    // isn't, the seed is broken — fail loudly rather than skip silently.
    const switcherTrigger = page.getByRole("button", { name: TENANT_NAME });
    await expect(switcherTrigger.first()).toBeVisible({ timeout: 10000 });

    // Open the dropdown and pick the second tenant.
    await switcherTrigger.first().click();
    await page.getByRole("button", { name: SECOND_TENANT_NAME }).click();

    // The switch flow does a hard navigation to the second subdomain.
    await page.waitForURL(
      new RegExp(`^http://${SECOND_TENANT_SLUG}\\.localtest\\.me:3000`),
      { timeout: 15000 },
    );

    // The whole point of using localtest.me instead of *.localhost: the
    // session cookie is scoped to .localtest.me and is sent to the new
    // subdomain, so we land authenticated. If cookies were host-only the
    // login form would be visible here.
    await expect(page.locator('input[name="email"]')).toHaveCount(0);

    // The switcher trigger now reflects the new current tenant.
    await expect(
      page.getByRole("button", { name: SECOND_TENANT_NAME }).first(),
    ).toBeVisible({ timeout: 10000 });
  });
});
