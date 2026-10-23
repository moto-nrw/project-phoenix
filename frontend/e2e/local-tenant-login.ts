import { expect, type BrowserContext, type Page } from "@playwright/test";

interface Credentials {
  readonly email: string;
  readonly password: string;
}

const sessions = new Map<
  string,
  Awaited<ReturnType<BrowserContext["cookies"]>>
>();

async function hasSession(page: Page, origin: string): Promise<boolean> {
  const response = await page.request.get(`${origin}/api/auth/session`);
  if (!response.ok())
    throw new Error("Test-Sitzung konnte nicht geladen werden.");
  const session = (await response.json()) as {
    user?: { id?: string };
    error?: string;
  };
  return !!session.user?.id && !session.error;
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
): Promise<void> {
  localURL(origin);
  const key = JSON.stringify([origin, credentials.email, credentials.password]);
  const cached = sessions.get(key);
  if (cached) {
    await page.context().addCookies(cached);
    if (await hasSession(page, origin)) {
      await page.goto(`${origin}/`, { waitUntil: "domcontentloaded" });
      return;
    }
    sessions.delete(key);
  }
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
  sessions.set(key, await page.context().cookies(origin));
}
