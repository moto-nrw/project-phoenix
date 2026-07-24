import { expect, type Page, test } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

// Smoke-Test des kindbezogenen Betreuungsplan-Tabs auf students/[id]
// (Kinderprofil-Planansicht). Prüft die End-to-End-Verdrahtung: der Tab
// erscheint, ruft GET /api/timetable/student/{id}/day, rendert die Tagesansicht
// (Ankunft/Freie Betreuung/Abholung als Zeitleiste) und lässt sich auf die
// Wochenansicht umschalten. KEINE Testdaten-Anlage nötig — reine Leseansicht.
//
// Login und Tenant-Herkunft spiegeln betreuungsplan-flow.spec.ts: der lokale
// Stack bedient die Tenant-App nur unter {slug}.localhost:3000, die Login-Maske
// liegt auf der Tenant-Wurzel `/`. Slug und Admin-Zugang stehen in
// backend/.seed-state.json; E2E_TENANT_SLUG / E2E_TEST_EMAIL / E2E_TEST_PASSWORD
// überschreiben sie. Ohne verwertbare Werte überspringt die Spec.

interface SeedAccess {
  slug: string;
  email: string;
  password: string;
}

function loadAccess(): SeedAccess | null {
  let slug = process.env.E2E_TENANT_SLUG;
  let email = process.env.E2E_TEST_EMAIL;
  let password = process.env.E2E_TEST_PASSWORD;
  try {
    const raw = readFileSync(
      join(process.cwd(), "..", "backend", ".seed-state.json"),
      "utf8",
    );
    const seed = JSON.parse(raw) as {
      bootstrap?: { tenant_slug?: string };
      accounts?: { admin?: Array<{ email?: string; password?: string }> };
    };
    const admin = seed.accounts?.admin?.[0];
    slug = slug ?? seed.bootstrap?.tenant_slug;
    email = email ?? admin?.email;
    password = password ?? admin?.password;
  } catch {
    // Keine Seed-Datei (z. B. CI ohne lokalen Stack): nur Env-Werte zählen.
  }
  if (slug && email && password) return { slug, email, password };
  return null;
}

const access = loadAccess();
const base = access ? `http://${access.slug}.localhost:3000` : "";
const DATA = { timeout: 15000 } as const;

async function login(page: Page, email: string, password: string) {
  await page.goto(`${base}/`, { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="email"]', { timeout: 20000 });
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', password);
  await page.click('button:has-text("Anmelden")');
  await page.waitForSelector('input[type="email"]', {
    state: "detached",
    timeout: 20000,
  });
}

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
    test.skip(
      access === null,
      "backend/.seed-state.json (oder E2E_TENANT_SLUG/E2E_TEST_EMAIL/E2E_TEST_PASSWORD) erforderlich",
    );
    if (!access) return;
    await login(page, access.email, access.password);
  });

  test("zeigt die Tagesansicht und schaltet auf Woche um", async ({ page }) => {
    const studentId = await firstStudentId(page);
    test.skip(studentId === null, "Kein Kind im Tenant gefunden");
    if (studentId === null) return;

    const dayResponse = page.waitForResponse(
      (r) =>
        /\/api\/timetable\/student\/\d+\/day/.test(r.url()) &&
        r.status() === 200,
      DATA,
    );
    await page.goto(`${base}/students/${studentId}?tab=betreuungsplan`, {
      waitUntil: "domcontentloaded",
    });

    // Tab is present and active.
    await expect(page.getByRole("tab", { name: "Betreuungsplan" })).toBeVisible(
      DATA,
    );
    // The day/week toggle + the fetch confirm the view mounted end-to-end.
    await expect(page.getByRole("tab", { name: "Tag" })).toBeVisible(DATA);
    await expect(page.getByRole("tab", { name: "Woche" })).toBeVisible(DATA);
    await dayResponse;
    await expect(
      page.getByRole("button", { name: "Nächster Tag" }),
    ).toBeVisible(DATA);

    // Switch to the week view.
    await page.getByRole("tab", { name: "Woche" }).click();
    await expect(
      page.getByRole("button", { name: "Vorherige Woche" }),
    ).toBeVisible(DATA);
  });
});
