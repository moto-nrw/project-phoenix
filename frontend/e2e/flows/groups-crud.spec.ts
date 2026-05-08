import {
  test as uiTest,
  expect as uiExpect,
  apiTest,
  apiExpect,
} from "../fixtures";
import { uniqueSuffix } from "../helpers/factories";
import * as routes from "../helpers/routes";

/**
 * Group CRUD coverage. We test the API contract end-to-end (create group →
 * assign student → query members → delete) via HTTP because the create-
 * group form is buried in modals and the API surface is the more durable
 * test target. The UI half asserts that seeded groups render on the
 * /database/groups admin page.
 *
 * Cleanup is mandatory: the backend rejects DELETE on a group that still
 * has students (HTTP 409), so we delete the helper student first before
 * removing the group. The fixtures still track both ids, so teardown
 * cleans up leftovers even when assertions fail before the explicit delete.
 */

apiTest.describe("Group CRUD via HTTP API", () => {
  apiTest(
    "create group, assign student, list members, then delete in correct order",
    async ({ adminApi, studentFactory, groupFactory }) => {
      const student = await studentFactory.create({
        first_name: `GroupProbe${uniqueSuffix()}`,
        last_name: "Student",
        school_class: "1a",
      });
      const group = await groupFactory.create({
        name: `E2E Probe Group ${uniqueSuffix()}`,
      });

      // === The new group should show up in the listing.
      const listRes = await adminApi.get("/api/groups");
      apiExpect(listRes.status()).toBe(200);
      const listBody = (await listRes.json()) as {
        data: Array<{ id: number; name: string }>;
      };
      apiExpect(listBody.data.find((g) => g.id === group.id)).toBeDefined();

      // === Assign the student to the group via the student PUT path —
      // there's no dedicated "/groups/{id}/students" write endpoint;
      // membership is owned by the student record.
      const assignRes = await adminApi.put(`/api/students/${student.id}`, {
        data: {
          first_name: student.first_name,
          last_name: student.last_name,
          school_class: student.school_class,
          group_id: group.id,
        },
      });
      apiExpect(assignRes.status()).toBe(200);
      const assignBody = (await assignRes.json()) as {
        data: { group_id: number | null };
      };
      apiExpect(assignBody.data.group_id).toBe(group.id);

      // === The group's student roster now includes our test student.
      const membersRes = await adminApi.get(`/api/groups/${group.id}/students`);
      apiExpect(membersRes.status()).toBe(200);
      const membersBody = (await membersRes.json()) as {
        data: Array<{ id: number }>;
      };
      apiExpect(
        membersBody.data.find((member) => member.id === student.id),
      ).toBeDefined();

      // Order matters: the group's DELETE returns 409 while the student is
      // still attached, so delete the student first. The factories still
      // track both ids, so teardown cleans up leftovers if any assertion
      // above failed before we reached this point.
      const studentDel = await adminApi.delete(`/api/students/${student.id}`);
      apiExpect(studentDel.status()).toBe(200);
      const groupDel = await adminApi.delete(`/api/groups/${group.id}`);
      apiExpect(
        groupDel.status(),
        `group delete body: ${await groupDel.text()}`,
      ).toBe(200);
    },
  );
});

uiTest.describe("Group list UI", () => {
  uiTest(
    "admin sees seeded groups on /database/groups",
    async ({ authenticatedPage: page, app, groupVisibilityProbe }) => {
      await page.goto(app.primary(routes.groupsList));

      // Explicit state-backed fixture from the Go-owned e2e scenario. The seeder
      // chooses the canonical pair the list must render; specs no longer
      // infer it from lookup-map ordering on the frontend side.
      const [firstGroupName, secondGroupName] =
        groupVisibilityProbe.expectedVisibleNames;
      await uiExpect(
        page.getByRole("button", { name: new RegExp(firstGroupName) }).first(),
      ).toBeVisible({
        timeout: 20000,
      });
      await uiExpect(
        page.getByRole("button", { name: new RegExp(secondGroupName) }).first(),
      ).toBeVisible({
        timeout: 10000,
      });
    },
  );

  uiTest(
    "admin creates a group via the GroupCreateModal and the row appears in the list",
    async ({ authenticatedPage: page, app, adminApi, groupFactory }) => {
      // Issue #1142 lists "Erstellen" as a Gruppen task. The HTTP path
      // is covered by the API spec above; this UI test exercises the
      // open-modal → fill-form → submit → row-visible chain.
      const name = `UICreateGruppe ${uniqueSuffix()}`;

      await page.goto(app.primary(routes.groupsList));

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
      const res = await adminApi.get("/api/groups");
      uiExpect(res.status()).toBe(200);
      const body = (await res.json()) as {
        data: Array<{ id: number; name: string }>;
      };
      const match = body.data.find((g) => g.name === name);
      uiExpect(
        match,
        `group "${name}" not found via /api/groups`,
      ).toBeDefined();
      const id = match!.id;

      // Hand the id to the factory so its teardown deletes it.
      groupFactory.track(id);
    },
  );
});
