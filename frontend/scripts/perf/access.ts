import type { Page } from "@playwright/test";
import { loadSeedAccess, type SeedAccess } from "../seed-state";

/**
 * Tenant-Slug und Admin-Login aus backend/.seed-state.json; E2E_TENANT_SLUG /
 * E2E_TEST_EMAIL / E2E_TEST_PASSWORD überschreiben. Gleiches Muster wie die
 * e2e-Specs (frontend/e2e/betreuungsplan-flow.spec.ts).
 */
export function loadAccess(profile?: string): SeedAccess {
  const access = loadSeedAccess(profile ? { profile } : {});
  if (!access) {
    throw new Error(
      "Perf harness needs backend/.seed-state.json or E2E_TENANT_SLUG/E2E_TEST_EMAIL/E2E_TEST_PASSWORD.",
    );
  }
  return access;
}

/**
 * Port des gemessenen Next-Servers. Er muss explizit gesetzt sein, damit der
 * Harness nicht unbemerkt einen Server aus einem anderen Worktree misst.
 */
export function perfPort(): string {
  const port = process.env.PERF_PORT;
  if (!port || !/^\d+$/.test(port)) {
    throw new Error(
      "PERF_PORT must be an integer between 1 and 65535 for the performance harness.",
    );
  }
  const portNumber = Number(port);
  if (portNumber < 1 || portNumber > 65_535) {
    throw new Error(
      "PERF_PORT must be an integer between 1 and 65535 for the performance harness.",
    );
  }

  return port;
}

export function tenantBaseUrl(access: SeedAccess): string {
  return `http://${access.slug}.localhost:${perfPort()}`;
}

/**
 * `page.goto` mit `domcontentloaded`; ein `net::ERR_ABORTED` wird geschluckt.
 * Es tritt auf, wenn die Seite die Dokument-Navigation selbst ersetzt (etwa
 * `useSession({ required: true })` mit Client-Redirect, bevor das Dokument
 * fertig ist). Was danach sichtbar ist, prüfen die Aufrufer.
 */
export async function gotoTolerant(page: Page, url: string): Promise<void> {
  try {
    await page.goto(url, { waitUntil: "domcontentloaded" });
  } catch (error) {
    if (!(error instanceof Error && error.message.includes("ERR_ABORTED"))) {
      throw error;
    }
  }
}

/**
 * Login über die Tenant-Wurzel. `domcontentloaded`, nie `load`/`networkidle`:
 * SSE hält Verbindungen dauerhaft offen.
 */
export async function login(page: Page, access: SeedAccess): Promise<void> {
  await page.goto(`${tenantBaseUrl(access)}/`, {
    waitUntil: "domcontentloaded",
  });
  await page.waitForSelector('input[type="email"]', { timeout: 30_000 });
  await page.fill('input[type="email"]', access.email);
  await page.fill('input[type="password"]', access.password);
  await page.click('button:has-text("Anmelden")');
  await page.waitForSelector('input[type="email"]', {
    state: "detached",
    timeout: 30_000,
  });
}
