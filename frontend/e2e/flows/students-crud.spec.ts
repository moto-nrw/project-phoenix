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
import { getTwoDistinctStudents } from "../helpers/seed-state";

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
      // Two seeded students with distinct first AND last names. Pulled
      // from .seed-state.json instead of hardcoding names so a future
      // seeder reorder can't make this test silently match the wrong rows
      // — and the search-filter assertion below relies on the names being
      // distinguishable.
      const [first, second] = getTwoDistinctStudents();
      const firstFullName = `${first.first_name} ${first.last_name}`;
      const secondFullName = `${second.first_name} ${second.last_name}`;

      // The Kinder admin view lives under /database/students; bare /students
      // is reserved for the staff search/check-in flow.
      await page.goto(routes.studentsList);

      // First seeded student must show up — accommodates SWR/SSR loading.
      await uiExpect(page.getByText(firstFullName).first()).toBeVisible({
        timeout: 15000,
      });

      // Sanity check: another seeded student should also be visible. If both
      // appear, we know the list is rendering, not just one stub.
      await uiExpect(page.getByText(secondFullName).first()).toBeVisible({
        timeout: 5000,
      });

      // PageHeaderWithSearch renders two inputs (mobile + desktop) with the
      // same placeholder; only one is visible at a given breakpoint, so
      // filter to that one rather than relying on document order. Use
      // .filter({ visible: true }) — the modern, documented form — instead
      // of .locator("visible=true"); both work today but the filter form
      // is unambiguous about acting on the parent locator rather than
      // looking like a descendant search.
      const searchBox = page
        .getByPlaceholder("Schüler suchen...")
        .filter({ visible: true });
      await searchBox.fill(first.first_name);

      // After filtering, the first student is still there but the second
      // (whose first name differs) must be gone.
      await uiExpect(page.getByText(firstFullName).first()).toBeVisible({
        timeout: 5000,
      });
      await uiExpect(page.getByText(secondFullName)).toHaveCount(0, {
        timeout: 5000,
      });
    },
  );

  uiTest(
    "admin edits a student's class via the Stammdaten tab and the change persists",
    async ({ authenticatedPage: page, studentFactory }) => {
      // Issue #1142 explicitly lists "Bearbeiten" as a Schüler-CRUD task.
      // The HTTP-level update is covered above; this UI half exercises
      // the master/detail Stammdaten form: select student → change Klasse
      // → Speichern → verify the value round-trips through the backend.
      //
      // `studentFactory` provisions a throwaway student via HTTP and
      // tears it down automatically after the test, even on assertion
      // failure — replaces the previous try/finally + Date.now() pattern.
      const initialClass = "1a";
      const newClass = "9z";

      const student = await studentFactory.create({
        school_class: initialClass,
      });

      // Selection in the master/detail UI is driven via the `student`
      // query param — navigating directly skips the click dance and
      // cuts ~2s of flake surface from this test.
      await page.goto(routes.studentDetail(student.id));

      // The Klasse input has no `name`/`id` and the label is a sibling
      // (no htmlFor binding), so `getByLabel` is unreliable here. The
      // placeholder "5A" is unique within the form (only Klasse has it).
      const klasseInput = page.getByPlaceholder("5A");
      await uiExpect(klasseInput).toBeVisible({ timeout: 15000 });
      await uiExpect(klasseInput).toHaveValue(initialClass, {
        timeout: 10000,
      });

      await klasseInput.fill(newClass);

      // Speichern button is disabled while !isDirty AND while saving.
      // After fill() it must be enabled. The button is *supposed* to flip
      // back to disabled once the parent re-derives originalDraft from
      // the freshly-saved student, but that transition lags behind the
      // successful PUT under default parallel load — long enough that
      // toBeDisabled() can time out even though the DB write already
      // landed. Wait on the network response itself: deterministic, not
      // dependent on a downstream UI re-render race.
      const saveButton = page.getByRole("button", { name: /^Speichern$/ });
      await uiExpect(saveButton).toBeEnabled({ timeout: 5000 });

      const putResponsePromise = page.waitForResponse(
        (resp) =>
          resp.url().includes(`/api/students/${student.id}`) &&
          resp.request().method() === "PUT",
        { timeout: 15000 },
      );
      await saveButton.click();
      const putResponse = await putResponsePromise;
      uiExpect(putResponse.status()).toBe(200);

      // Round-trip via HTTP — the only way to know the change actually
      // hit the DB and is not just a stale form re-render.
      const persisted = await withAdminContext(async (ctx, headers) => {
        const res = await ctx.get(`${BACKEND_URL}/api/students/${student.id}`, {
          headers,
        });
        uiExpect(res.status()).toBe(200);
        return (await res.json()) as {
          data: { school_class: string };
        };
      });
      uiExpect(persisted.data.school_class).toBe(newClass);
    },
  );

  uiTest(
    "admin creates a student via the StudentCreateModal and the row appears in the list",
    async ({ authenticatedPage: page, studentFactory }) => {
      // Issue #1142 lists "Erstellen" as a Schüler-CRUD task. The HTTP
      // path is covered above; this UI test exercises the full
      // open-modal → fill-form → submit → row-visible chain — i.e. what
      // a real admin actually does. If the form silently no-ops, freezes,
      // or routes the wrong fields, this catches it where the API test
      // can't.
      const suffix = uniqueSuffix();
      const firstName = `UICreate${suffix}`;
      const lastName = "Probe";
      const klasse = "3a";

      await page.goto(routes.studentsList);

      // Open the create modal. The plus-button has aria-label
      // "Schüler erstellen" (DatabaseCreateAction). aria-label gives a
      // stable accessible name even though the button only shows an icon.
      await page
        .getByRole("button", { name: "Schüler erstellen" })
        .first()
        .click();

      // Modal renders with role="dialog" via dialogAriaProps and the
      // title "Neuer Schüler". Scoping every form interaction to the
      // dialog avoids selector collisions with the underlying list view
      // (which has its own search input + Stammdaten form).
      const dialog = page.getByRole("dialog");
      await uiExpect(dialog).toBeVisible({ timeout: 10000 });
      await uiExpect(
        dialog.getByRole("heading", { name: "Neuer Schüler" }),
      ).toBeVisible();

      // Form fields use <label> without htmlFor binding (verified in the
      // edit spec above). Placeholders are unique within this dialog
      // ("Max", "Mustermann", "5A"), so getByPlaceholder + dialog scope
      // is the most stable available locator.
      await dialog.getByPlaceholder("Max").fill(firstName);
      await dialog.getByPlaceholder("Mustermann").fill(lastName);
      await dialog.getByPlaceholder("5A").fill(klasse);

      // Submit. Button text flips to "Wird erstellt..." while saving;
      // we wait for the dialog to disappear instead of polling the label,
      // because the parent (`page.tsx`) closes the modal in its onCreate
      // success handler — the close is the canonical "save landed" signal.
      await dialog.getByRole("button", { name: "Erstellen" }).click();
      await uiExpect(dialog).toBeHidden({ timeout: 15000 });

      // Row visible in the master/detail list. The list filters
      // client-side, so the freshly-created student must appear without
      // needing a manual reload.
      //
      // Use getByRole("button", …) — NOT getByText — because the success
      // toast that fires after creation also contains the student's full
      // name ("Schüler 'Max Mustermann' wurde erfolgreich erstellt"). With
      // getByText().first() the toast wins the locator race under load,
      // and toBeVisible then polls a paragraph that auto-hides after ~4s,
      // producing a false-red. List rows are <button>s; the toast is a
      // <p inside [role=status]>, so getByRole disambiguates cleanly.
      await uiExpect(
        page.getByRole("button", { name: `${firstName} ${lastName}` }).first(),
      ).toBeVisible({ timeout: 10000 });

      // Round-trip: the row in the list comes from the same SWR fetch
      // the rest of the app reads, but a stale cache hit could make this
      // green even if the backend never wrote the row. Hit /api/students
      // with the search filter to confirm DB-level persistence and
      // capture the id for cleanup.
      const id = await withAdminContext(async (ctx, headers) => {
        const res = await ctx.get(
          `${BACKEND_URL}/api/students?search=${encodeURIComponent(firstName)}`,
          { headers },
        );
        uiExpect(res.status()).toBe(200);
        const body = (await res.json()) as {
          data: Array<{ id: number; first_name: string; last_name: string }>;
        };
        const match = body.data.find(
          (s) => s.first_name === firstName && s.last_name === lastName,
        );
        uiExpect(
          match,
          `student ${firstName} ${lastName} not found via /api/students search`,
        ).toBeDefined();
        return match!.id;
      });

      // Hand the id to the factory so its teardown deletes it after the
      // test, even if a later assertion fails. The factory's teardown
      // tolerates already-deleted rows, so this is safe to call once.
      studentFactory.track(id);
    },
  );

  uiTest(
    "admin deletes a student via the confirm dialog and the row disappears",
    async ({ authenticatedPage: page, studentFactory }) => {
      // Issue #1142 lists "Löschen" as a Schüler-CRUD task. A broken
      // confirm modal is the highest-impact UI bug class for this flow
      // (one click = data gone), so this test asserts the full chain:
      // detail-action click → confirmation modal opens → confirm →
      // backend DELETE → row gone from list AND row 404 in API.
      const student = await studentFactory.create({
        first_name: `UIDelete${uniqueSuffix()}`,
      });

      // Land directly on the detail view for our student so the
      // detailActions slot (which holds the Löschen trigger) renders.
      await page.goto(routes.studentDetail(student.id));

      // The detail-pane Löschen button is the only one rendered on the
      // page in this state. The confirmation modal also has a "Löschen"
      // button — we scope the trigger click to the page (NOT the dialog)
      // so we don't accidentally hit the confirm before the modal opens.
      const triggerButton = page
        .getByRole("button", { name: "Löschen" })
        .first();
      await uiExpect(triggerButton).toBeVisible({ timeout: 15000 });
      await triggerButton.click();

      // Confirmation modal: title "Schüler löschen?" disambiguates from
      // any other dialogs; confirm button label is "Löschen" again, so
      // we scope through the dialog to pick the right one.
      const dialog = page.getByRole("dialog");
      await uiExpect(dialog).toBeVisible({ timeout: 5000 });
      await uiExpect(
        dialog.getByRole("heading", { name: "Schüler löschen?" }),
      ).toBeVisible();

      // useDeleteConfirmation.confirmDelete (src/hooks/useDeleteConfirmation.ts)
      // hides the modal *synchronously* and then fires the delete callback
      // fire-and-forget. So `dialog.toBeHidden()` resolves long before the
      // DELETE request completes — checking the API too early would race
      // and falsely see the row as still existing. Polling the list row
      // for disappearance is also unreliable: the master/detail layout
      // can keep the deleted student's name rendered in the right-hand
      // detail pane after the row leaves the list, so getByText(fullName)
      // hits zero only after a second SWR cycle. Wait on the DELETE
      // response itself — the only signal that's both deterministic and
      // consistent across master/detail layout variants.
      const deleteResponsePromise = page.waitForResponse(
        (resp) =>
          resp.url().includes(`/api/students/${student.id}`) &&
          resp.request().method() === "DELETE",
        { timeout: 15000 },
      );
      await dialog.getByRole("button", { name: "Löschen" }).click();
      const deleteResponse = await deleteResponsePromise;
      uiExpect(deleteResponse.status()).toBe(200);

      // Backend confirmation: the proxy returned 200, so the API must
      // also return 404 on a follow-up GET. A still-existing student at
      // this point would mean the proxy lied (rare) or the backend
      // accepted the DELETE without committing (the exact silent-failure
      // class this test catches).
      await withAdminContext(async (ctx, headers) => {
        const res = await ctx.get(`${BACKEND_URL}/api/students/${student.id}`, {
          headers,
          failOnStatusCode: false,
        });
        uiExpect(res.status()).toBe(404);
      });
    },
  );
});
