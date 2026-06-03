import { expect, test } from "@playwright/test";
import { mkdir, stat } from "node:fs/promises";
import path from "node:path";

// Build-time PDF generation for the public /help guide pages.
//
// The HTML guide pages are the single source of truth. This script renders
// them with headless Chromium and exports clean A4 PDFs (no browser-injected
// date / URL / title / page-number header or footer — which window.print()
// cannot suppress). Output lands in public/help/pdfs/ and is served as a
// static download; it is regenerated on every CI build, never committed, so
// the PDFs can never drift from the live guide content.
//
// Run via: pnpm run generate:guides

const GUIDES = [
  { route: "/help/setup", file: "setup.pdf" },
  { route: "/help/features", file: "features.pdf" },
  { route: "/help/nfc", file: "nfc.pdf" },
] as const;

const OUT_DIR = path.join(process.cwd(), "public", "help", "pdfs");

// A correctly rendered guide PDF is always well above this; anything smaller
// means the page rendered blank or the images never loaded. Failing here (and
// retrying via the config) is preferable to silently shipping a broken
// download into the production image.
const MIN_PDF_BYTES = 20_000;

// Serial so the single render server isn't hammered by parallel page loads.
test.describe.configure({ mode: "serial" });

for (const guide of GUIDES) {
  test(`generate ${guide.file}`, async ({ page }) => {
    await mkdir(OUT_DIR, { recursive: true });
    await page.goto(guide.route, { waitUntil: "load" });

    // Next.js streams the root loading.tsx ("Laden…") fallback first and swaps
    // in the real content once the route segment resolves (a long window in
    // `next dev`, which compiles routes on demand). Wait for the guide's own
    // <h1> to be visible so we never PDF the loading placeholder. This is more
    // reliable than `networkidle` (which Playwright discourages and which can
    // hang on analytics/SSE) or `domcontentloaded` (which fires on the fallback).
    await page
      .locator("main h1")
      .first()
      .waitFor({ state: "visible", timeout: 30_000 });

    // Screenshots use loading="lazy"; force eager load and wait for every image
    // to fully decode so none are missing/half-painted in the exported PDF.
    // decode() resolves once the image is paintable and rejects on a broken
    // source — swallow the rejection so one bad asset can't hang the render.
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
    await page.pdf({
      path: outPath,
      format: "A4",
      printBackground: true,
      displayHeaderFooter: false,
      margin: { top: "18mm", right: "16mm", bottom: "18mm", left: "16mm" },
    });

    // Sanity-check the artifact actually rendered before it ships in the image.
    const { size } = await stat(outPath);
    expect(
      size,
      `${guide.file} is only ${size} bytes — render likely failed`,
    ).toBeGreaterThan(MIN_PDF_BYTES);
  });
}
