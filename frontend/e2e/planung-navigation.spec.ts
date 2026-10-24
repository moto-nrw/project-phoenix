import { expect, test } from "@playwright/test";

import {
  loginToTenant,
  MISSING_SEED_ACCESS,
  seedAccess as access,
  tenantBase as base,
} from "./tenant-session";
import {
  berlinTodayISO,
  parseISODate,
  toISODate,
} from "../src/lib/date-helpers";
import { getWeekRange } from "../src/lib/timetable-helpers";

// Flache Sidebar-Einträge statt des Planung-Akkordeons, /planung und die
// Alt-Stubs als Redirects mit Parameter-Übersetzung in das neue
// Drei-Parameter-Schema (d, view, block/verlauf).

// Login und Tenant-Herkunft: e2e/tenant-session.ts.
// Relative page.goto-Ziele laufen gegen die Tenant-Herkunft.
if (access) test.use({ baseURL: base });

// `?week=N` zeigt seit c39bf33169 auf den Montag der Zielwoche (docs/07 §1),
// nicht auf heute + N*7 Tage; die Datumsregel prüft planung-redirect.test.ts
// ("resolvePlanungRedirect week offset snaps to Monday").
function mondayOfWeekOffset(offset: number): string {
  return toISODate(getWeekRange(parseISODate(berlinTodayISO()), offset).from);
}

test.describe("Planung navigation", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(access === null, MISSING_SEED_ACCESS);
    if (!access) return;
    await loginToTenant(page, access);
  });

  test("sidebar shows the planning pages inside the Planung group and navigates each area", async ({
    page,
  }) => {
    const sidebar = page.locator("aside");

    // Die Gruppe Planung (#2826) ist beim ersten Besuch zu; die Gruppenzeile
    // klappt sie auf, ohne zu navigieren.
    const planung = sidebar.getByRole("button", { name: "Planung" });
    await expect(planung).toHaveAttribute("aria-expanded", "false");
    await planung.click();
    await expect(planung).toHaveAttribute("aria-expanded", "true");

    await expect(sidebar.getByText("Betreuungsplan")).toBeVisible();
    // exact: "Mein Dienstplan" (Gruppe Team) enthält "Dienstplan" ebenfalls.
    await expect(
      sidebar.getByText("Dienstplan", { exact: true }),
    ).toBeVisible();
    await expect(
      sidebar.getByText("Vertretungsplan", { exact: true }),
    ).toBeVisible();
    await expect(sidebar.getByText("Schuljahr und Kalender")).toBeVisible();
    await expect(sidebar.getByText("Planungsübersicht")).toHaveCount(0);
    // Der zentrale Einstieg bündelt alle Vertretungsvorgänge; er steht im
    // Tagesbetrieb, der standardmäßig offen ist.
    await expect(sidebar.getByText("Übergaben", { exact: true })).toBeVisible();
    await expect(
      sidebar.getByText("Vertretungen", { exact: true }),
    ).toHaveCount(0);

    await sidebar.getByText("Betreuungsplan").click();
    await page.waitForURL((url) => url.pathname.endsWith("/betreuungsplan"));

    await sidebar.getByText("Dienstplan", { exact: true }).click();
    await page.waitForURL((url) => url.pathname.endsWith("/dienstplan"));

    await sidebar.getByText("Vertretungsplan", { exact: true }).click();
    await page.waitForURL((url) => url.pathname.endsWith("/vertretung"));
  });

  test("legacy deep links redirect with translated params", async ({
    page,
  }) => {
    // /planung?tab=dienstplan -> /dienstplan
    await page.goto("/planung?tab=dienstplan");
    await page.waitForURL(
      (url) => url.pathname.endsWith("/dienstplan") && url.search === "",
    );

    // /planung ohne tab -> /betreuungsplan
    await page.goto("/planung");
    await page.waitForURL((url) => url.pathname.endsWith("/betreuungsplan"));

    // /timetables?view=week&week=1&instance=42
    // -> /betreuungsplan?d={Montag der nächsten Woche}&view=woche&block=42
    await page.goto("/timetables?view=week&week=1&instance=42");
    await page.waitForURL((url) => url.pathname.endsWith("/betreuungsplan"));
    const betreuungsplanUrl = new URL(page.url());
    expect(betreuungsplanUrl.searchParams.get("d")).toBe(mondayOfWeekOffset(1));
    expect(betreuungsplanUrl.searchParams.get("view")).toBe("woche");
    expect(betreuungsplanUrl.searchParams.get("block")).toBe("42");
    expect([...betreuungsplanUrl.searchParams.keys()].sort()).toEqual([
      "block",
      "d",
      "view",
    ]);

    // /vertretungsplan?instance=42&history=1 -> /vertretung?block=42&verlauf=1
    await page.goto("/vertretungsplan?instance=42&history=1");
    await page.waitForURL((url) => url.pathname.endsWith("/vertretung"));
    const vertretungUrl = new URL(page.url());
    expect(vertretungUrl.searchParams.get("block")).toBe("42");
    expect(vertretungUrl.searchParams.get("verlauf")).toBe("1");
    expect([...vertretungUrl.searchParams.keys()].sort()).toEqual([
      "block",
      "verlauf",
    ]);

    // /staff/dienstplan -> /dienstplan ohne Query. Wichtig: die Start-URL
    // endet selbst auf "/dienstplan", deshalb explizit auf das Verschwinden
    // des /staff-Präfixes warten, sonst erfüllt sich die Assertion sofort.
    await page.goto("/staff/dienstplan");
    await page.waitForURL(
      (url) =>
        url.pathname.endsWith("/dienstplan") &&
        !url.pathname.includes("/staff/") &&
        url.search === "",
    );
  });

  test("/substitutions stays its own page", async ({ page }) => {
    await page.goto("/substitutions");
    await page.waitForLoadState("networkidle");
    expect(new URL(page.url()).pathname.endsWith("/substitutions")).toBe(true);
    const main = page.locator("main");
    await expect(main.getByText("Laufende Betreuungen")).toBeVisible();
    await expect(
      main.getByText("Vertretungen heute", { exact: true }),
    ).toBeVisible();
    await expect(main.getByText("Gruppen", { exact: true })).toBeVisible();
    await expect(main.getByText("Gruppenzugriff")).toHaveCount(0);
  });

  test("mobile Mehr drawer lists the Planung group and its pages", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/dashboard");
    await page.getByRole("button", { name: "Mehr" }).click();
    // Auf die Mehr-Schublade begrenzt: die eingeklappte Desktop-Seitenleiste
    // bleibt im DOM und trägt dieselben Beschriftungen.
    const drawer = page.getByRole("dialog", { name: "Navigation" });

    await expect(drawer.getByText("Betreuungsplan")).toBeVisible();
    // exact: "Mein Dienstplan" (Gruppe Team) enthält "Dienstplan" ebenfalls.
    await expect(drawer.getByText("Dienstplan", { exact: true })).toBeVisible();
    await expect(
      drawer.getByText("Vertretungsplan", { exact: true }),
    ).toBeVisible();
    await expect(drawer.getByText("Übergaben", { exact: true })).toBeVisible();
    await expect(drawer.getByText("Planung", { exact: true })).toBeVisible();
  });
});
