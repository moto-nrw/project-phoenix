import { test, expect } from "@playwright/test";

// Test credentials from environment variables (never commit real credentials!)
const TEST_EMAIL = process.env.E2E_TEST_EMAIL ?? "";
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD ?? "";

test.describe("Sick badge size consistency", () => {
  test.beforeEach(async ({ page }) => {
    // Skip if no credentials configured
    test.skip(
      !TEST_EMAIL || !TEST_PASSWORD,
      "E2E_TEST_EMAIL and E2E_TEST_PASSWORD must be set",
    );

    await page.goto("/");

    // Wait for login form
    await page.waitForSelector('input[type="email"]');

    // Fill login form (German labels)
    await page.fill('input[type="email"]', TEST_EMAIL);
    await page.fill('input[type="password"]', TEST_PASSWORD);

    // Click Anmelden button
    await page.click('button:has-text("Anmelden")');

    // Wait for navigation away from login page
    await page.waitForURL(
      (url) => !url.pathname.includes("login") && url.pathname !== "/",
      {
        timeout: 15000,
      },
    );
  });

  test("Krank badge should be same size as location badge when student is sick and present", async ({
    page,
  }) => {
    // Navigate to OGS groups page where location badges appear
    await page.goto("/ogs-groups");

    // Wait for student cards to appear (don't use networkidle - SSE keeps it busy)
    await page
      .waitForSelector("[data-location-status]", { timeout: 10000 })
      .catch(() => {
        // If no location badges found, just continue
      });

    // Take initial screenshot to see page state
    await page.screenshot({
      path: "e2e/screenshots/ogs-groups-page.png",
      fullPage: true,
    });

    // Look for a sick indicator badge
    const sickBadge = page.locator('[data-sick-indicator="true"]').first();
    const sickCount = await sickBadge.count();

    // Find main location badge
    const locationBadge = page.locator("[data-location-status]").first();
    const locationCount = await locationBadge.count();

    console.log(
      `Found ${locationCount} location badges, ${sickCount} sick badges`,
    );

    if (sickCount > 0 && locationCount > 0) {
      // Find a card that has BOTH badges (sick student who is present)
      const sickIndicators = page.locator('[data-sick-indicator="true"]');
      const allSickBadges = await sickIndicators.all();

      for (const sickBadgeEl of allSickBadges) {
        // Get parent container that should also have the location badge
        const parentContainer = sickBadgeEl
          .locator("xpath=ancestor::div[contains(@class, 'flex-col')]")
          .first();
        const siblingLocationBadge = parentContainer
          .locator("[data-location-status]")
          .first();

        if ((await siblingLocationBadge.count()) > 0) {
          // Found a container with both badges - compare sizes
          const locationBox = await siblingLocationBadge.boundingBox();
          const sickBox = await sickBadgeEl.boundingBox();

          expect(locationBox).not.toBeNull();
          expect(sickBox).not.toBeNull();

          if (locationBox && sickBox) {
            console.log(
              `Location badge: ${locationBox.width}x${locationBox.height}px, Sick badge: ${sickBox.width}x${sickBox.height}px`,
            );

            // Heights should be similar (within 2px tolerance)
            expect(Math.abs(locationBox.height - sickBox.height)).toBeLessThan(
              3,
            );

            // Take screenshot of the specific card
            await parentContainer.screenshot({
              path: "e2e/screenshots/sick-badge-comparison.png",
            });

            return; // Test passed
          }
        }
      }
    }

    console.log(
      "No sick student currently present in OGS - skipping size comparison",
    );
  });
});
