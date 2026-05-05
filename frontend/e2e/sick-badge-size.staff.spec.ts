import { test, expect } from "./fixtures";

test.describe("Sick badge size consistency", () => {
  test("Krank badge should be same size as location badge when student is sick and present", async ({
    authenticatedPage: page,
  }) => {
    await page.goto("/ogs-groups");

    // Wait for student cards to appear (don't use networkidle — SSE keeps it busy)
    await page
      .waitForSelector("[data-location-status]", { timeout: 10000 })
      .catch(() => {
        // If no location badges found, just continue
      });

    await page.screenshot({
      path: "e2e/screenshots/ogs-groups-page.png",
      fullPage: true,
    });

    const sickBadge = page.locator('[data-sick-indicator="true"]').first();
    const sickCount = await sickBadge.count();

    const locationBadge = page.locator("[data-location-status]").first();
    const locationCount = await locationBadge.count();

    if (sickCount === 0 || locationCount === 0) {
      test.skip(
        true,
        "No sick student currently present in OGS — skipping size comparison",
      );
      return;
    }

    const sickIndicators = page.locator('[data-sick-indicator="true"]');
    const allSickBadges = await sickIndicators.all();

    for (const sickBadgeEl of allSickBadges) {
      const parentContainer = sickBadgeEl
        .locator("xpath=ancestor::div[contains(@class, 'flex-col')]")
        .first();
      const siblingLocationBadge = parentContainer
        .locator("[data-location-status]")
        .first();

      if ((await siblingLocationBadge.count()) > 0) {
        const locationBox = await siblingLocationBadge.boundingBox();
        const sickBox = await sickBadgeEl.boundingBox();

        expect(locationBox).not.toBeNull();
        expect(sickBox).not.toBeNull();

        if (locationBox && sickBox) {
          expect(Math.abs(locationBox.height - sickBox.height)).toBeLessThan(3);

          await parentContainer.screenshot({
            path: "e2e/screenshots/sick-badge-comparison.png",
          });

          return;
        }
      }
    }

    test.skip(
      true,
      "Found sick badges and location badges but none in the same card — skipping size comparison",
    );
  });
});
