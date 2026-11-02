import {
  test,
  expect,
  type BrowserContext,
  type Page,
  type Route,
} from "@playwright/test";
import { randomBytes } from "node:crypto";
import { networkPorts } from "./ports.mjs";

// #5194, #5399: after a logout or a "Zur Schulanmeldung" in one tab, a
// response of another tab that arrives late must neither write the old
// session cookie back nor restore access. The server is the source of truth
// (#5399): a response writes a session cookie only for a changed session of
// a live refresh-token family, and the session route reads a revoked family
// as ended. The synthetic backend models the session family: logout revokes
// it, and its session routes and session validation refuse a token of a
// revoked family although the token has not expired. No real backend, keys or
// personal data.
const ports = networkPorts("tenantSwitch");
const source = `http://school-a.mfa.localhost:${ports.frontend}`;
const target = `http://school-b.mfa.localhost:${ports.frontend}`;
const cookieName = "mfa-localhost.session-token";
const fixtureState = `http://127.0.0.1:${ports.api}/__fixture/switch-state`;

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
  method?: string,
) {
  let release!: () => void;
  const released = new Promise<void>((resolve) => (release = resolve));
  let heldCookieValue: string | undefined;
  let heldResponse!: (value: { setCookie: boolean }) => void;
  const held = new Promise<{ setCookie: boolean }>(
    (resolve) => (heldResponse = resolve),
  );
  let holding = false;
  await page.route(pattern, async (route: Route) => {
    // The fixture host names resolve only in Chromium (CI disables them for
    // Node), so the held request goes to loopback under its own Host.
    const request = route.request();
    // An app tab sends more than the held request; hold only the first match.
    if (holding || (method && request.method() !== method)) {
      await route.fallback();
      return;
    }
    holding = true;
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
  "push DELETE": {
    pattern: "**/api/notifications/push/subscriptions?**",
    start: (page: Page) =>
      page.evaluate(() =>
        fetch(
          `/api/notifications/push/subscriptions?endpoint=${encodeURIComponent("https://push.example.invalid/device")}`,
          { method: "DELETE" },
        ).then((r) => r.status),
      ),
  },
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

      // Tab 2's request still runs on the live session; its answer is held
      // back. A mere re-read of the session carries no cookie (#5399): before,
      // Auth.js wrote the session again with every answer.
      const pending = await holdResponse(tab2, late.pattern, late.start);
      expect((await pending.held).setCookie).toBe(false);

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

      // The held answer arrives late and leaves the deleted cookie deleted.
      await pending.release();
      await expectSessionStaysEnded(context);
    });
  }
}

/** The deleted session stays deleted, grants nothing and is not shown. */
async function expectSessionStaysEnded(context: BrowserContext) {
  expect(await sessionCookie(context)).toBeUndefined();

  const tab3 = await context.newPage();
  await tab3.goto(`${source}/api/health`);
  const status = await tab3.evaluate(() =>
    fetch("/api/me/profile").then((r) => r.status),
  );
  expect(status).toBe(401);
  const session = await tab3.evaluate(async () =>
    (await fetch("/api/auth/session")).json(),
  );
  expect(session).toBeNull();

  // A fresh page load of the school asks for the login again.
  await tab3.goto(`${source}/home`);
  await tab3.waitForURL((url) => url.pathname === "/");
  await expect(tab3.locator('input[type="email"]')).toBeVisible();
  expect(await sessionCookie(context)).toBeUndefined();
  await context.close();
}

/** A renewing update(): it moves the access token into the refresh window. */
const renewingUpdate = (page: Page) =>
  page.evaluate(async () => {
    const csrf = (await (await fetch("/api/auth/csrf")).json()) as {
      csrfToken: string;
    };
    const response = await fetch("/api/auth/session", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        csrfToken: csrf.csrfToken,
        data: { emailChanged: true },
      }),
    });
    return response.status;
  });

/**
 * Holds a request on its way to the server until release(): it carries the
 * old cookie, and the server builds its answer only after the logout. This is
 * any late answer whose request outlived its logout, from another tab, a
 * frozen one or one on another school's subdomain.
 */
async function holdRequest(
  page: Page,
  pattern: string,
  start: (page: Page) => Promise<unknown>,
  method: string,
) {
  let release!: () => void;
  const released = new Promise<void>((resolve) => (release = resolve));
  let sent!: () => void;
  const intercepted = new Promise<void>((resolve) => (sent = resolve));
  let answered!: (setCookie: boolean) => void;
  const answer = new Promise<boolean>((resolve) => (answered = resolve));
  let holding = false;
  await page.route(pattern, async (route: Route) => {
    const request = route.request();
    if (holding || request.method() !== method) {
      await route.fallback();
      return;
    }
    holding = true;
    sent();
    await released;
    const target = new URL(request.url());
    const host = target.host;
    target.hostname = "127.0.0.1";
    const response = await route.fetch({
      url: target.toString(),
      headers: { ...(await request.allHeaders()), host },
    });
    answered(
      response
        .headersArray()
        .some(
          (header) =>
            header.name.toLowerCase() === "set-cookie" &&
            header.value.startsWith(`${cookieName}=`),
        ),
    );
    await route.fulfill({ response });
  });
  const finished = start(page).catch(() => undefined);
  return {
    intercepted,
    release: async () => {
      release();
      const setCookie = await answer;
      await finished;
      return setCookie;
    },
  };
}

for (const [actionName, endSession] of Object.entries(endActions)) {
  // Gap A of the final review: the answer is built after the revocation.
  test(`${actionName}: a renewal that reaches the server after the logout writes no cookie`, async ({
    browser,
  }) => {
    const context = await browser.newContext();
    await isolate(context);
    const tab1 = await context.newPage();
    await login(tab1);
    const tab2 = await context.newPage();
    await tab2.goto(`${source}/api/health`);
    const pending = await holdRequest(
      tab2,
      "**/api/auth/session",
      renewingUpdate,
      "POST",
    );
    await pending.intercepted;

    const [deleted] = await Promise.all([
      waitForSessionCookieDeletion(tab1),
      endSession(tab1),
    ]);
    expect(deleted).toBe(true);
    await tab1.close();

    // The renewal fails on the revoked family; its answer sets no cookie.
    expect(await pending.release()).toBe(false);
    await expectSessionStaysEnded(context);
  });

  // Gap B of the final review: a tab started while the logout runs. It also
  // runs on the other school's subdomain, which shares the cookie (#5402).
  test(`${actionName}: a tab opened during the logout reads the session as ended`, async ({
    browser,
  }) => {
    const context = await browser.newContext();
    await isolate(context);
    const tab1 = await context.newPage();
    await login(tab1);
    let releaseCsrf!: () => void;
    const csrfHeld = new Promise<void>((resolve) => (releaseCsrf = resolve));
    await tab1.route("**/api/auth/csrf", async (route) => {
      await csrfHeld;
      await route.continue();
    });
    const revoked = tab1.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/auth/logout" &&
        response.status() === 204,
    );
    const ending = Promise.all([
      waitForSessionCookieDeletion(tab1),
      endSession(tab1),
    ]);
    await revoked;
    // The revocation committed; the cookie is still there (CSRF stalls).
    expect(await sessionCookie(context)).toBeDefined();

    const tab3 = await context.newPage();
    const otherSchool = actionName === "logout" ? target : source;
    await tab3.goto(`${otherSchool}/api/health`);
    const [read, body] = await Promise.all([
      tab3.waitForResponse(
        (response) => new URL(response.url()).pathname === "/api/auth/session",
      ),
      tab3.evaluate(async () => (await fetch("/api/auth/session")).json()),
    ]);
    expect((body as { error?: string }).error).toBe("RefreshTokenError");
    expect(
      (await read.headersArray()).filter(
        ({ name, value }) =>
          name.toLowerCase() === "set-cookie" &&
          value.startsWith(`${cookieName}=`),
      ),
    ).toEqual([]);

    releaseCsrf();
    const [deleted] = await ending;
    expect(deleted).toBe(true);
    await tab1.close();
    await tab3.close();
    await expectSessionStaysEnded(context);
  });
}

// #5402: the tenant cookie spans every school's subdomain. A renewal from a
// tab of school A, answered only after "Zur Schulanmeldung" at school B and a
// new login, writes no cookie and leaves the new session alone. (The fixture
// issues sessions of school A only, so the new login happens there.)
test("Zur Schulanmeldung: a late renewal from the other school keeps the new session", async ({
  browser,
}) => {
  const context = await browser.newContext();
  await isolate(context);
  const tab1 = await context.newPage();
  await login(tab1);
  const tab2 = await context.newPage();
  await tab2.goto(`${source}/api/health`);
  const pending = await holdRequest(
    tab2,
    "**/api/auth/session",
    renewingUpdate,
    "POST",
  );
  await pending.intercepted;

  const [deleted] = await Promise.all([
    waitForSessionCookieDeletion(tab1),
    endActions["Zur Schulanmeldung"](tab1),
  ]);
  expect(deleted).toBe(true);
  await login(tab1);
  const fresh = (await context.cookies(source)).find(
    (cookie) => cookie.name === cookieName,
  )?.value;
  expect(fresh).toBeDefined();

  expect(await pending.release()).toBe(false);
  expect(
    (await context.cookies(source)).find((cookie) => cookie.name === cookieName)
      ?.value,
  ).toBe(fresh);
  expect(
    await tab1.evaluate(() => fetch("/api/me/profile").then((r) => r.status)),
  ).toBe(200);
  await context.close();
});

// The window the server cannot close: an answer built while the family was
// still live, delivered after the cookie's deletion. Its cookie lands, but it
// names a revoked family: it opens nothing, reads as ended and a fresh page
// asks for the login (#5194, #5399).
test("logout: a renewal answered before the revocation and delivered after it opens nothing", async ({
  browser,
}) => {
  const context = await browser.newContext();
  await isolate(context);
  const tab1 = await context.newPage();
  await login(tab1);
  const tab2 = await context.newPage();
  await tab2.goto(`${source}/api/health`);
  const pending = await holdResponse(
    tab2,
    "**/api/auth/session",
    renewingUpdate,
    "POST",
  );
  expect((await pending.held).setCookie).toBe(true);

  const [deleted] = await Promise.all([
    waitForSessionCookieDeletion(tab1),
    endActions.logout(tab1),
  ]);
  expect(deleted).toBe(true);
  await tab1.close();

  await pending.release();
  expect(await pending.restored(context)).toBe(true);

  const tab3 = await context.newPage();
  await tab3.goto(`${source}/api/health`);
  expect(
    await tab3.evaluate(() => fetch("/api/me/profile").then((r) => r.status)),
  ).toBe(401);
  const session = (await tab3.evaluate(async () =>
    (await fetch("/api/auth/session")).json(),
  )) as { error?: string };
  expect(session.error).toBe("RefreshTokenError");
  await tab3.goto(`${source}/home`);
  await tab3.waitForURL((url) => url.pathname === "/");
  await expect(tab3.locator('input[type="email"]')).toBeVisible();
  await context.close();
});

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
