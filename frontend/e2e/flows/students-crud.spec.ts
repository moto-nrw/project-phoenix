import { test as uiTest, expect as uiExpect } from "../fixtures";
import { test as apiTest, expect as apiExpect } from "@playwright/test";
import { withAdminContext } from "../helpers/admin-api";
import { BACKEND_URL } from "../helpers/iot";

/**
 * Student CRUD coverage. The full create → read → update → delete cycle is
 * driven via HTTP because the create form has many optional fields and is
 * better-targeted with focused integration tests; we'd rather catch payload
 * regressions here than in a flaky form-walking test. The companion UI test
 * verifies that seeded students show up on the admin's /students list.
 */

apiTest.describe("Student CRUD via HTTP API", () => {
  // Use a unique-ish suffix so re-running without `migrate reset` doesn't
  // collide with leftover rows. The student ID is server-assigned, so we
  // capture it from the create response.
  const suffix = `${Date.now()}`;
  const firstName = `E2E${suffix}`;

  apiTest("create → list → get → update → delete cycle", async () => {
    await withAdminContext(async (ctx, headers) => {
      // === CREATE ===
      const createRes = await ctx.post(`${BACKEND_URL}/api/students`, {
        headers,
        data: {
          first_name: firstName,
          last_name: "Probe",
          school_class: "1a",
        },
      });
      // Backend returns 201 Created on POST; accept the 2xx range to stay
      // resilient if we ever standardise on 200.
      apiExpect(
        createRes.status(),
        `create body: ${await createRes.text()}`,
      ).toBeGreaterThanOrEqual(200);
      apiExpect(createRes.status()).toBeLessThan(300);
      const created = (await createRes.json()) as {
        data: { id: number; first_name: string; last_name: string };
      };
      const studentId = created.data.id;
      apiExpect(studentId).toBeGreaterThan(0);
      apiExpect(created.data.first_name).toBe(firstName);

      try {
        // === LIST (filtered by search query) ===
        const listRes = await ctx.get(
          `${BACKEND_URL}/api/students?search=${encodeURIComponent(firstName)}`,
          { headers },
        );
        apiExpect(listRes.status()).toBe(200);
        const list = (await listRes.json()) as {
          data: Array<{ id: number; first_name: string }>;
        };
        apiExpect(list.data.find((s) => s.id === studentId)).toBeDefined();

        // === GET BY ID ===
        const getRes = await ctx.get(
          `${BACKEND_URL}/api/students/${studentId}`,
          { headers },
        );
        apiExpect(getRes.status()).toBe(200);
        const fetched = (await getRes.json()) as {
          data: { id: number; first_name: string; school_class: string };
        };
        apiExpect(fetched.data.id).toBe(studentId);
        apiExpect(fetched.data.school_class).toBe("1a");

        // === UPDATE ===
        const updateRes = await ctx.put(
          `${BACKEND_URL}/api/students/${studentId}`,
          {
            headers,
            data: {
              first_name: firstName,
              last_name: "Updated",
              school_class: "2b",
            },
          },
        );
        apiExpect(updateRes.status()).toBe(200);
        const updated = (await updateRes.json()) as {
          data: { last_name: string; school_class: string };
        };
        apiExpect(updated.data.last_name).toBe("Updated");
        apiExpect(updated.data.school_class).toBe("2b");
      } finally {
        // === DELETE (always — keep DB tidy if assertions failed) ===
        const delRes = await ctx.delete(
          `${BACKEND_URL}/api/students/${studentId}`,
          { headers },
        );
        apiExpect(delRes.status()).toBe(200);

        // GET after delete must 404
        const afterDel = await ctx.get(
          `${BACKEND_URL}/api/students/${studentId}`,
          { headers, failOnStatusCode: false },
        );
        apiExpect(afterDel.status()).toBe(404);
      }
    });
  });
});

uiTest.describe("Student list UI", () => {
  uiTest(
    "admin sees seeded students on /database/students and can filter by search",
    async ({ authenticatedPage: page }) => {
      // The Kinder admin view lives under /database/students; bare /students
      // is reserved for the staff search/check-in flow.
      await page.goto("/database/students");

      // The first seeded student is Felix Schneider (DemoStudents[0]). Wait
      // for him to appear — accommodates SWR/SSR loading.
      await uiExpect(page.getByText("Felix Schneider").first()).toBeVisible({
        timeout: 15000,
      });

      // Sanity check: another seeded student should also be visible. If both
      // appear, we know the list is rendering, not just one stub.
      await uiExpect(page.getByText("Emma Meyer").first()).toBeVisible({
        timeout: 5000,
      });

      // PageHeaderWithSearch renders two inputs (mobile + desktop) with the
      // same placeholder; only one is visible at a given breakpoint, so
      // filter to that one rather than relying on document order.
      const searchBox = page
        .getByPlaceholder("Schüler suchen...")
        .locator("visible=true");
      await searchBox.fill("Felix");

      // After filtering, Felix is still there but Emma should be gone.
      await uiExpect(page.getByText("Felix Schneider").first()).toBeVisible({
        timeout: 5000,
      });
      await uiExpect(page.getByText("Emma Meyer")).toHaveCount(0, {
        timeout: 5000,
      });
    },
  );
});
