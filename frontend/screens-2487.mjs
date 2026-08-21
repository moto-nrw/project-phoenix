import { chromium } from "@playwright/test";
import { mkdirSync } from "node:fs";

const OUT = "/Users/theitger/Downloads/2487-screens";
const BASE = "http://demo-school.localhost:3000";
mkdirSync(OUT, { recursive: true });

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

const open = async () => {
  await page.goto(`${BASE}/database/students`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(6000);
};

// ---- 1) Sammelauswahl -------------------------------------------------
await open();
await page.getByRole("textbox", { name: "Kinder suchen..." }).fill("Bauer");
await page.waitForTimeout(1200);
await page.getByRole("button", { name: "Auswählen" }).click();
await page.waitForTimeout(1200);
await shot("02-auswahlmodus");

await page.getByRole("button", { name: /Alle \d+ auswählen/ }).click();
await page.waitForTimeout(800);
await shot("03-alle-angezeigten-ausgewaehlt");

await page.getByRole("button", { name: "Betreuung beenden", exact: true }).first().click();
await page.waitForTimeout(1500);

// Grund + Freitext
await dialog().getByRole("combobox", { name: "Grund" }).click();
await page.waitForTimeout(400);
await page.getByRole("option", { name: "Umzug" }).click();
await page.waitForTimeout(600);
await shot("04-modal-angaben");

await dialog().getByRole("button", { name: "Weiter", exact: true }).click();
await page.waitForTimeout(3000);
await shot("05-modal-vorschau");

await dialog().getByRole("button", { name: "Betreuung beenden", exact: true }).click();
await page.waitForTimeout(5000);
await shot("06-liste-mit-geplantem-ende");

await browser.close();
