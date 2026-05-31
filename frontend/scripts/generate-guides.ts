import { test } from "@playwright/test";
import { mkdir } from "node:fs/promises";
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

// Serial so a single dev server isn't hammered by parallel page loads.
test.describe.configure({ mode: "serial" });

for (const guide of GUIDES) {
  test(`generate ${guide.file}`, async ({ page }) => {
    await mkdir(OUT_DIR, { recursive: true });
    await page.goto(guide.route, { waitUntil: "networkidle" });

    // Screenshots use loading="lazy"; force eager load and wait for every
    // image to settle so none are missing in the exported PDF.
    await page.evaluate(
      () =>
        new Promise<void>((resolve) => {
          const images = Array.from(document.querySelectorAll("img"));
          images.forEach((img) => {
            img.loading = "eager";
          });
          void Promise.all(
            images.map((img) =>
              img.complete
                ? Promise.resolve()
                : new Promise<void>((res) => {
                    img.addEventListener("load", () => res());
                    img.addEventListener("error", () => res());
                  }),
            ),
          ).then(() => resolve());
        }),
    );

    await page.pdf({
      path: path.join(OUT_DIR, guide.file),
      format: "A4",
      printBackground: true,
      displayHeaderFooter: false,
      margin: { top: "18mm", right: "16mm", bottom: "18mm", left: "16mm" },
    });
  });
}
