import { randomBytes } from "node:crypto";
import {
  test,
  expect,
  type BrowserContext,
  type Page,
  type Route,
} from "@playwright/test";
import { networkPorts } from "./ports.mjs";

const port = networkPorts("tenantSwitch").frontend;

// #5194 for the portals with their own host-only cookie: parents, school,
// operator and caterer. After the portal's logout revoked its family, a
// response held back from before (same tab or a second tab) writes the old
// cookie back; the still-unexpired access token must open nothing. The
// session is opened with the portal's own Auth.js credentials provider and a
// synthetic credential login, the logout runs through the portal's real BFF logout
// route, and the cookie is cleared through Auth.js. Synthetic data only.

const portals = [
  {
    name: "Eltern",
    origin: `http://eltern.localhost:${port}`,
    basePath: "/api/parent/auth",
    provider: "parent-credentials",
    scope: "parent",
    cookie: "parent.session-token",
    logout: "/api/parent/auth/logout",
    read: "/api/parent/calendar",
  },
  {
    name: "Schule",
    origin: `http://schule.mfa.localhost:${port}`,
    basePath: "/api/school/auth",
    provider: "school-credentials",
    scope: "school",
    cookie: "school.session-token",
    logout: "/api/school/auth/logout",
    read: "/api/school/supervisions",
  },
  {
    name: "Betreiber",
    origin: `http://operator.localhost:${port}`,
    basePath: "/api/operator/auth",
    provider: "operator-credentials",
    scope: "platform",
    cookie: "operator.session-token",
    logout: "/api/operator/auth/logout",
    read: "/api/operator/provisioning/organizations",
  },
  {
    name: "Caterer",
    origin: `http://caterer.localhost:${port}`,
    basePath: "/api/caterer/auth",
    provider: "caterer-credentials",
    scope: "caterer",
    cookie: "caterer.session-token",
    logout: "/api/caterer/auth/logout",
    read: "/api/caterer/schools",
  },
] as const;
type Portal = (typeof portals)[number];

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
}

/** Opens a session in the portal's own Auth.js instance. */
async function signIn(page: Page, portal: Portal) {
  // Token handoff is server-only; use the public credentials contract.
  const password = randomBytes(24).toString("hex");
  await page.goto(`${portal.origin}/api/health`);
  const status = await page.evaluate(
    async ({ basePath, provider, password }) => {
      const { csrfToken } = (await (
        await fetch(`${basePath}/csrf`)
      ).json()) as {
        csrfToken: string;
      };
      const body = new URLSearchParams({
        csrfToken,
        email: "fixture@example.invalid",
        password,
        json: "true",
      });
      const response = await fetch(`${basePath}/callback/${provider}`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body,
        // Auth.js redirects to NEXTAUTH_URL; only the cookie matters.
        redirect: "manual",
      });
      return response.type === "opaqueredirect" ? 302 : response.status;
    },
    { basePath: portal.basePath, provider: portal.provider, password },
  );
  expect(status).toBeLessThan(400);
}

async function sessionCookie(context: BrowserContext, portal: Portal) {
  return (await context.cookies(portal.origin)).find(
    (cookie) => cookie.name === portal.cookie,
  );
}

const readStatus = (page: Page, path: string) =>
  page.evaluate((target) => fetch(target).then((r) => r.status), path);

/** Holds the answer of a request started now, Set-Cookie included. */
async function holdResponse(
  page: Page,
  pattern: string,
  start: () => Promise<unknown>,
  cookieName: string,
) {
  let release!: () => void;
  const released = new Promise<void>((resolve) => (release = resolve));
  let reportHeld!: (setCookie: boolean) => void;
  const held = new Promise<boolean>((resolve) => (reportHeld = resolve));
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
    reportHeld(
      response
        .headersArray()
        .some(
          (h) =>
            h.name.toLowerCase() === "set-cookie" &&
            h.value.startsWith(`${cookieName}=`) &&
            !h.value.startsWith(`${cookieName}=;`),
        ),
    );
    await released;
    await route.fulfill({ response });
  });
  const finished = start().catch(() => undefined);
  return {
    held,
    release: async () => {
      release();
      await finished;
    },
  };
}

/** The portal's logout: revoke through its BFF route, then Auth.js signout. */
async function logOut(page: Page, portal: Portal) {
  const result = await page.evaluate(
    async ({ basePath, logout }) => {
      const revoked = await fetch(logout, { method: "POST" });
      const { csrfToken } = (await (
        await fetch(`${basePath}/csrf`)
      ).json()) as {
        csrfToken: string;
      };
      await fetch(`${basePath}/signout`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: new URLSearchParams({ csrfToken, json: "true" }),
        redirect: "manual",
      });
      return revoked.status;
    },
    { basePath: portal.basePath, logout: portal.logout },
  );
  expect(result).toBe(204);
}

const lateSources = {
  "BFF GET": (portal: Portal) => ({
    pattern: `**${portal.read}`,
    start: (page: Page) => readStatus(page, portal.read),
  }),
  "session GET": (portal: Portal) => ({
    pattern: `**${portal.basePath}/session`,
    start: (page: Page) =>
      page.evaluate(
        (path) => fetch(path).then((r) => r.status),
        `${portal.basePath}/session`,
      ),
  }),
  "session update() POST": (portal: Portal) => ({
    pattern: `**${portal.basePath}/session`,
    start: (page: Page) =>
      page.evaluate(async (basePath) => {
        const { csrfToken } = (await (
          await fetch(`${basePath}/csrf`)
        ).json()) as { csrfToken: string };
        const response = await fetch(`${basePath}/session`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ csrfToken, data: {} }),
        });
        return response.status;
      }, portal.basePath),
  }),
} as const;

for (const portal of portals) {
  for (const [sourceName, makeSource] of Object.entries(lateSources)) {
    for (const tabs of ["same tab", "second tab"] as const) {
      test(`${portal.name}: a late ${sourceName} Set-Cookie (${tabs}) restores no access`, async ({
        browser,
      }) => {
        const context = await browser.newContext();
        await isolate(context);
        const tab1 = await context.newPage();
        await signIn(tab1, portal);
        expect(await readStatus(tab1, portal.read)).toBe(200);
        const lateTab = tabs === "same tab" ? tab1 : await context.newPage();
        if (lateTab !== tab1) await lateTab.goto(`${portal.origin}/api/health`);

        const source = makeSource(portal);
        const pending = await holdResponse(
          lateTab,
          source.pattern,
          () => source.start(lateTab),
          portal.cookie,
        );
        expect(await pending.held).toBe(true);

        await logOut(tab1, portal);
        expect(await sessionCookie(context, portal)).toBeUndefined();

        // The held answer arrives late and writes the old cookie back.
        await pending.release();
        expect(await sessionCookie(context, portal)).toBeDefined();

        // A fresh page of the portal still reads nothing protected.
        const fresh = await context.newPage();
        await fresh.goto(`${portal.origin}/api/health`);
        expect(await readStatus(fresh, portal.read)).toBe(401);
        await expect
          .poll(() =>
            fresh.evaluate(async (path) => {
              const body = (await (await fetch(path)).json()) as {
                error?: string;
              } | null;
              return body?.error ?? null;
            }, `${portal.basePath}/session`),
          )
          .toBe("RefreshTokenError");
        await context.close();
      });
    }
  }

  test(`${portal.name}: logout leaves another family of the portal valid`, async ({
    browser,
  }) => {
    const device1 = await browser.newContext();
    const device2 = await browser.newContext();
    await isolate(device1);
    await isolate(device2);
    const page1 = await device1.newPage();
    const page2 = await device2.newPage();
    await signIn(page1, portal);
    await signIn(page2, portal);

    await logOut(page1, portal);

    expect(await readStatus(page2, portal.read)).toBe(200);
    await device1.close();
    await device2.close();
  });
}
