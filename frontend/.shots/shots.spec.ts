import { test, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const BASE = "http://demo-school.localhost:3040";
const OUT = "/Users/theitger/Downloads/2619-flo-screens";
const EMAIL = "demo1@mail.de";
const PASSWORD = "Test1234%";

const DESKTOP = { width: 1440, height: 900 };
const MOBILE = { width: 390, height: 844 };

type Target = {
  name: string;
  path: string;
  after?: (page: Page) => Promise<void>;
};

const openRoomDrawer = async (page: Page) => {
  const tile = page.getByRole("button", { name: /Aula/ }).first();
  await tile.waitFor({ state: "visible", timeout: 60_000 });
  await page.waitForTimeout(1500);
  await tile.click();
  await page
    .locator("#room-detail-panel")
    .first()
    .waitFor({ state: "visible", timeout: 60_000 });
  await page.waitForTimeout(3000);
};

const TARGETS: Target[] = [
  { name: "mitarbeiter-detail", path: "/staff/3" },
  { name: "raum-drawer", path: "/rooms", after: openRoomDrawer },
  { name: "dienstplan", path: "/dienstplan" },
  { name: "anmeldungen-liste", path: "/admin/enrollments" },
  { name: "anmeldung-detail", path: "/admin/enrollments/24" },
  { name: "anmeldephasen", path: "/enrollment-phases" },
  { name: "anmeldephase-pruefen", path: "/enrollment-phases/1/review" },
  { name: "anmeldeformular", path: "/enrollment-form" },
];

async function login(page: Page) {
  await page.context().clearCookies();
  await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
  await page
    .locator('input[type="email"], input[name="email"]')
    .first()
    .fill(EMAIL);
  await page.locator('input[type="password"]').first().fill(PASSWORD);
  await page.locator('button[type="submit"]').first().click();
  await page.waitForURL((url) => !url.pathname.includes("/login"), {
    timeout: 60_000,
  });
  await page.waitForTimeout(2000);
}

async function settle(page: Page, maxMs = 90_000) {
  const t0 = Date.now();
  let quiet = 0;
  while (Date.now() - t0 < maxMs) {
    let v = 1;
    try {
      v = await page.evaluate(() => {
        const busy = document.querySelectorAll('[aria-busy="true"]').length;
        const pulse = [...document.querySelectorAll(".animate-pulse")].filter(
          (e) => e.getClientRects().length > 0,
        ).length;
        return busy + pulse;
      });
    } catch {
      // Navigation in flight (query-param replace, redirect): try again.
      v = 1;
    }
    if (v === 0) {
      quiet += 1;
      if (quiet >= 2) return true;
    } else {
      quiet = 0;
    }
    await page.waitForTimeout(2000);
  }
  return false;
}

async function shoot(
  page: Page,
  target: Target,
  file: string,
  viewport: { width: number; height: number },
  fullPage: boolean,
) {
  await page.setViewportSize(viewport);
  const go = async () => {
    for (let attempt = 0; attempt < 3; attempt += 1) {
      try {
        await page.goto(`${BASE}${target.path}`, {
          waitUntil: "domcontentloaded",
        });
        return;
      } catch {
        // ERR_ABORTED: a client redirect cancelled the navigation; retry.
        await page.waitForTimeout(1500);
      }
    }
  };
  await go();
  await page.waitForTimeout(1000);
  if (page.url().includes("/login")) {
    await login(page);
    await go();
  }
  const settled = await settle(page);
  if (target.after) await target.after(page);
  await page.evaluate(() =>
    document.querySelectorAll("nextjs-portal").forEach((e) => e.remove()),
  );
  await page.waitForTimeout(500);
  await page.screenshot({ path: file, fullPage });
  const h1 = await page.evaluate(() =>
    (document.querySelector("h1")?.textContent ?? "").trim(),
  );
  const alerts = await page.evaluate(
    () => document.querySelectorAll("[role=alert]").length,
  );
  console.log(
    `${path.basename(file)}: settled=${settled} h1="${h1}" alerts=${alerts} url=${page.url()}`,
  );
}

test("flo screens", async ({ page }) => {
  test.setTimeout(30 * 60_000);
  fs.mkdirSync(OUT, { recursive: true });
  await login(page);
  for (const target of TARGETS) {
    await shoot(
      page,
      target,
      path.join(OUT, `d-${target.name}.png`),
      DESKTOP,
      true,
    );
    await shoot(
      page,
      target,
      path.join(OUT, `m-${target.name}.png`),
      MOBILE,
      false,
    );
  }
});
