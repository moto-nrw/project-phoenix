import { test, expect } from "./fixtures";
import * as routes from "./helpers/routes";

test.describe("Sick badge size consistency", () => {
  test("Krank badge should be same size as location badge when student is sick and present", async ({
    authenticatedPage: page,
    app,
    adminApi,
    presentReadyStudent,
  }) => {
    const student = presentReadyStudent;
    const fullName = `${student.first_name} ${student.last_name}`;

    try {
      const markSickRes = await adminApi.put(`/api/students/${student.id}`, {
        data: { sick: true, excused: false },
      });
      expect(markSickRes.status(), await markSickRes.text()).toBe(200);

      await page.goto(app.primary(routes.ogsGroups));
      await page.waitForSelector("[data-location-status]", { timeout: 15000 });

      const studentCard = page.getByRole("button", {
        name: new RegExp(fullName),
      });
      await expect(studentCard).toBeVisible({ timeout: 15000 });

      const locationBadge = studentCard
        .locator("[data-location-status]")
        .first();
      const sickBadge = studentCard
        .locator('[data-sick-indicator="true"]')
        .first();

      await expect(locationBadge).toBeVisible({ timeout: 10000 });
      await expect(sickBadge).toBeVisible({ timeout: 10000 });

      const locationBox = await locationBadge.boundingBox();
      const sickBox = await sickBadge.boundingBox();

      expect(locationBox).not.toBeNull();
      expect(sickBox).not.toBeNull();

      if (!locationBox || !sickBox) {
        throw new Error("badge bounding boxes were unexpectedly null");
      }

      expect(Math.abs(locationBox.height - sickBox.height)).toBeLessThan(3);
    } finally {
      const clearSickRes = await adminApi.put(`/api/students/${student.id}`, {
        data: { sick: false },
      });
      expect(clearSickRes.status(), await clearSickRes.text()).toBe(200);
    }
  });
});
