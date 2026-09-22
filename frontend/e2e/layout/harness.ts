import type { Page } from "@playwright/test";
import tailwindcss from "@tailwindcss/postcss";
import { readFile } from "node:fs/promises";
import path from "node:path";
import postcss, { type AcceptedPlugin } from "postcss";

// Gemeinsames Gerüst der Layouttests: das echte globals.css, durch Tailwind
// kompiliert, und der Rumpf von `TenantPage` in einer Shell mit fester Höhe.

const FRONTEND_DIR = process.cwd();
const GLOBALS_CSS = path.join(FRONTEND_DIR, "src", "styles", "globals.css");

export async function compileGlobalsCss(): Promise<string> {
  const source = await readFile(GLOBALS_CSS, "utf8");
  // @tailwindcss/postcss bringt ein eigenes postcss mit; der Cast umgeht den
  // Typvergleich zweier postcss-Versionen, zur Laufzeit ist das Plugin gleich.
  const plugin = tailwindcss({
    base: FRONTEND_DIR,
  }) as unknown as AcceptedPlugin;
  const result = await postcss([plugin]).process(source, { from: GLOBALS_CSS });
  return result.css;
}

/** Gerüst von `TenantPage` in einer Shell mit fester Höhe. */
export function tenantPage(body: string, shellHeight = 900): string {
  return `
    <div style="display:flex;flex-direction:column;height:${shellHeight}px" data-testid="shell">
      <div class="flex w-full flex-1 flex-col">
        <header style="height:96px">Kopfkarte</header>
        <div class="moto-tenant-body mt-6 space-y-6">
          ${body}
        </div>
      </div>
    </div>`;
}

export async function render(
  page: Page,
  css: string,
  body: string,
): Promise<void> {
  await page.setContent(
    `<!doctype html><html><head><style>${css}</style></head><body>${body}</body></html>`,
  );
}

export async function box(page: Page, testId: string) {
  const rect = await page.getByTestId(testId).boundingBox();
  if (!rect) throw new Error(`${testId} is not rendered`);
  return rect;
}
