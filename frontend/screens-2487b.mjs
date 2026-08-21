import { chromium } from "@playwright/test";
const OUT = "/Users/theitger/Downloads/2487-screens";
const BASE = "http://demo-school.localhost:3000";

const browser = await chromium.launch({
  args: ["--host-resolver-rules=MAP *.localhost 127.0.0.1"],
});
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
const shot = async (name) => {
  await page.evaluate(() => document.querySelector("nextjs-portal")?.remove());
  await page.screenshot({ path: `${OUT}/${name}.png` });
  console.log("saved", name);
};
const dialog = () => page.locator("[data-modal-focus-scope]");

await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
await page.getByLabel("E-Mail-Adresse").fill("demo1@mail.de");
await page.getByLabel("Passwort", { exact: true }).fill("Test1234%");
await page.locator('button[type="submit"]').first().click();
await page.waitForURL("**/dashboard", { timeout: 60000 });

// ---- detail actions on the child with a planned exit -------------------
await page.goto(`${BASE}/database/students`, { waitUntil: "domcontentloaded" });
await page.waitForTimeout(6000);
await page.getByRole("textbox", { name: "Kinder suchen..." }).fill("Bauer");
await page.waitForTimeout(1500);
await page.getByRole("button", { name: /Mia Bauer/ }).first().click();
await page.waitForTimeout(2500);
await shot("07-detail-geplantes-ende");

// the child whose care already ended is gone from the normal list
await page.getByRole("textbox", { name: "Kinder suchen..." }).fill("Albrecht");
await page.waitForTimeout(2000);
await shot("08-beendetes-kind-nicht-in-liste");

// ---- archive ------------------------------------------------------------
await page.goto(`${BASE}/database/students/ended-care`, { waitUntil: "domcontentloaded" });
await page.waitForTimeout(5000);
await shot("09-beendete-betreuungen");

await page.getByRole("button", { name: "Wieder aufnehmen" }).first().click();
await page.waitForTimeout(1500);
await shot("10-wiederaufnahme");

await dialog().locator('label[for="care-resume-checked"]').click();
await page.waitForTimeout(500);
await shot("11-wiederaufnahme-bestaetigt");

// ---- mobile -------------------------------------------------------------
await page.setViewportSize({ width: 390, height: 844 });
await page.goto(`${BASE}/database/students/ended-care`, { waitUntil: "domcontentloaded" });
await page.waitForTimeout(4000);
await shot("12-mobil-beendete-betreuungen");

await page.goto(`${BASE}/database/students`, { waitUntil: "domcontentloaded" });
await page.waitForTimeout(5000);
await page.getByRole("textbox", { name: "Kinder suchen..." }).fill("Bauer");
await page.waitForTimeout(1500);
await page.getByRole("button", { name: /Mia Bauer/ }).first().click();
await page.waitForTimeout(2500);
await shot("13-mobil-detail");

await browser.close();
