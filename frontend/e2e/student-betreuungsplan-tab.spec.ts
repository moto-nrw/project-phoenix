import { expect, type Page, test } from "@playwright/test";

import {
  loginToTenant,
  MISSING_SEED_ACCESS,
  seedAccess as access,
  tenantBase as base,
} from "./tenant-session";

// Smoke-Test des kindbezogenen Betreuungsplan-Tabs auf students/[id]
// (Kinderprofil-Planansicht). Prüft die End-to-End-Verdrahtung: der Tab
// erscheint, ruft GET /api/timetable/student/{id}/day, rendert die Tagesansicht
// (Ankunft/Freie Betreuung/Abholung als Zeitleiste) und lässt sich auf die
// Wochenansicht umschalten. KEINE Testdaten-Anlage nötig — reine Leseansicht.
//
// Login und Tenant-Herkunft: e2e/tenant-session.ts.

const DATA = { timeout: 15000 } as const;

/** Grabs the first student id via the API (admin sees all). */
async function firstStudentId(page: Page): Promise<string | null> {
  const res = await page.request.get(`${base}/api/students?page_size=1`);
  if (!res.ok()) return null;
  // The tenant route returns a wrapped paginated shape:
  // { data: { data: Student[], pagination } }.
  const body = (await res.json()) as {
    data?:
      | { data?: Array<{ id: string | number }> }
      | Array<{ id: string | number }>;
  };
  const container = body.data;
  const list = Array.isArray(container) ? container : (container?.data ?? []);
  const id = list[0]?.id;
  return id != null ? String(id) : null;
}

test.describe("Kinderprofil Betreuungsplan-Tab", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(access === null, MISSING_SEED_ACCESS);
    if (!access) return;
    await loginToTenant(page, access);
  });

  test("zeigt die Tagesansicht und schaltet auf Woche um", async ({ page }) => {
    // Cold profile and timetable routes compile on the CI dev server.
    test.setTimeout(120000);
    const studentId = await firstStudentId(page);
    test.skip(studentId === null, "Kein Kind im Tenant gefunden");
    if (studentId === null) return;

    const [dayResponse] = await Promise.all([
      page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname ===
            `/api/timetable/student/${studentId}/day` &&
          r.request().method() === "GET",
        { timeout: 60000 },
      ),
      page.goto(`${base}/students/${studentId}?tab=betreuungsplan`, {
        waitUntil: "domcontentloaded",
      }),
    ]);
    expect(dayResponse.status()).toBe(200);

    // Tab is present and active.
    await expect(
      page.getByRole("tab", { name: "Tages- und Wochenplan" }),
    ).toBeVisible(DATA);
    // The day/week toggle + the fetch confirm the view mounted end-to-end.
    // Seit 7b2e83b328 ist der Umschalter ein SegmentedControl (Toggle-Buttons
    // mit aria-pressed in der Gruppe "Ansicht"), keine Tabs mehr.
    const viewToggle = page.getByRole("group", { name: "Ansicht" });
    await expect(viewToggle.getByRole("button", { name: "Tag" })).toBeVisible(
      DATA,
    );
    await expect(viewToggle.getByRole("button", { name: "Woche" })).toBeVisible(
      DATA,
    );
    await expect(
      page.getByRole("button", { name: "Nächster Tag" }),
    ).toBeVisible(DATA);

    // Switch to the week view.
    await viewToggle.getByRole("button", { name: "Woche" }).click();
    await expect(
      page.getByRole("button", { name: "Vorherige Woche" }),
    ).toBeVisible(DATA);
  });
});
