import {
  test,
  expect,
  type BrowserContext,
  type Page,
  type Route,
} from "@playwright/test";
import { randomBytes } from "node:crypto";

// #5194: after a logout or a "Zur Schulanmeldung" in one tab, a response of
// another tab that arrives late with the old session cookie must not restore
// access. The synthetic backend models the session family: logout revokes
// it, and its session routes refuse a token of a revoked family although the
// token has not expired. No real backend, keys or personal data.
const source = "http://school-a.mfa.localhost:13845";
const target = "http://school-b.mfa.localhost:13845";
const cookieName = "mfa-localhost.session-token";
const fixtureState = "http://127.0.0.1:18445/__fixture/switch-state";

async function isolate(context: BrowserContext) {
  await context.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    if (
      !url.hostname.endsWith(".localhost") ||
      url.pathname === "/monitoring" ||
      url.pathname.startsWith("/ingest")
    ) {
      await route.abort();
      return;
    }
    await route.continue();
  });
  await context.addInitScript(() => {
    for (const slug of ["school-a", "school-b"])
      localStorage.setItem(
        `moto.tenant.notification-setup.v1.${slug}.1`,
        JSON.stringify({ done: true }),
      );
  });
}

async function login(page: Page) {
  await page.goto(source);
  await page.locator('input[type="email"]').fill("fixture@example.invalid");
  await page
    .locator('input[type="password"]')
    .fill(randomBytes(24).toString("hex"));
  await page.getByRole("button", { name: "Anmelden", exact: true }).click();
  await page.waitForURL(`${source}/home`);
}

/** Only cookie metadata and its presence; never its value in evidence. */
async function sessionCookie(context: BrowserContext) {
  return (await context.cookies(source))
    .map(({ name, domain, httpOnly }) => ({ name, domain, httpOnly }))
    .find((cookie) => cookie.name === cookieName);
}

/** A concurrent UI answer may rewrite the jar after a valid deletion. */
async function waitForSessionCookieDeletion(page: Page) {
  const response = await page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/auth/signout",
  );
  return (
    response.status() === 200 &&
    (await response.headersArray()).some(({ name, value }) => {
      if (name.toLowerCase() !== "set-cookie") return false;
      const parts = value
        .toLowerCase()
        .split(";")
        .map((part) => part.trim());
      return (
        parts[0] === `${cookieName}=` &&
        parts.includes("max-age=0") &&
        parts.includes("path=/") &&
        parts.some((part) => /^domain=\.?mfa\.localhost$/.test(part))
      );
    })
  );
}

/**
 * Starts a request in the second tab and holds its response, including the
 * Set-Cookie of the still-live session, until release() is called.
 */
async function holdResponse(
  page: Page,
  pattern: string,
  start: (page: Page) => Promise<unknown>,
) {
  let release!: () => void;
  const released = new Promise<void>((resolve) => (release = resolve));
  let heldCookieValue: string | undefined;
  let heldResponse!: (value: { setCookie: boolean }) => void;
  const held = new Promise<{ setCookie: boolean }>(
    (resolve) => (heldResponse = resolve),
  );
  await page.route(pattern, async (route: Route) => {
    // The fixture host names resolve only in Chromium (CI disables them for
    // Node), so the held request goes to loopback under its own Host.
    const request = route.request();
    const target = new URL(request.url());
    const host = target.host;
    target.hostname = "127.0.0.1";
    const response = await route.fetch({
      url: target.toString(),
      headers: { ...(await request.allHeaders()), host },
    });
    const setCookie = response
      .headersArray()
      .filter(
        (header) =>
          header.name.toLowerCase() === "set-cookie" &&
          header.value.startsWith(`${cookieName}=`) &&
          !header.value.startsWith(`${cookieName}=;`),
      )
      .at(-1);
    heldCookieValue = setCookie?.value
      .split(";")[0]
      ?.slice(cookieName.length + 1);
    heldResponse({ setCookie: setCookie !== undefined });
    await released;
    await route.fulfill({ response });
  });
  const finished = start(page).catch(() => undefined);
  return {
    held,
    restored: async (context: BrowserContext) =>
      heldCookieValue !== undefined &&
      (await context.cookies(source)).some(
        (cookie) =>
          cookie.name === cookieName && cookie.value === heldCookieValue,
      ),
    release: async () => {
      release();
      await finished;
    },
  };
}

const lateSources = {
  "BFF GET": {
    pattern: "**/api/me/profile",
    start: (page: Page) =>
      page.evaluate(() => fetch("/api/me/profile").then((r) => r.status)),
  },
  "session GET": {
    pattern: "**/api/auth/session",
    start: (page: Page) =>
      page.evaluate(() => fetch("/api/auth/session").then((r) => r.status)),
  },
  "session update() POST": {
    pattern: "**/api/auth/session",
    start: (page: Page) =>
      page.evaluate(async () => {
        const csrf = (await (await fetch("/api/auth/csrf")).json()) as {
          csrfToken: string;
        };
        const response = await fetch("/api/auth/session", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ csrfToken: csrf.csrfToken, data: {} }),
        });
        return response.status;
      }),
  },
} as const;

const endActions = {
  logout: async (page: Page) => {
    await page
      .getByRole("button", { name: /Profilmenü|Profil/ })
      .first()
      .click();
    await page.getByRole("button", { name: "Abmelden" }).first().click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Abmelden" })
      .click();
    await page.waitForURL((url) => url.pathname === "/");
  },
  "Zur Schulanmeldung": async (page: Page) => {
    await page.goto(`${target}/home`);
    await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
    await page.waitForURL(`${target}/`);
  },
} as const;

for (const [actionName, endSession] of Object.entries(endActions)) {
  for (const [sourceName, late] of Object.entries(lateSources)) {
    test(`${actionName}: a late ${sourceName} Set-Cookie restores no access`, async ({
      browser,
      request,
    }) => {
      const context = await browser.newContext();
      await isolate(context);
      const tab1 = await context.newPage();
      await login(tab1);
      const tab2 = await context.newPage();
      await tab2.goto(`${source}/api/health`);
      const before = await (await request.get(fixtureState)).json();

      // Tab 2's request still runs on the live session; its answer, with the
      // session cookie, is held back.
      const pending = await holdResponse(tab2, late.pattern, late.start);
      expect((await pending.held).setCookie).toBe(true);

      const [deleted] = await Promise.all([
        waitForSessionCookieDeletion(tab1),
        endSession(tab1),
      ]);
      expect(deleted).toBe(true);
      const afterEnd = await (await request.get(fixtureState)).json();
      expect(afterEnd.logoutRequests - before.logoutRequests).toBe(1);
      expect(afterEnd.liveFamilies).toBe(before.liveFamilies - 1);
      // Stop other UI answers from obscuring which cookie the held response
      // installs. Revocation already committed; concurrent copies are allowed
      // to exist, but must grant no access (#5194).
      await tab1.close();

      // The held answer arrives late and writes the old cookie back.
      await pending.release();
      expect(await sessionCookie(context)).toBeDefined();
      expect(await pending.restored(context)).toBe(true);

      // Protected data stays locked although the access token is unexpired.
      const tab3 = await context.newPage();
      await tab3.goto(`${source}/api/health`);
      const status = await tab3.evaluate(() =>
        fetch("/api/me/profile").then((r) => r.status),
      );
      expect(status).toBe(401);
      // The session the browser sees ends too. Other tabs may still poll the
      // session with the written-back cookie, so its state converges.
      await expect
        .poll(() =>
          tab3.evaluate(async () => {
            const body = (await (await fetch("/api/auth/session")).json()) as {
              error?: string;
            } | null;
            return body?.error ?? null;
          }),
        )
        .toBe("RefreshTokenError");

      // A fresh page load of the school asks for the login again.
      await tab3.goto(`${source}/home`);
      await tab3.waitForURL((url) => url.pathname === "/");
      await expect(tab3.locator('input[type="email"]')).toBeVisible();
      await context.close();
    });
  }
}

test("another device keeps its session when one device logs out", async ({
  browser,
}) => {
  const device1 = await browser.newContext();
  const device2 = await browser.newContext();
  await isolate(device1);
  await isolate(device2);
  const page1 = await device1.newPage();
  const page2 = await device2.newPage();
  await login(page1);
  await login(page2);

  await endActions.logout(page1);

  const status = await page2.evaluate(() =>
    fetch("/api/me/profile").then((r) => r.status),
  );
  expect(status).toBe(200);
  await device1.close();
  await device2.close();
});

test("a failed revocation keeps the session and shows the error with a retry", async ({
  browser,
  request,
}) => {
  const context = await browser.newContext();
  await isolate(context);
  const page = await context.newPage();
  await login(page);
  await page.route("**/api/auth/logout", (route) =>
    route.fulfill({
      status: 503,
      json: { status: 503, code: "general.unavailable" },
    }),
  );
  const before = await (await request.get(fixtureState)).json();

  await page
    .getByRole("button", { name: /Profilmenü|Profil/ })
    .first()
    .click();
  await page.getByRole("button", { name: "Abmelden" }).first().click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Abmelden" })
    .click();

  const alert = page.getByRole("alert").filter({ hasText: "die Abmeldung" });
  await expect(alert).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  if (process.env.LATE_COOKIE_SHOT_DIR) {
    await page.screenshot({
      path: `${process.env.LATE_COOKIE_SHOT_DIR}/logout-failed.png`,
    });
  }
  expect(page.url()).toBe(`${source}/home`);
  expect(await sessionCookie(context)).toBeDefined();
  const after = await (await request.get(fixtureState)).json();
  expect(after.liveFamilies).toBe(before.liveFamilies);

  await page.unroute("**/api/auth/logout");
  const [deleted] = await Promise.all([
    waitForSessionCookieDeletion(page),
    alert.getByRole("button", { name: "Wiederholen" }).click(),
  ]);
  await page.waitForURL((url) => url.pathname === "/");
  expect(deleted).toBe(true);
  const afterRetry = await (await request.get(fixtureState)).json();
  expect(afterRetry.logoutRequests - before.logoutRequests).toBe(1);
  expect(afterRetry.liveFamilies).toBe(before.liveFamilies - 1);
  await context.close();
});
