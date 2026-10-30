import { test, expect, type BrowserContext, type Page } from "@playwright/test";
import { randomBytes } from "node:crypto";
import { join } from "node:path";

const source = "http://school-a.mfa.localhost:13845";
const target = "http://school-b.mfa.localhost:13845";

// Chromium resolves *.localhost even when the CI Node process cannot. Read
// only session metadata in a browser tab; never render/dump the session JSON.
async function browserSession(
  context: BrowserContext,
  origin: string,
  path = "/api/auth/session",
) {
  const probe = await context.newPage();
  try {
    await probe.goto(`${origin}/api/health`);
    return await probe.evaluate(async (sessionPath) => {
      const session = await (await fetch(sessionPath)).json();
      return { tenantId: session.user?.tenantId, scope: session.user?.scope };
    }, path);
  } finally {
    await probe.close();
  }
}

async function login(page: Page, context: BrowserContext) {
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
  await page.goto(source);
  await page.locator('input[type="email"]').fill("fixture@example.invalid");
  await page
    .locator('input[type="password"]')
    .fill(randomBytes(24).toString("hex"));
  await page.getByRole("button", { name: "Anmelden", exact: true }).click();
  await page.waitForURL(`${source}/home`);
  // Only inspect cookie metadata, never include session contents in evidence.
  const metadata = (await context.cookies(target))
    .filter((cookie) => cookie.name === "mfa-localhost.session-token")
    .map(({ name, domain, httpOnly }) => ({ name, domain, httpOnly }));
  expect(metadata).toEqual([
    {
      name: "mfa-localhost.session-token",
      domain: ".mfa.localhost",
      httpOnly: true,
    },
  ]);
  expect((await browserSession(context, target)).tenantId).toBe(1);
}

for (const entry of ["menu", "relogin"] as const) {
  for (const stage of ["lookup", "DELETE"] as const) {
    test(`${entry}: a hanging push ${stage} does not prevent cookie deletion and navigation`, async ({
      page,
      context,
    }, testInfo) => {
      await login(page, context);
      if (entry === "relogin") {
        await page
          .getByRole("button", { name: "Einrichtung wechseln" })
          .click();
        await page.getByText("School B", { exact: true }).click();
      }
      await page.evaluate((hangLookup) => {
        Object.defineProperty(navigator.serviceWorker, "getRegistration", {
          value: async () => ({
            pushManager: {
              getSubscription: async () =>
                hangLookup
                  ? new Promise(() => undefined)
                  : {
                      endpoint: "https://push.example.invalid/device",
                      unsubscribe: async () => true,
                    },
            },
          }),
        });
      }, stage === "lookup");
      const deletion = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === "/api/auth/signout" &&
          response.request().method() === "POST",
      );
      const pushRequests: string[] = [];
      await page.route(
        "**/api/notifications/push/subscriptions?**",
        async (route) => {
          pushRequests.push(route.request().method());
          // A transport that never answers; the logout must abort it.
        },
      );
      const aborted =
        stage === "DELETE"
          ? page.waitForEvent("requestfailed", {
              predicate: (request) =>
                new URL(request.url()).pathname ===
                "/api/notifications/push/subscriptions",
            })
          : null;
      if (entry === "menu") {
        await page
          .getByRole("button", { name: /Profilmenü|Profil/ })
          .first()
          .click();
        await page.getByRole("button", { name: "Abmelden" }).first().click();
        await page
          .getByRole("dialog")
          .getByRole("button", { name: "Abmelden" })
          .click();
      } else
        await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
      expect((await deletion).status()).toBe(200);
      if (aborted) {
        expect((await aborted).failure()?.errorText).toContain("ERR_ABORTED");
        expect(pushRequests).toEqual(["DELETE"]);
      }
      await page.waitForURL(entry === "menu" ? `${source}/` : `${target}/`);
      await expect(page.locator('input[type="email"]')).toBeVisible();
      expect(
        (await context.cookies(target)).filter(
          (cookie) => cookie.name === "mfa-localhost.session-token",
        ),
      ).toHaveLength(0);
      await page.screenshot({ path: testInfo.outputPath("logged-out.png") });
    });
  }
}

test("a second tab cannot start a competing logout while relogin CSRF is stalled", async ({
  page,
  context,
}) => {
  await login(page, context);
  // This page uses required-session consumers; they must not redirect the
  // still-active source document while target navigation is held back.
  await page.goto(`${source}/profile`);
  const other = await context.newPage();
  await other.goto(`${source}/home`);
  await other.getByRole("button", { name: "Einrichtung wechseln" }).click();
  await other.getByRole("button", { name: "Einrichtung wechseln" }).click();
  let release!: () => void;
  const released = new Promise<void>((resolve) => {
    release = resolve;
  });
  let releaseNavigation!: () => void;
  const navigationReleased = new Promise<void>((resolve) => {
    releaseNavigation = resolve;
  });
  let navigationStarted!: () => void;
  const navigation = new Promise<void>((resolve) => {
    navigationStarted = resolve;
  });
  await page.route(`${target}/`, async (route) => {
    navigationStarted();
    await navigationReleased;
    await route.continue();
  });
  let csrfRequests = 0;
  let signouts = 0;
  await page.route("**/api/auth/csrf", async (route) => {
    csrfRequests += 1;
    await released;
    await route.continue();
  });
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === "/api/auth/signout") signouts += 1;
  });
  try {
    await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
    await page.getByText("School B", { exact: true }).click();
    const echo = page.waitForResponse(
      (response) => new URL(response.url()).pathname === "/api/auth/session",
    );
    await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
    await echo;
    // The actual second document's storage listener/broadcast has now
    // delivered the ended session to the source's real provider and guard.
    await expect(page.getByText("Sitzung wird erneuert...")).toBeVisible();
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    );
    expect(csrfRequests).toBe(1);
    expect(signouts).toBe(0);
    expect(page.url()).toBe(`${source}/profile`);
    release();
    await navigation;
    const lateEcho = page.waitForResponse(
      (response) => new URL(response.url()).pathname === "/api/auth/session",
    );
    await other.evaluate(() => {
      const channel = new BroadcastChannel("next-auth");
      channel.postMessage({ event: "session", data: { trigger: "signout" } });
      channel.close();
    });
    await (await lateEcho).finished();
    // The old execution context may already be suspended by navigation;
    // observe network/URL state without evaluating inside that context.
    expect(signouts).toBe(1);
    expect(page.url()).toBe(`${source}/profile`);
    releaseNavigation();
    await page.waitForURL(`${target}/`);
    await expect(page.locator('input[type="email"]')).toBeVisible();
    expect(signouts).toBe(1);
  } finally {
    release();
    releaseNavigation();
    await other.close();
  }
});

test("failed family revocation keeps the shared session and shows a recoverable error", async ({
  page,
  context,
}) => {
  await login(page, context);
  await page.route("**/api/auth/logout", (route) =>
    route.fulfill({
      status: 503,
      json: { status: 503, code: "general.unavailable" },
    }),
  );
  await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
  await page.getByText("School B", { exact: true }).click();
  await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "die Abmeldung" }),
  ).toBeVisible();
  expect(page.url()).toBe(`${source}/home`);
  expect((await browserSession(context, target)).tenantId).toBe(1);
  await page.unroute("**/api/auth/logout");
  await page.getByRole("button", { name: "Wiederholen" }).click();
  await page.waitForURL(`${target}/`);
  await expect(page.locator('input[type="email"]')).toBeVisible();
});

test("a failed Auth.js cookie deletion stops at the target guard and permits explicit recovery", async ({
  page,
  context,
}) => {
  await login(page, context);
  // Auth.js' client currently resolves signOut even for a JSON 500 response.
  await page.route("**/api/auth/signout", (route) =>
    route.fulfill({ status: 500, json: { error: "fixture signout failure" } }),
  );
  await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
  await page.getByText("School B", { exact: true }).click();
  await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
  await page.waitForURL(`${target}/`);
  const recovery = page.getByRole("button", { name: "Zur Schulanmeldung" });
  await expect(recovery).toBeVisible();
  await expect(
    page.getByRole("alert").filter({ hasText: "zweiten Anmeldenachweis" }),
  ).toBeVisible();
  expect(page.url()).toBe(`${target}/`);
  // When Auth.js is available again, the explicit action reaches the form.
  await page.unroute("**/api/auth/signout");
  await recovery.click();
  await expect(page.locator('input[type="email"]')).toBeVisible();
});

test("tenant reauthentication preserves the independent school-portal session", async ({
  page,
  context,
}) => {
  const school = "http://schule.mfa.localhost:13845";
  await login(page, context);
  await page.goto(`${school}/login`);
  await page.locator('input[type="email"]').fill("fixture@example.invalid");
  await page
    .locator('input[type="password"]')
    .fill(randomBytes(24).toString("hex"));
  await page.getByRole("button", { name: "Anmelden", exact: true }).click();
  await page.waitForURL(`${school}/`);
  const metadata = (await context.cookies(school))
    .filter((cookie) => cookie.name === "school.session-token")
    .map(({ name, domain }) => ({ name, domain }));
  expect(metadata).toEqual([
    { name: "school.session-token", domain: "schule.mfa.localhost" },
  ]);
  await page.goto(`${target}/home`);
  await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
  await page.waitForURL(`${target}/`);
  await expect(page.locator('input[type="email"]')).toBeVisible();
  expect(
    (await browserSession(context, school, "/api/school/auth/session")).scope,
  ).toBe("school");
});

for (const entry of ["switcher", "guard"] as const) {
  test(`${entry}: shared-domain MFA refusal ends the old session before target login`, async ({
    page,
    context,
    request,
  }) => {
    await login(page, context);
    const before = await (
      await request.get("http://127.0.0.1:18445/__fixture/switch-state")
    ).json();
    if (entry === "switcher") {
      await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
      await page.getByText("School B", { exact: true }).click();
    } else {
      await page.goto(`${target}/home`);
    }
    const relogin = page.getByRole("button", { name: "Zur Schulanmeldung" });
    await expect(relogin).toBeVisible();
    if (process.env.TENANT_SWITCH_SHOT_DIR) {
      const hint = page
        .getByRole("alert")
        .filter({ hasText: "zweiten Anmeldenachweis" });
      const region = entry === "guard" ? hint.locator("..") : hint;
      await region.screenshot({
        path: join(process.env.TENANT_SWITCH_SHOT_DIR, `${entry}.png`),
      });
    }
    await relogin.click();
    await page.waitForURL(`${target}/`);
    await expect(page.locator('input[type="email"]')).toBeVisible();
    const after = await (
      await request.get("http://127.0.0.1:18445/__fixture/switch-state")
    ).json();
    expect(after.logoutRequests - before.logoutRequests).toBe(1);
    expect(after.switchRequests - before.switchRequests).toBe(1);
    expect(
      (await context.cookies(target)).filter(
        (cookie) => cookie.name === "mfa-localhost.session-token",
      ).length,
    ).toBe(0);
  });
}
