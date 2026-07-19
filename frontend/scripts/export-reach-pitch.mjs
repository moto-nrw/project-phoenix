import { chromium } from "@playwright/test";
import { PDFDocument } from "pdf-lib";
import fs from "node:fs/promises";
import path from "node:path";

const defaults = {
  fileBaseName: "moto-reach-pitch-slides",
  from: 1,
  framePadding: 48,
  outDir: path.resolve(process.cwd(), "../outputs/reach-pitch-web"),
  pageHeightIn: 9,
  pageWidthIn: 16,
  scale: 4,
  selector: ".pitch-slide",
  url: "http://localhost:3001/reach-pitch",
  viewportHeight: 1080,
  viewportWidth: 1920,
};

function readOption(name) {
  const index = process.argv.indexOf(`--${name}`);
  if (index === -1) return undefined;
  return process.argv[index + 1];
}

function readFlag(name) {
  return process.argv.includes(`--${name}`);
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

function readNonNegativeNumberOption(name, defaultValue) {
  const rawValue = readOption(name);
  if (rawValue === undefined) return defaultValue;

  const value = Number.parseInt(rawValue, 10);
  if (!Number.isFinite(value) || value < 0) {
    throw new Error(`--${name} must be a non-negative integer`);
  }

  return value;
}

function readFloatOption(name, defaultValue) {
  const rawValue = readOption(name);
  if (rawValue === undefined) return defaultValue;

  const value = Number.parseFloat(rawValue);
  if (!Number.isFinite(value) || value <= 0) {
    throw new Error(`--${name} must be a positive number`);
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
  fileBaseName: readStringOption("file-base-name", defaults.fileBaseName),
  flat: readFlag("flat"),
  framePadding: readNonNegativeNumberOption(
    "frame-padding",
    defaults.framePadding,
  ),
  from: readNumberOption("from", defaults.from),
  outDir: path.resolve(readStringOption("out-dir", defaults.outDir)),
  pageHeightIn: readFloatOption("page-height-in", defaults.pageHeightIn),
  pageWidthIn: readFloatOption("page-width-in", defaults.pageWidthIn),
  scale: readNumberOption("scale", defaults.scale),
  selector: readStringOption("selector", defaults.selector),
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
const pdfPageWidth = config.pageWidthIn * 72;
const pdfPageHeight = config.pageHeightIn * 72;

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

    ${
      config.flat
        ? `
          ${config.selector} {
            border: 0 !important;
            border-radius: 0 !important;
            box-shadow: none !important;
          }
        `
        : ""
    }
  `,
});

const slideCount = await page.locator(config.selector).count();
const lastSlideNumber = Math.min(config.to, slideCount);

if (firstSlideIndex >= slideCount) {
  throw new Error(`Deck only has ${slideCount} slides`);
}

const pdf = await PDFDocument.create();

for (let index = firstSlideIndex; index < lastSlideNumber; index += 1) {
  const slideNumber = index + 1;
  const slide = page.locator(config.selector).nth(index);
  if (!config.flat) {
    await page.evaluate(({ selector, slideIndex }) => {
      const currentSlide = document.querySelectorAll(selector)[slideIndex];
      currentSlide?.scrollIntoView({ block: "center", inline: "center" });
    }, { selector: config.selector, slideIndex: index });
  }
  await slide.scrollIntoViewIfNeeded();
  await page.evaluate(async ({ selector, slideIndex }) => {
    const currentSlide = document.querySelectorAll(selector)[slideIndex];
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
  }, { selector: config.selector, slideIndex: index });

  const screenshotPath = path.join(
    qaDir,
    `slide-${String(slideNumber).padStart(2, "0")}@${config.scale}x${
      config.flat ? "-flat" : "-framed"
    }.png`,
  );
  const screenshot = config.flat
    ? await slide.screenshot({
        animations: "disabled",
        path: screenshotPath,
        type: "png",
      })
    : await page.screenshot({
        animations: "disabled",
        clip: await page.evaluate(
          ({
            framePadding,
            pdfPageHeight,
            pdfPageWidth,
            selector,
            slideIndex,
          }) => {
            const currentSlide = document.querySelectorAll(selector)[
              slideIndex
            ];
            if (!currentSlide)
              throw new Error(`Slide ${slideIndex + 1} not found`);

            const rect = currentSlide.getBoundingClientRect();
            const desiredWidth = rect.width + framePadding * 2;
            const desiredHeight =
              desiredWidth * (pdfPageHeight / pdfPageWidth);
            const width = Math.min(window.innerWidth, desiredWidth);
            const height = Math.min(window.innerHeight, desiredHeight);
            const slideCenterX = rect.left + rect.width / 2;
            const slideCenterY = rect.top + rect.height / 2;
            const x = Math.max(
              0,
              Math.min(window.innerWidth - width, slideCenterX - width / 2),
            );
            const y = Math.max(
              0,
              Math.min(window.innerHeight - height, slideCenterY - height / 2),
            );

            return { height, width, x, y };
          },
          {
            framePadding: config.framePadding,
            pdfPageHeight,
            pdfPageWidth,
            selector: config.selector,
            slideIndex: index,
          },
        ),
        fullPage: false,
        path: screenshotPath,
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
const fileBaseName = `${config.fileBaseName}-${range}-${config.scale}x${
  config.flat ? "-flat" : "-framed"
}`;
const pdfPath = path.join(config.outDir, `${fileBaseName}.pdf`);

await fs.writeFile(pdfPath, await pdf.save());
await browser.close();

console.info(
  `Exported ${lastSlideNumber - config.from + 1} slides to ${pdfPath}`,
);
