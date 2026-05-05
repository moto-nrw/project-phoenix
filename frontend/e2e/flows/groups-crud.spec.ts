import {
  test as uiTest,
  expect as uiExpect,
  apiTest,
  apiExpect,
} from "../fixtures";
import { withAdminContext } from "../helpers/admin-api";
import { BACKEND_URL } from "../helpers/iot";
import { getFirstTwoGroupDisplayNames } from "../helpers/seed-state";

/**
 * Group CRUD coverage. We test the API contract end-to-end (create group →
 * assign student → query members → delete) via HTTP because the create-
 * group form is buried in modals and the API surface is the more durable
 * test target. The UI half asserts that seeded groups render on the
 * /database/groups admin page.
 *
 * Cleanup is mandatory: the backend rejects DELETE on a group that still
 * has students (HTTP 409), so we delete the helper student first before
 * removing the group. Wrapped in `try/finally` so leftover state can't
 * accumulate even when assertions fail.
 */

apiTest.describe("Group CRUD via HTTP API", () => {
  apiTest(
    "create group, assign student, list members, then delete in correct order",
    async () => {
      const suffix = `${Date.now()}`;

      await withAdminContext(async (ctx, headers) => {
        // === Create a fresh student we can move around — never touch
        // seeded students because they may be referenced by other tests.
        const studentRes = await ctx.post(`${BACKEND_URL}/api/students`, {
          headers,
          data: {
            first_name: `GroupProbe${suffix}`,
            last_name: "Student",
            school_class: "1a",
          },
        });
        apiExpect(studentRes.status()).toBeLessThan(300);
        const studentBody = (await studentRes.json()) as {
          data: { id: number };
        };
        const studentId = studentBody.data.id;

        // === Create the group itself
        const groupRes = await ctx.post(`${BACKEND_URL}/api/groups`, {
          headers,
          data: { name: `E2E Probe Group ${suffix}` },
        });
        apiExpect(
          groupRes.status(),
          `create group body: ${await groupRes.text()}`,
        ).toBeLessThan(300);
        const groupBody = (await groupRes.json()) as {
          data: { id: number; name: string };
        };
        const groupId = groupBody.data.id;
        apiExpect(groupId).toBeGreaterThan(0);

        try {
          // === The new group should show up in the listing.
          const listRes = await ctx.get(`${BACKEND_URL}/api/groups`, {
            headers,
          });
          apiExpect(listRes.status()).toBe(200);
          const listBody = (await listRes.json()) as {
            data: Array<{ id: number; name: string }>;
          };
          apiExpect(listBody.data.find((g) => g.id === groupId)).toBeDefined();

          // === Assign the student to the group via the student PUT path —
          // there's no dedicated "/groups/{id}/students" write endpoint;
          // membership is owned by the student record.
          const assignRes = await ctx.put(
            `${BACKEND_URL}/api/students/${studentId}`,
            {
              headers,
              data: {
                first_name: `GroupProbe${suffix}`,
                last_name: "Student",
                school_class: "1a",
                group_id: groupId,
              },
            },
          );
          apiExpect(assignRes.status()).toBe(200);
          const assignBody = (await assignRes.json()) as {
            data: { group_id: number | null };
          };
          apiExpect(assignBody.data.group_id).toBe(groupId);

          // === The group's student roster now includes our test student.
          const membersRes = await ctx.get(
            `${BACKEND_URL}/api/groups/${groupId}/students`,
            { headers },
          );
          apiExpect(membersRes.status()).toBe(200);
          const membersBody = (await membersRes.json()) as {
            data: Array<{ id: number }>;
          };
          apiExpect(
            membersBody.data.find((s) => s.id === studentId),
          ).toBeDefined();
        } finally {
          // Order matters: the group's DELETE returns 409 while the
          // student is still attached, so remove the student first.
          await ctx.delete(`${BACKEND_URL}/api/students/${studentId}`, {
            headers,
            failOnStatusCode: false,
          });
          const groupDel = await ctx.delete(
            `${BACKEND_URL}/api/groups/${groupId}`,
            { headers },
          );
          apiExpect(
            groupDel.status(),
            `group delete body: ${await groupDel.text()}`,
          ).toBe(200);
        }
      });
    },
  );
});

uiTest.describe("Group list UI", () => {
  uiTest(
    "admin sees seeded groups on /database/groups",
    async ({ authenticatedPage: page }) => {
      await page.goto("/database/groups");

      // Two seeded group display names pulled from the seeder's lookup
      // table — no hardcoded "Sternengruppe"/"Bärengruppe" so the test
      // survives a seeder reorder. If both render, the list is wired up,
      // not just one stub.
      const [firstGroup, secondGroup] = getFirstTwoGroupDisplayNames();
      await uiExpect(page.getByText(firstGroup).first()).toBeVisible({
        timeout: 15000,
      });
      await uiExpect(page.getByText(secondGroup).first()).toBeVisible({
        timeout: 5000,
      });
    },
  );
});
