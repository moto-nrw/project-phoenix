import { expect, test } from "@playwright/test";
import { mkdir, readFile, stat, unlink, writeFile } from "node:fs/promises";
import path from "node:path";
import { PDFDocument, rgb } from "pdf-lib";

// Build-time PDF generation for the public /help guide pages.
//
// The HTML guide pages are the single source of truth. This script renders
// them with headless Chromium and exports clean A4 PDFs. Output lands in
// public/help/pdfs/ and is served as a static download.
//
// Run via: pnpm run generate:guides

const GUIDES = [
  { route: "/help/setup", file: "setup.pdf" },
  { route: "/help/features", file: "features.pdf" },
  { route: "/help/nfc", file: "nfc.pdf" },
  {
    route: "/help/nfc/erste-schritte",
    file: "nfc-erste-schritte.pdf",
    onePager: true,
  },
] as const;

const OUT_DIR = path.join(process.cwd(), "public", "help", "pdfs");

// A correctly rendered guide PDF is always well above this; anything smaller
// means the page rendered blank or the images never loaded. Failing here (and
// retrying via the config) is preferable to silently shipping a broken
// download into the production image.
const MIN_PDF_BYTES = 20_000;
const PDF_MARGIN = { top: "18mm", right: "16mm", bottom: "18mm", left: "16mm" };
const ONE_PAGER_MARGIN = { top: "0", right: "0", bottom: "0", left: "0" };
const DOT_SPACING_PT = 10.5;
const DOT_RADIUS_PT = 0.75;
const DOT_OFFSET_PT = 0.75;

async function applyGuidePdfBackground(rawPath: string, outPath: string) {
  const rawBytes = await readFile(rawPath);
  const source = await PDFDocument.load(rawBytes);
  const target = await PDFDocument.create();
  const pageCount = source.getPageCount();

  for (let index = 0; index < pageCount; index += 1) {
    const sourcePage = source.getPage(index);
    const { width, height } = sourcePage.getSize();
    const targetPage = target.addPage([width, height]);

    targetPage.drawRectangle({
      x: 0,
      y: 0,
      width,
      height,
      color: rgb(249 / 255, 250 / 255, 251 / 255),
    });

    for (let x = DOT_OFFSET_PT; x < width; x += DOT_SPACING_PT) {
      for (let y = DOT_OFFSET_PT; y < height; y += DOT_SPACING_PT) {
        targetPage.drawCircle({
          x,
          y,
          size: DOT_RADIUS_PT,
          color: rgb(17 / 255, 24 / 255, 39 / 255),
          opacity: 0.12,
        });
      }
    }

    const [embeddedPage] = await target.embedPdf(rawBytes, [index]);
    if (!embeddedPage) {
      throw new Error(`Could not embed page ${index + 1} from ${rawPath}`);
    }
    targetPage.drawPage(embeddedPage);
  }

  await writeFile(outPath, await target.save());
}

// Serial so the single render server isn't hammered by parallel page loads.
test.describe.configure({ mode: "serial" });

for (const guide of GUIDES) {
  test(`generate ${guide.file}`, async ({ page }) => {
    const onePager = "onePager" in guide && guide.onePager;
    await mkdir(OUT_DIR, { recursive: true });
    await page.goto(guide.route, { waitUntil: "load" });

    // Wait for the guide title so we never PDF the loading placeholder.
    await page
      .locator("main h1:visible")
      .first()
      .waitFor({ state: "visible", timeout: 30_000 });

    // Force screenshots to decode before the PDF render starts.
    await page.evaluate(async () => {
      const images = Array.from(document.querySelectorAll("img"));
      images.forEach((img) => {
        img.loading = "eager";
      });
      await Promise.all(
        images.map((img) => img.decode().catch(() => undefined)),
      );
    });

    const outPath = path.join(OUT_DIR, guide.file);
    const rawPath = path.join(OUT_DIR, `${guide.file}.raw`);
    await page.addStyleTag({
      content: `
        @media print {
          html,
          body,
          body > div.min-h-screen.bg-white,
          body > div.min-h-screen.bg-white > div.relative,
          .moto-dotted-background--guide {
            background: transparent !important;
            background-image: none !important;
          }

          body::before,
          .moto-dotted-background--guide::before {
            display: none !important;
          }
        }
      `,
    });
    const pdfOptions = {
      path: rawPath,
      format: "A4",
      printBackground: true,
      displayHeaderFooter: false,
      margin: onePager ? ONE_PAGER_MARGIN : PDF_MARGIN,
      scale: 1,
      omitBackground: true,
    } as Parameters<typeof page.pdf>[0] & { omitBackground: boolean };
    await page.pdf(pdfOptions);
    await applyGuidePdfBackground(rawPath, outPath);
    await unlink(rawPath);

    // Sanity-check the artifact actually rendered before it ships in the image.
    const { size } = await stat(outPath);
    expect(
      size,
      `${guide.file} is only ${size} bytes , render likely failed`,
    ).toBeGreaterThan(MIN_PDF_BYTES);
  });
}
