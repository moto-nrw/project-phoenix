import type { Page } from "@playwright/test";
import { loginToLocalTenant } from "../../e2e/local-tenant-login";
import {
  loadSeedAccess,
  readSeedAccess,
  type SeedAccess,
  type SeedRole,
} from "../seed-state";

/** Einzelne Aktionen sollen vor dem Gesamtbudget mit ihrem Call-Log scheitern. */
export const PERF_ACTION_TIMEOUT = 15_000;

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
 * Zugang eines Kontos aus dem Seed-Profil. `admin` folgt loadAccess (mit den
 * E2E-Überschreibungen), jede andere Rolle kommt nur aus der Seed-Datei.
 */
export function loadRoleAccess(role: SeedRole): SeedAccess {
  if (role === "admin") return loadAccess();
  const access = readSeedAccess({ role });
  if (!access) {
    throw new Error(`Perf harness needs backend/.seed-state.json for ${role}.`);
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
    const reason = error instanceof Error ? error.message : "";
    if (!/net::ERR_ABORTED/.test(reason)) throw error;
  }
}

/**
 * Login über die Tenant-Wurzel, inklusive des MFA-Schritts mit dem Code aus
 * Mailpit (`MAILPIT_URL`). Ohne ihn bleibt jeder Schul-Login vor dem
 * Code-Feld stehen. `loginToLocalTenant` wartet nie auf `load`/`networkidle`:
 * SSE hält Verbindungen dauerhaft offen.
 */
export async function login(page: Page, access: SeedAccess): Promise<void> {
  await loginToLocalTenant(page, tenantBaseUrl(access), access);
}

/** Den Browser-Hinweis vor der Messung schließen; storageState merkt die Wahl. */
export async function dismissNotificationHint(
  page: Page,
  baseUrl: string,
): Promise<void> {
  await gotoTolerant(page, `${baseUrl}/home`);
  const dismiss = page.getByRole("button", { name: "Verstanden" });
  await dismiss.waitFor({ timeout: 10_000 }).catch(() => undefined);
  if (await dismiss.isVisible()) await dismiss.click();
}
