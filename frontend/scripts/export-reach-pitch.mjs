import { chromium } from "@playwright/test";
import { PDFDocument } from "pdf-lib";
import fs from "node:fs/promises";
import path from "node:path";

const root = process.cwd();
const outDir = path.join(root, "../outputs/reach-pitch-web");
const qaDir = path.join(outDir, "qa");
const url = "http://localhost:3001/reach-pitch";
const scale = Number.parseInt(process.env.PITCH_EXPORT_SCALE ?? "2", 10);
const safeScale = Number.isFinite(scale) && scale > 0 ? scale : 2;

await fs.mkdir(qaDir, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({
  deviceScaleFactor: safeScale,
  viewport: { width: 1600, height: 900 },
});
await page.goto(url, { waitUntil: "networkidle" });

const slides = await page.locator(".pitch-slide").count();
const pdf = await PDFDocument.create();

for (let index = 0; index < slides; index += 1) {
  const slide = page.locator(".pitch-slide").nth(index);
  const screenshot = await slide.screenshot({
    path: path.join(
      qaDir,
      `slide-${String(index + 1).padStart(2, "0")}@${safeScale}x.png`,
    ),
  });
  const image = await pdf.embedPng(screenshot);
  const pdfPage = pdf.addPage([1600, 900]);
  pdfPage.drawImage(image, { x: 0, y: 0, width: 1600, height: 900 });
}

await page.pdf({
  path: path.join(outDir, "moto-reach-pitch-web-16x9.pdf"),
  width: "16in",
  height: "9in",
  printBackground: true,
  preferCSSPageSize: true,
});

await fs.writeFile(
  path.join(outDir, `moto-reach-pitch-web-slides-${safeScale}x.pdf`),
  await pdf.save(),
);

await browser.close();
