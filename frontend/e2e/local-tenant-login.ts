import { expect, type BrowserContext, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import {
  mkdirSync,
  readFileSync,
  renameSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

interface Credentials {
  readonly email: string;
  readonly password: string;
}

type SessionCookies = Awaited<ReturnType<BrowserContext["cookies"]>>;

function readSession(path: string): SessionCookies | undefined {
  try {
    return JSON.parse(readFileSync(path, "utf8")) as SessionCookies;
  } catch (error) {
    if (
      error instanceof SyntaxError ||
      (error instanceof Error && "code" in error && error.code === "ENOENT")
    )
      return undefined;
    throw error;
  }
}

function saveSession(
  directory: string,
  path: string,
  cookies: SessionCookies,
): void {
  mkdirSync(directory, { recursive: true, mode: 0o700 });
  const temporary = `${path}.${process.pid}.tmp`;
  writeFileSync(temporary, JSON.stringify(cookies), { mode: 0o600 });
  renameSync(temporary, path);
}

async function hasSession(page: Page, origin: string): Promise<boolean> {
  const response = await page.request.get(`${origin}/api/auth/session`);
  if (!response.ok())
    throw new Error("Test-Sitzung konnte nicht geladen werden.");
  // Auth.js answers `null` without a session, for example when a cached
  // cookie belongs to another local database.
  const session = (await response.json()) as {
    user?: { id?: string };
    error?: string;
  } | null;
  return !!session?.user?.id && !session.error;
}

function localURL(value: string): URL {
  const url = new URL(value);
  if (
    url.protocol !== "http:" ||
    (url.hostname !== "localhost" && !url.hostname.endsWith(".localhost")) ||
    url.username ||
    url.password
  ) {
    throw new Error("Der Test-Login erlaubt nur lokale HTTP-Adressen.");
  }
  return url;
}

/** Read only this login's mail, never a previous code or another account's. */
export async function localMFACode(
  recipient: string,
  notBefore: number,
  mailpitURL = process.env.MAILPIT_URL,
): Promise<string> {
  if (!mailpitURL) throw new Error("Für den Test-Login fehlt MAILPIT_URL.");
  const base = localURL(mailpitURL);
  let code: string | undefined;
  await expect(async () => {
    const list = await fetch(new URL("/api/v1/messages?limit=50", base), {
      redirect: "error",
      signal: AbortSignal.timeout(5_000),
    });
    if (!list.ok) throw new Error(`Mailpit antwortet mit ${list.status}.`);
    const data = (await list.json()) as {
      messages: Array<{
        ID: string;
        Created: string;
        To: Array<{ Address: string }>;
      }>;
    };
    const messages = data.messages
      .filter(
        (message) =>
          Date.parse(message.Created) >= notBefore &&
          message.To.some(
            ({ Address }) => Address.toLowerCase() === recipient.toLowerCase(),
          ),
      )
      .sort((a, b) => Date.parse(b.Created) - Date.parse(a.Created));
    for (const message of messages) {
      const response = await fetch(
        new URL(`/api/v1/message/${encodeURIComponent(message.ID)}`, base),
        { redirect: "error", signal: AbortSignal.timeout(5_000) },
      );
      if (!response.ok) continue;
      const body = (await response.json()) as { Text?: string; HTML?: string };
      code = /\b\d{6}\b/.exec(body.Text ?? body.HTML ?? "")?.[0];
      if (code) return;
    }
    throw new Error("Der aktuelle MFA-Code ist noch nicht eingetroffen.");
  }).toPass({ timeout: 30_000, intervals: [250, 500] });
  return code!;
}

/** Complete the real UI MFA flow before a caller saves browser storage. */
export async function loginToLocalTenant(
  page: Page,
  origin: string,
  credentials: Credentials,
  cacheDirectory = join(
    tmpdir(),
    `moto-playwright-sessions-${process.getuid?.() ?? "local"}`,
  ),
): Promise<void> {
  localURL(origin);
  // Playwright replaces the entire worker after a failing test. Keep only
  // completed local sessions outside reports/artifacts, just like devtools.
  // A miss or invalid session still completes the regular UI MFA flow below.
  const key = createHash("sha256")
    .update(JSON.stringify([origin, credentials.email, credentials.password]))
    .digest("hex");
  const path = join(cacheDirectory, `${key}.json`);
  if (await restoreSession(page, origin, cacheDirectory, path)) return;
  // Parallel workers share one account, and the backend sends at most three
  // MFA codes per account in fifteen minutes. On a cold cache one worker logs
  // in while the others wait and then reuse its session.
  mkdirSync(cacheDirectory, { recursive: true, mode: 0o700 });
  await withLoginLock(`${path}.lock`, async () => {
    if (await restoreSession(page, origin, cacheDirectory, path)) return;
    await loginThroughUI(page, origin, credentials);
    saveSession(cacheDirectory, path, await page.context().cookies(origin));
  });
}

async function restoreSession(
  page: Page,
  origin: string,
  cacheDirectory: string,
  path: string,
): Promise<boolean> {
  const cached = readSession(path);
  if (!cached) return false;
  await page.context().addCookies(cached);
  if (!(await hasSession(page, origin))) return false;
  await page.goto(`${origin}/`, { waitUntil: "domcontentloaded" });
  saveSession(cacheDirectory, path, await page.context().cookies(origin));
  return true;
}

async function withLoginLock(
  lock: string,
  run: () => Promise<void>,
): Promise<void> {
  for (;;) {
    try {
      mkdirSync(lock);
      break;
    } catch (error) {
      if (!(
        error instanceof Error &&
        "code" in error &&
        error.code === "EEXIST"
      ))
        throw error;
    }
    let age: number;
    try {
      age = Date.now() - statSync(lock).mtimeMs;
    } catch {
      continue;
    }
    // A worker torn down mid-login leaves its lock; a login takes seconds.
    if (age > 60_000) rmSync(lock, { recursive: true, force: true });
    else await new Promise((resolve) => setTimeout(resolve, 250));
  }
  try {
    await run();
  } finally {
    rmSync(lock, { recursive: true, force: true });
  }
}

async function loginThroughUI(
  page: Page,
  origin: string,
  credentials: Credentials,
): Promise<void> {
  await page.goto(`${origin}/`, { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="email"]', { timeout: 30_000 });
  await page.fill('input[type="email"]', credentials.email);
  await page.fill('input[type="password"]', credentials.password);
  const notBefore = Date.now();
  const pending = page.waitForResponse(
    (response) =>
      new URL(response.url()).origin === origin &&
      new URL(response.url()).pathname === "/api/auth/login" &&
      response.request().method() === "POST",
  );
  const [response] = await Promise.all([
    pending,
    page.getByRole("button", { name: "Anmelden", exact: true }).click(),
  ]);
  if (!response.ok())
    throw new Error(`Test-Login fehlgeschlagen (${response.status()}).`);
  const result = (await response.json()) as { status?: string };
  const enrollment = result.status === "mfa_enrollment_required";
  if (enrollment) {
    await page
      .getByRole("button", { name: "Code an meine E-Mail senden" })
      .click();
  }
  if (enrollment || result.status === "mfa_required") {
    const input = page.locator('input[autocomplete="one-time-code"]').first();
    await expect(input).toBeVisible();
    const code = await localMFACode(credentials.email, notBefore);
    await input.fill(code);
    if (enrollment) {
      await page.getByRole("button", { name: "Weiter zum Dashboard" }).click();
    }
  }
  // The email field disappears at the MFA step too. Only a real session proves
  // that the login finished, including the asynchronous NextAuth exchange.
  await expect(async () => {
    if (!(await hasSession(page, origin))) {
      throw new Error("Test-Login hat noch keine vollständige Sitzung.");
    }
  }).toPass({ timeout: 30_000, intervals: [250, 500] });
}
