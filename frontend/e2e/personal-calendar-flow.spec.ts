import { expect, test, type Page } from "@playwright/test";

import {
  loginToTenant,
  MISSING_SEED_ACCESS,
  pickDate,
  seedAccess as access,
  tenantBase as base,
  waitForAnimations,
} from "./tenant-session";
import { toISODate } from "../src/lib/date-helpers";

// Login und Tenant-Herkunft: e2e/tenant-session.ts.
// Relative page.goto-Ziele laufen gegen die Tenant-Herkunft.
if (access) test.use({ baseURL: base });

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}

// The fixture uses today's date, including Saturday/Sunday. Make those days
// visible explicitly rather than depending on the work-week default.
async function showFullWeek(page: Page): Promise<void> {
  const weekends = page.getByRole("button", { name: /^Sa\/So/ });
  if ((await weekends.getAttribute("aria-pressed")) !== "true")
    await weekends.click();
  await expect(weekends).toHaveAttribute("aria-pressed", "true");
}

// Der angelegte Termin wird wieder gelöscht. Sonst stapeln sich bei
// wiederholten Läufen gleichzeitige Blöcke im Wochenraster und überdecken
// einander.
let createdAppointmentId: string | null = null;

test.describe("Personal calendar flow", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(access === null, MISSING_SEED_ACCESS);
    if (!access) return;
    await loginToTenant(page, access);
  });

  test.afterEach(async ({ page }) => {
    if (!createdAppointmentId) return;
    const id = createdAppointmentId;
    createdAppointmentId = null;
    const res = await page.request.delete(
      `${base}/api/calendar/appointments/${id}`,
    );
    expect(
      res.ok(),
      `Termin ${id} löschen fehlgeschlagen (${res.status()})`,
    ).toBe(true);
  });

  test("staff creates, accepts, and sees a recurring all-staff appointment", async ({
    page,
  }) => {
    // Kalt kompiliert der Next-Dev-Server /calendar und die Detailansicht
    // spürbar langsam; gleiches Budget wie die übrigen Tenant-Flows.
    test.setTimeout(90000);
    const today = new Date();
    const startDate = toISODate(today);
    const recurrenceEnd = toISODate(addDays(today, 7));
    const title = `Playwright Kalendertermin ${Date.now()}`;

    await page.goto("/calendar");
    await expect(
      page.getByRole("heading", { name: "Mein Kalender" }),
    ).toBeVisible();
    await showFullWeek(page);

    await page.getByRole("button", { name: "Neuer Termin" }).click();
    const createHeading = page.getByRole("heading", {
      name: "Termin erstellen",
    });
    await expect(createHeading).toBeVisible();
    await waitForAnimations(
      page.getByRole("dialog").filter({ has: createHeading }),
    );

    // Rolle statt getByLabel: die Checkbox "Eltern per E-Mail benachrichtigen"
    // trägt "Titel" in ihrer Beschreibung, "Ort" steckt in weiteren Labels.
    await page.getByRole("textbox", { name: "Titel", exact: true }).fill(title);
    await page
      .getByRole("textbox", { name: "Ort", exact: true })
      .fill("E2E Raum");
    await pickDate(
      page,
      page.getByRole("button", { name: "Startdatum", exact: true }),
      startDate,
    );
    await pickDate(
      page,
      page.getByRole("button", { name: "Enddatum", exact: true }),
      startDate,
    );
    await page.getByLabel("Startzeit").fill("09:00");
    await page.getByLabel("Endzeit").fill("10:00");
    // Kit-Checkbox: das <input> ist sr-only, klickbar ist das umschließende
    // Label. Danach derselbe Endzustand wie zuvor mit .check().
    const allStaff = page.getByLabel("Alle Mitarbeitenden", { exact: true });
    await page.locator("label", { has: allStaff }).click();
    await expect(allStaff).toBeChecked();
    await expect(
      page.getByText("Alle Mitarbeitenden: Alle Mitarbeitenden"),
    ).toBeVisible();
    await page.getByRole("combobox", { name: "Wiederholung" }).click();
    await page.getByRole("option", { name: "Wöchentlich" }).click();
    await pickDate(
      page,
      page.getByRole("button", { name: "Endet am", exact: true }),
      recurrenceEnd,
    );

    const createResponse = page.waitForResponse(
      (response) =>
        response.url().includes("/api/calendar/appointments") &&
        response.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Termin speichern" }).click();
    const created = await createResponse;
    await expect(created.ok()).toBeTruthy();
    // Antwort: Termin-Detail ({ appointment: { id, … }, … }).
    const createdJson = (await created.json()) as {
      data?: { appointment?: { id?: string | number } };
    };
    const createdId = createdJson.data?.appointment?.id;
    expect(createdId, "Termin-Anlage muss eine ID liefern").toBeTruthy();
    createdAppointmentId = String(createdId);

    // Seit 0fccd5815c ("tap-to-detail") ist ein Termin im Raster ein
    // kompakter Block; Antwortstatus, Ort und Zusagen/Absagen trägt die
    // Detailansicht "Termin", die ein Klick auf den Block öffnet.
    const currentWeekEvent = page.getByRole("button", { name: title });
    await expect(currentWeekEvent).toBeVisible();
    await currentWeekEvent.click();
    const eventDetail = page.getByRole("dialog", { name: "Termin" });
    await expect(eventDetail).toContainText(title);
    await expect(eventDetail).toContainText("Offen");
    await expect(eventDetail).toContainText("E2E Raum");
    await waitForAnimations(eventDetail);

    const responseUpdate = page.waitForResponse(
      (response) =>
        response.url().includes("/api/calendar/recipients/") &&
        response.url().includes("/response") &&
        response.request().method() === "POST",
    );
    await eventDetail.getByRole("button", { name: "Zusagen" }).click();
    await expect((await responseUpdate).ok()).toBeTruthy();
    // Zusagen schließt die Detailansicht; der neue Status steht nach erneutem
    // Öffnen darin, ohne Neuladen der Seite.
    await expect(eventDetail).toBeHidden();
    await currentWeekEvent.click();
    await expect(eventDetail).toContainText("Zugesagt");
    await waitForAnimations(eventDetail);

    // Gespeichert im Backend: nach frischem Laden der Seite steht die Zusage
    // weiterhin im Termin.
    await page.reload({ waitUntil: "domcontentloaded" });
    await showFullWeek(page);
    await expect(currentWeekEvent).toBeVisible();
    await currentWeekEvent.click();
    await expect(eventDetail).toContainText("Zugesagt");
    await waitForAnimations(eventDetail);

    await eventDetail.getByRole("button", { name: "Schließen" }).click();
    await page.getByRole("button", { name: "Nächste Woche" }).click();
    await expect(page.getByRole("button", { name: title })).toBeVisible();
  });
});
