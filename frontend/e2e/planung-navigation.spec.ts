import { expect, type Page, test } from "@playwright/test";
import { parseISODate, toISODate } from "../src/lib/date-helpers";

// Planung-Redesign (docs/planung-redesign/docs/03): flache
// flache Sidebar-Einträge statt des Planung-Akkordeons, /planung und die
// Alt-Stubs als Redirects mit Parameter-Übersetzung in das neue
// Drei-Parameter-Schema (d, view, block/verlauf).

// Test credentials from environment variables (never commit real credentials!).
// No fallback defaults per the repo no-fallback policy — the suite skips when
// either is unset (see beforeEach) rather than substituting an empty string.
const TEST_EMAIL = process.env.E2E_TEST_EMAIL;
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD;

async function login(page: Page, email: string, password: string) {
  await page.goto("/");
  await page.waitForSelector('input[type="email"]');
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', password);
  await page.click('button:has-text("Anmelden")');
  await page.waitForURL(
    (url) => !url.pathname.includes("login") && url.pathname !== "/",
    { timeout: 15000 },
  );
}

function todayPlusDays(days: number): string {
  const anchor = parseISODate(toISODate(new Date()));
  anchor.setDate(anchor.getDate() + days);
  return toISODate(anchor);
}

test.describe("Planung navigation", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(
      !TEST_EMAIL || !TEST_PASSWORD,
      "E2E_TEST_EMAIL and E2E_TEST_PASSWORD must be set",
    );
    if (!TEST_EMAIL || !TEST_PASSWORD) return;
    await login(page, TEST_EMAIL, TEST_PASSWORD);
  });

  test("sidebar shows the three flat planning entries and navigates each area", async ({
    page,
  }) => {
    const sidebar = page.locator("aside");

    // Kein Planung-Akkordeon mehr, keine Kalenderzeiträume in der Sidebar.
    await expect(sidebar.getByText("Betreuungsplan")).toBeVisible();
    await expect(sidebar.getByText("Dienstplan")).toBeVisible();
    await expect(
      sidebar.getByText("Terminvertretungen", { exact: true }),
    ).toBeVisible();
    await expect(sidebar.getByText("Planungsübersicht")).toHaveCount(0);
    await expect(sidebar.getByText("Kalenderzeiträume")).toHaveCount(0);
    // Der zentrale Einstieg bündelt alle Vertretungsvorgänge.
    await expect(
      sidebar.getByText("Vertretungen", { exact: true }),
    ).toBeVisible();
    await expect(sidebar.getByText("Übergaben")).toHaveCount(0);

    await sidebar.getByText("Betreuungsplan").click();
    await page.waitForURL((url) => url.pathname.endsWith("/betreuungsplan"));

    await sidebar.getByText("Dienstplan").click();
    await page.waitForURL((url) => url.pathname.endsWith("/dienstplan"));

    await sidebar.getByText("Terminvertretungen", { exact: true }).click();
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
    // -> /betreuungsplan?d={heute+7}&view=woche&block=42
    await page.goto("/timetables?view=week&week=1&instance=42");
    await page.waitForURL((url) => url.pathname.endsWith("/betreuungsplan"));
    const betreuungsplanUrl = new URL(page.url());
    expect(betreuungsplanUrl.searchParams.get("d")).toBe(todayPlusDays(7));
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
    await expect(main.getByText("Termine", { exact: true })).toBeVisible();
    await expect(main.getByText("Gruppen", { exact: true })).toBeVisible();
    await expect(main.getByText("Gruppenzugriff")).toHaveCount(0);
  });

  test("mobile Mehr drawer lists the three planning areas", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/dashboard");
    await page.getByRole("button", { name: "Mehr" }).click();

    await expect(page.getByText("Betreuungsplan")).toBeVisible();
    await expect(page.getByText("Dienstplan")).toBeVisible();
    await expect(
      page.getByText("Terminvertretungen", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText("Vertretungen", { exact: true })).toBeVisible();
    await expect(page.getByText("Planung", { exact: true })).toHaveCount(0);
  });
});
