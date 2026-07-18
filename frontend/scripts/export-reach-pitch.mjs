import { chromium } from "@playwright/test";
import { PDFDocument } from "pdf-lib";
import fs from "node:fs/promises";
import path from "node:path";

const defaults = {
  from: 1,
  outDir: path.resolve(process.cwd(), "../outputs/reach-pitch-web"),
  scale: 4,
  url: "http://localhost:3001/reach-pitch",
  viewportHeight: 1080,
  viewportWidth: 1920,
};

function readOption(name) {
  const index = process.argv.indexOf(`--${name}`);
  if (index === -1) return undefined;
  return process.argv[index + 1];
}

function readNumberOption(name, defaultValue) {
  const rawValue = readOption(name);
  if (rawValue === undefined) return defaultValue;

  const value = Number.parseInt(rawValue, 10);
  if (!Number.isFinite(value) || value < 1) {
    throw new Error(`--${name} must be a positive integer`);
  }

  return value;
}

function readStringOption(name, defaultValue) {
  const value = readOption(name);
  if (value === undefined) return defaultValue;
  if (value.trim().length === 0) {
    throw new Error(`--${name} must not be empty`);
  }
  return value;
}

const config = {
  from: readNumberOption("from", defaults.from),
  outDir: path.resolve(readStringOption("out-dir", defaults.outDir)),
  scale: readNumberOption("scale", defaults.scale),
  to: readNumberOption("to", Number.MAX_SAFE_INTEGER),
  url: readStringOption("url", defaults.url),
  viewportHeight: readNumberOption("viewport-height", defaults.viewportHeight),
  viewportWidth: readNumberOption("viewport-width", defaults.viewportWidth),
};

if (config.to < config.from) {
  throw new Error("--to must be greater than or equal to --from");
}

const qaDir = path.join(config.outDir, "qa");
const firstSlideIndex = config.from - 1;
const pdfPageWidth = 16 * 72;
const pdfPageHeight = 9 * 72;

await fs.mkdir(qaDir, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({
  deviceScaleFactor: config.scale,
  viewport: {
    height: config.viewportHeight,
    width: config.viewportWidth,
  },
});

await page.goto(config.url, { waitUntil: "networkidle" });
await page.evaluate(async () => {
  await document.fonts.ready;
});

await page.addStyleTag({
  content: `
    nextjs-portal {
      display: none !important;
    }

    .pitch-slide {
      border: 0 !important;
      border-radius: 0 !important;
      box-shadow: none !important;
    }
  `,
});

const slideCount = await page.locator(".pitch-slide").count();
const lastSlideNumber = Math.min(config.to, slideCount);

if (firstSlideIndex >= slideCount) {
  throw new Error(`Deck only has ${slideCount} slides`);
}

const pdf = await PDFDocument.create();

for (let index = firstSlideIndex; index < lastSlideNumber; index += 1) {
  const slideNumber = index + 1;
  const slide = page.locator(".pitch-slide").nth(index);
  await slide.scrollIntoViewIfNeeded();
  await page.evaluate(async (slideIndex) => {
    const currentSlide = document.querySelectorAll(".pitch-slide")[slideIndex];
    if (!currentSlide) throw new Error(`Slide ${slideIndex + 1} not found`);

    const images = Array.from(currentSlide.querySelectorAll("img"));
    await Promise.all(
      images.map((image) => {
        if (image.complete) return Promise.resolve();
        return new Promise((resolve) => {
          image.addEventListener("load", resolve, { once: true });
          image.addEventListener("error", resolve, { once: true });
        });
      }),
    );
  }, index);

  const screenshot = await slide.screenshot({
    animations: "disabled",
    path: path.join(
      qaDir,
      `slide-${String(slideNumber).padStart(2, "0")}@${config.scale}x.png`,
    ),
    type: "png",
  });
  const image = await pdf.embedPng(screenshot);
  const pdfPage = pdf.addPage([pdfPageWidth, pdfPageHeight]);
  pdfPage.drawImage(image, {
    height: pdfPageHeight,
    width: pdfPageWidth,
    x: 0,
    y: 0,
  });
}

const range =
  config.from === 1 && lastSlideNumber === slideCount
    ? "all"
    : `${config.from}-${lastSlideNumber}`;
const fileBaseName = `moto-reach-pitch-slides-${range}-${config.scale}x`;
const pdfPath = path.join(config.outDir, `${fileBaseName}.pdf`);

await fs.writeFile(pdfPath, await pdf.save());
await browser.close();

console.info(
  `Exported ${lastSlideNumber - config.from + 1} slides to ${pdfPath}`,
);
