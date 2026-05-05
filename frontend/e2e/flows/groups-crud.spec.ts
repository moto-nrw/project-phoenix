// NOSONAR S2187 — SonarCloud's Jasmine detector does not recognise
// Playwright's `test(...)` blocks as test cases and false-flags the file
// as empty. This spec contains real Playwright tests (uiTest / apiTest).
import {
  test as uiTest,
  expect as uiExpect,
  apiTest,
  apiExpect,
} from "../fixtures";
import { withAdminContext } from "../helpers/admin-api";
import { uniqueSuffix } from "../helpers/factories";
import { BACKEND_URL } from "../helpers/iot";
import * as routes from "../helpers/routes";
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
      await page.goto(routes.groupsList);

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

  uiTest(
    "admin creates a group via the GroupCreateModal and the row appears in the list",
    async ({ authenticatedPage: page, groupFactory }) => {
      // Issue #1142 lists "Erstellen" as a Gruppen task. The HTTP path
      // is covered by the API spec above; this UI test exercises the
      // open-modal → fill-form → submit → row-visible chain.
      const name = `UICreateGruppe ${uniqueSuffix()}`;

      await page.goto(routes.groupsList);

      // The plus-button has aria-label "Gruppe erstellen"
      // (DatabaseCreateAction). Stable accessible name despite the
      // icon-only visual.
      await page
        .getByRole("button", { name: "Gruppe erstellen" })
        .first()
        .click();

      const dialog = page.getByRole("dialog");
      await uiExpect(dialog).toBeVisible({ timeout: 10000 });
      await uiExpect(
        dialog.getByRole("heading", { name: "Neue Gruppe" }),
      ).toBeVisible();

      // The DatabaseForm renders inputs with `name` attributes from the
      // groups.config field definitions — `name="name"` is the
      // Gruppenname field. Locating by name attribute is more stable
      // than placeholder text, which is brand copy ("z.B. Gruppe Blau")
      // that marketing might tweak.
      await dialog.locator('input[name="name"]').fill(name);

      await dialog.getByRole("button", { name: "Erstellen" }).click();

      // The page closes the modal in its onCreate success handler
      // (page.tsx:164). Modal hidden = create flow completed.
      await uiExpect(dialog).toBeHidden({ timeout: 15000 });

      // Row visible in the list. The list is SWR-backed; SWR mutate is
      // called after create, so the new row appears without a manual
      // reload.
      await uiExpect(page.getByText(name).first()).toBeVisible({
        timeout: 10000,
      });

      // Round-trip via /api/groups to confirm DB persistence and
      // capture the id for cleanup. The list endpoint is unfiltered
      // (no ?search=), so we walk the small result set ourselves —
      // groups don't have a search-by-name handler at the time of
      // writing, and adding one for tests would be tail-wagging-dog.
      const id = await withAdminContext(async (ctx, headers) => {
        const res = await ctx.get(`${BACKEND_URL}/api/groups`, { headers });
        uiExpect(res.status()).toBe(200);
        const body = (await res.json()) as {
          data: Array<{ id: number; name: string }>;
        };
        const match = body.data.find((g) => g.name === name);
        uiExpect(
          match,
          `group "${name}" not found via /api/groups`,
        ).toBeDefined();
        return match!.id;
      });

      // Hand the id to the factory so its teardown deletes it.
      groupFactory.track(id);
    },
  );
});
